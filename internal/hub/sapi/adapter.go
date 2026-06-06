// Package sapi implements the SiteAdapter for SAPI remote sites.
package sapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/hub"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

func init() {
	hub.Register(model.SiteTypeSAPI, &Adapter{})
}

type Adapter struct{}

type cachedToken struct {
	token     string
	expiresAt time.Time
}

var (
	tokenMu    sync.Mutex
	tokenCache = make(map[string]*cachedToken)
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Role  string   `json:"role"`
	Token string   `json:"token"`
	User  sapiUser `json:"user"`
}

type sapiUser struct {
	ID       json.RawMessage `json:"id"`
	Username string          `json:"username"`
	Name     string          `json:"name"`
	Email    string          `json:"email"`
	Enabled  *bool           `json:"enabled"`
	APIKey   string          `json:"apiKey"`
	APIKeys  []sapiAPIKey    `json:"apiKeys"`
}

type sapiAPIKey struct {
	ID            json.RawMessage `json:"id"`
	Name          string          `json:"name"`
	Key           string          `json:"key"`
	Enabled       *bool           `json:"enabled"`
	AllowedModels []string        `json:"allowedModels"`
	RPMLimit      int             `json:"rpmLimit"`
	CreatedAt     string          `json:"createdAt"`
}

type sapiModelEntry struct {
	ID string `json:"id"`
}

func (a *Adapter) FetchUserInfo(ctx context.Context, site *model.RemoteSite) (*hub.UserInfoResult, error) {
	resp, err := sapiFetch[struct {
		User sapiUser `json:"user"`
	}](ctx, site, http.MethodGet, "/api/user/me", nil)
	if err != nil {
		return nil, fmt.Errorf("fetch user info: %w", err)
	}
	return mapUserInfo(resp.User), nil
}

func (a *Adapter) FetchCheckInStatus(_ context.Context, _ *model.RemoteSite) (*bool, error) {
	return nil, nil
}

func (a *Adapter) PerformCheckIn(_ context.Context, _ *model.RemoteSite) (*hub.CheckInResult, error) {
	return &hub.CheckInResult{Success: false, Message: "check-in not supported for sapi sites"}, nil
}

func (a *Adapter) FetchModels(ctx context.Context, site *model.RemoteSite) ([]string, error) {
	type modelsResponse struct {
		Data []sapiModelEntry `json:"data"`
	}

	resp, err := fetchPublicJSON[modelsResponse](ctx, site, http.MethodGet, "/v1/models", nil)
	if err != nil {
		models, err2 := fetchPublicJSON[[]sapiModelEntry](ctx, site, http.MethodGet, "/v1/models", nil)
		if err2 != nil {
			return nil, err
		}
		return collectModelIDs(models), nil
	}
	return collectModelIDs(resp.Data), nil
}

func (a *Adapter) FetchModelPricing(_ context.Context, _ *model.RemoteSite) ([]hub.ModelPricingEntry, error) {
	return nil, nil
}

func (a *Adapter) FetchTokens(ctx context.Context, site *model.RemoteSite) ([]hub.RemoteToken, error) {
	resp, err := sapiFetch[struct {
		User sapiUser `json:"user"`
	}](ctx, site, http.MethodGet, "/api/user/me", nil)
	if err != nil {
		return nil, fmt.Errorf("fetch tokens: %w", err)
	}
	return mapTokens(resp.User), nil
}

func (a *Adapter) CreateToken(_ context.Context, _ *model.RemoteSite, _ hub.CreateTokenRequest) error {
	return fmt.Errorf("token creation not supported for sapi sites")
}

func (a *Adapter) ListChannels(_ context.Context, _ *model.RemoteSite) ([]hub.RemoteChannel, error) {
	return nil, nil
}

func (a *Adapter) CreateChannel(_ context.Context, _ *model.RemoteSite, _ hub.RemoteChannelCreateReq) error {
	return fmt.Errorf("channel creation not supported for sapi sites")
}

func (a *Adapter) UpdateChannel(_ context.Context, _ *model.RemoteSite, _ hub.RemoteChannelUpdateReq) error {
	return fmt.Errorf("channel update not supported for sapi sites")
}

func (a *Adapter) DeleteChannel(_ context.Context, _ *model.RemoteSite, _ int) error {
	return fmt.Errorf("channel deletion not supported for sapi sites")
}

func (a *Adapter) FetchAnnouncement(_ context.Context, _ *model.RemoteSite) (string, error) {
	return "", nil
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

func sapiFetch[T any](ctx context.Context, site *model.RemoteSite, method, endpoint string, reqBody interface{}) (T, error) {
	var zero T
	token, err := getValidToken(ctx, site)
	if err != nil {
		return zero, fmt.Errorf("auth: %w", err)
	}
	return fetchJSONWithHeaders[T](ctx, site, method, endpoint, reqBody, map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + token,
	})
}

