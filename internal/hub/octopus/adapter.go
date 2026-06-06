// Package octopus implements the SiteAdapter for remote Octopus-type sites.
// Unlike the common (New API) adapter which uses Bearer token auth, Octopus
// sites authenticate via JWT login with username/password.
package octopus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/hub"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

func init() {
	hub.Register(model.SiteTypeOctopus, &Adapter{})
}

// Adapter implements hub.SiteAdapter for Octopus-type remote sites.
type Adapter struct{}

// ── JWT token cache ─────────────────────────────────────────────────────────

type cachedToken struct {
	token     string
	expiresAt time.Time
}

var (
	tokenMu    sync.Mutex
	tokenCache = make(map[string]*cachedToken) // key: "baseURL:username"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token    string `json:"token"`
	ExpireAt string `json:"expire_at"`
}

func cacheKey(site *model.RemoteSite) string {
	return strings.TrimRight(site.BaseURL, "/") + ":" + site.Username
}

func getValidToken(ctx context.Context, site *model.RemoteSite) (string, error) {
	key := cacheKey(site)

	tokenMu.Lock()
	defer tokenMu.Unlock()

	if cached, ok := tokenCache[key]; ok {
		if time.Now().Add(time.Minute).Before(cached.expiresAt) {
			return cached.token, nil
		}
	}

	password, err := crypto.Decrypt(site.Password)
	if err != nil {
		return "", fmt.Errorf("decrypt password: %w", err)
	}

	body, err := json.Marshal(loginRequest{
		Username: site.Username,
		Password: password,
	})
	if err != nil {
		return "", err
	}

	url := strings.TrimRight(site.BaseURL, "/") + "/api/v1/user/login"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read login response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login failed (HTTP %d): %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var envelope struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    *loginResponse `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return "", fmt.Errorf("parse login response: %w", err)
	}
	if envelope.Code != 200 || envelope.Data == nil {
		return "", fmt.Errorf("login error: %s", envelope.Message)
	}

	expiresAt, _ := time.Parse(time.RFC3339, envelope.Data.ExpireAt)
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(15 * time.Minute)
	}

	tokenCache[key] = &cachedToken{
		token:     envelope.Data.Token,
		expiresAt: expiresAt,
	}
	return envelope.Data.Token, nil
}

// octopusFetch wraps the standard fetch with JWT auth instead of Bearer token.
func octopusFetch[T any](ctx context.Context, site *model.RemoteSite, method, endpoint string, reqBody interface{}) (T, error) {
	var zero T

	token, err := getValidToken(ctx, site)
	if err != nil {
		return zero, fmt.Errorf("auth: %w", err)
	}

	// Build a temporary site with the JWT token as access_token so FetchJSON works.
	tmpSite := *site
	tmpSite.AuthType = model.AuthTypeAccessToken
	tmpSite.AccessToken = token // already plaintext, not encrypted
	// Override base URL to include /api/v1 prefix if endpoint doesn't already
	base := strings.TrimRight(site.BaseURL, "/")

	// Octopus API endpoints are under /api/v1
	fullURL := base + endpoint

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + token,
	}

	var body io.Reader
	if reqBody != nil {
		b, _ := json.Marshal(reqBody)
		body = strings.NewReader(string(b))
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return zero, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("request %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, endpoint, truncate(string(respBody), 200))
	}

	var envelope struct {
		Success *bool           `json:"success,omitempty"`
		Code    *int            `json:"code,omitempty"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return zero, fmt.Errorf("unmarshal response: %w", err)
	}

	if envelope.Success != nil && !*envelope.Success {
		return zero, fmt.Errorf("API error: %s", envelope.Message)
	}
	if envelope.Code != nil && *envelope.Code != 200 {
		return zero, fmt.Errorf("API error (code %d): %s", *envelope.Code, envelope.Message)
	}

	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return zero, nil
	}

	var result T
	if err := json.Unmarshal(envelope.Data, &result); err != nil {
		return zero, fmt.Errorf("unmarshal data: %w", err)
	}
	return result, nil
}

// ── SiteAdapter implementation ──────────────────────────────────────────────

type octopusUserInfo struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func (a *Adapter) FetchUserInfo(ctx context.Context, site *model.RemoteSite) (*hub.UserInfoResult, error) {
	// Octopus doesn't have a /api/user/self that returns quota; the login itself validates credentials.
	// We use the token cache validation as a proxy.
	_, err := getValidToken(ctx, site)
	if err != nil {
		return nil, err
	}
	return &hub.UserInfoResult{
		Username: site.Username,
	}, nil
}

func (a *Adapter) FetchCheckInStatus(_ context.Context, _ *model.RemoteSite) (*bool, error) {
	return nil, nil // Octopus does not support check-in
}

func (a *Adapter) PerformCheckIn(_ context.Context, _ *model.RemoteSite) (*hub.CheckInResult, error) {
	return &hub.CheckInResult{Success: false, Message: "check-in not supported"}, nil
}