func fetchPublicJSON[T any](ctx context.Context, site *model.RemoteSite, method, endpoint string, reqBody interface{}) (T, error) {
	return fetchJSONWithHeaders[T](ctx, site, method, endpoint, reqBody, map[string]string{"Content-Type": "application/json"})
}

func fetchJSONWithHeaders[T any](ctx context.Context, site *model.RemoteSite, method, endpoint string, reqBody interface{}, headers map[string]string) (T, error) {
	var zero T
	var body io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return zero, fmt.Errorf("marshal request body: %w", err)
		}
		body = bytes.NewReader(b)
	}
	url := strings.TrimRight(site.BaseURL, "/") + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return zero, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("request %s %s: %w", method, endpoint, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, endpoint, truncate(respBody, 200))
	}
	var result T
	if err := json.Unmarshal(respBody, &result); err != nil {
		return zero, fmt.Errorf("unmarshal response from %s: %w", endpoint, err)
	}
	return result, nil
}

func getValidToken(ctx context.Context, site *model.RemoteSite) (string, error) {
	key := cacheKey(site)
	tokenMu.Lock()
	defer tokenMu.Unlock()
	if cached, ok := tokenCache[key]; ok && time.Now().Add(time.Minute).Before(cached.expiresAt) {
		return cached.token, nil
	}
	password, err := crypto.Decrypt(site.Password)
	if err != nil {
		return "", fmt.Errorf("decrypt password: %w", err)
	}
	if site.Username == "" || password == "" {
		return "", fmt.Errorf("username and password are required")
	}
	login, err := fetchJSONWithHeaders[loginResponse](ctx, site, http.MethodPost, "/api/auth/login", loginRequest{
		Username: site.Username,
		Password: password,
	}, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return "", fmt.Errorf("login: %w", err)
	}
	if login.Token == "" {
		return "", fmt.Errorf("login response missing token")
	}
	tokenCache[key] = &cachedToken{token: login.Token, expiresAt: time.Now().Add(12 * time.Hour)}
	return login.Token, nil
}

func mapUserInfo(user sapiUser) *hub.UserInfoResult {
	status := 1
	if user.Enabled != nil && !*user.Enabled {
		status = 2
	}
	displayName := user.Name
	if displayName == "" {
		displayName = user.Username
	}
	return &hub.UserInfoResult{
		ID:          parseNumericID(user.ID),
		Username:    user.Username,
		DisplayName: displayName,
		Email:       user.Email,
		Status:      status,
		AccessToken: primaryAPIKey(user),
	}
}

func mapTokens(user sapiUser) []hub.RemoteToken {
	keys := user.APIKeys
	if len(keys) == 0 && user.APIKey != "" {
		keys = []sapiAPIKey{{Name: "Primary API Key", Key: user.APIKey}}
	}
	result := make([]hub.RemoteToken, 0, len(keys))
	for index, key := range keys {
		if key.Key == "" {
			continue
		}
		status := 1
		if key.Enabled != nil && !*key.Enabled {
			status = 2
		}
		name := key.Name
		if name == "" {
			name = fmt.Sprintf("API Key %d", index+1)
		}
		result = append(result, hub.RemoteToken{
			ID:          parseNumericID(key.ID),
			Name:        name,
			Key:         key.Key,
			Status:      status,
			ModelLimits: strings.Join(key.AllowedModels, ","),
			CreatedTime: parseTimeUnix(key.CreatedAt),
		})
	}
	return result
}

func primaryAPIKey(user sapiUser) string {
	for _, key := range user.APIKeys {
		if key.Key == "" {
			continue
		}
		if key.Enabled == nil || *key.Enabled {
			return key.Key
		}
	}
	return user.APIKey
}

func collectModelIDs(entries []sapiModelEntry) []string {
	models := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, id)
	}
	return models
}

func parseNumericID(raw json.RawMessage) int {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var num int
	if err := json.Unmarshal(raw, &num); err == nil {
		return num
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0
	}
	if _, err := fmt.Sscanf(text, "%d", &num); err == nil {
		return num
	}
	matches := regexp.MustCompile(`\d+`).FindAllString(text, -1)
	if len(matches) == 0 {
		return 0
	}
	fmt.Sscanf(matches[len(matches)-1], "%d", &num)
	return num
}

func parseTimeUnix(value string) int64 {
	if value == "" {
		return 0
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.Unix()
		}
	}
	return 0
}

func cacheKey(site *model.RemoteSite) string {
	return strings.TrimRight(site.BaseURL, "/") + ":" + site.Username
}

func truncate(data []byte, n int) string {
	text := string(data)
	if len(text) <= n {
		return text
	}
	return text[:n] + "..."
}