func (a *Adapter) FetchModels(ctx context.Context, site *model.RemoteSite) ([]string, error) {
	type llmInfo struct {
		Name string `json:"name"`
	}
	infos, err := octopusFetch[[]llmInfo](ctx, site, http.MethodGet, "/api/v1/model/list", nil)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(infos))
	for _, m := range infos {
		names = append(names, m.Name)
	}
	return names, nil
}

func (a *Adapter) FetchModelPricing(ctx context.Context, site *model.RemoteSite) ([]hub.ModelPricingEntry, error) {
	type llmInfo struct {
		Name       string  `json:"name"`
		Input      float64 `json:"input"`
		Output     float64 `json:"output"`
		CacheRead  float64 `json:"cache_read"`
		CacheWrite float64 `json:"cache_write"`
	}
	infos, err := octopusFetch[[]llmInfo](ctx, site, http.MethodGet, "/api/v1/model/list", nil)
	if err != nil {
		return nil, err
	}
	result := make([]hub.ModelPricingEntry, 0, len(infos))
	for _, m := range infos {
		result = append(result, hub.ModelPricingEntry{
			ModelName: m.Name,
			Quota:     m.Input,
		})
	}
	return result, nil
}

func (a *Adapter) FetchTokens(_ context.Context, _ *model.RemoteSite) ([]hub.RemoteToken, error) {
	return nil, nil // Not applicable for Octopus managed sites
}

func (a *Adapter) CreateToken(_ context.Context, _ *model.RemoteSite, _ hub.CreateTokenRequest) error {
	return fmt.Errorf("token creation not supported for octopus sites")
}

// ── Channels ────────────────────────────────────────────────────────────────

type octopusChannel struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     int    `json:"type"`
	Enabled  bool   `json:"enabled"`
	Model    string `json:"model"`
	BaseUrls []struct {
		URL string `json:"url"`
	} `json:"base_urls"`
}

func (a *Adapter) ListChannels(ctx context.Context, site *model.RemoteSite) ([]hub.RemoteChannel, error) {
	channels, err := octopusFetch[[]octopusChannel](ctx, site, http.MethodGet, "/api/v1/channel/list", nil)
	if err != nil {
		return nil, err
	}
	result := make([]hub.RemoteChannel, 0, len(channels))
	for _, ch := range channels {
		baseURL := ""
		if len(ch.BaseUrls) > 0 {
			baseURL = ch.BaseUrls[0].URL
		}
		status := 2 // disabled
		if ch.Enabled {
			status = 1
		}
		result = append(result, hub.RemoteChannel{
			ID:      ch.ID,
			Name:    ch.Name,
			Type:    ch.Type,
			Status:  status,
			Models:  ch.Model,
			BaseURL: baseURL,
		})
	}
	return result, nil
}

func (a *Adapter) CreateChannel(ctx context.Context, site *model.RemoteSite, ch hub.RemoteChannelCreateReq) error {
	payload := map[string]interface{}{
		"name":      ch.Name,
		"type":      ch.Type,
		"enabled":   true,
		"base_urls": []map[string]string{{"url": ch.BaseURL}},
		"keys":      []map[string]interface{}{{"enabled": true, "channel_key": ch.Key}},
		"model":     ch.Models,
	}
	_, err := octopusFetch[interface{}](ctx, site, http.MethodPost, "/api/v1/channel/create", payload)
	return err
}

func (a *Adapter) UpdateChannel(ctx context.Context, site *model.RemoteSite, ch hub.RemoteChannelUpdateReq) error {
	payload := map[string]interface{}{
		"id":    ch.ID,
		"model": ch.Models,
	}
	_, err := octopusFetch[interface{}](ctx, site, http.MethodPost, "/api/v1/channel/update", payload)
	return err
}

func (a *Adapter) DeleteChannel(ctx context.Context, site *model.RemoteSite, channelID int) error {
	endpoint := fmt.Sprintf("/api/v1/channel/delete/%d", channelID)
	_, err := octopusFetch[interface{}](ctx, site, http.MethodDelete, endpoint, nil)
	return err
}

// ── Announcements / Status ──────────────────────────────────────────────────

func (a *Adapter) FetchAnnouncement(_ context.Context, _ *model.RemoteSite) (string, error) {
	return "", nil // Octopus does not have a public notice endpoint
}

func (a *Adapter) FetchSiteStatus(_ context.Context, _ *model.RemoteSite) (*hub.SiteStatusInfo, error) {
	return nil, nil
}

func (a *Adapter) RedeemCode(_ context.Context, _ *model.RemoteSite, _ string) (*hub.RedeemResult, error) {
	return nil, nil
}

func (a *Adapter) FetchUsageLogs(_ context.Context, _ *model.RemoteSite, _, _ int) ([]hub.RemoteUsageLog, error) {
	return nil, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
