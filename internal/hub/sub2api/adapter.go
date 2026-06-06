// Package sub2api implements the SiteAdapter for Sub2API-type remote sites.
// Sub2API uses a JWT access+refresh token auth flow with {code, message, data} envelope.
package sub2api

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
	hub.Register(model.SiteTypeSub2API, &Adapter{})
}

// Adapter implements hub.SiteAdapter for Sub2API-type remote sites.
type Adapter struct{}

// ── JWT auth with refresh token flow ────────────────────────────────────────

type cachedToken struct {
	accessToken  string
	refreshToken string
	expiresAt    time.Time
}

var (
	tokenMu    sync.Mutex
	tokenCache = make(map[string]*cachedToken) // key: "baseURL:username"
)

// sub2apiEnvelope is the response envelope used by Sub2API: {code: 0, message, data}.
type sub2apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func cacheKey(site *model.RemoteSite) string {
	return strings.TrimRight(site.BaseURL, "/") + ":" + site.Username
}

// getValidToken returns a valid access token, refreshing if needed.
// Sub2API stores access_token in site.AccessToken (encrypted) and refresh_token in site.Password (encrypted).
func getValidToken(ctx context.Context, site *model.RemoteSite) (string, error) {
	key := cacheKey(site)

	tokenMu.Lock()
	defer tokenMu.Unlock()

	if cached, ok := tokenCache[key]; ok {
		// Proactive refresh: if within 120 seconds of expiry
		if time.Now().Add(120 * time.Second).Before(cached.expiresAt) {
			return cached.accessToken, nil
		}
		// Try refresh token flow
		newToken, err := refreshToken(ctx, site, cached.refreshToken)
		if err == nil {
			tokenCache[key] = newToken
			return newToken.accessToken, nil
		}
		// Refresh failed, fall through to try stored token
	}

	// Use stored access token
	accessToken, err := crypto.Decrypt(site.AccessToken)
	if err != nil {
		return "", fmt.Errorf("decrypt access token: %w", err)
	}

	// Try stored refresh token to get a fresh access token
	refreshTokenStr, _ := crypto.Decrypt(site.Password) // Password field stores refresh token
	if refreshTokenStr != "" {
		newToken, err := refreshToken(ctx, site, refreshTokenStr)
		if err == nil {
			tokenCache[key] = newToken
			return newToken.accessToken, nil
		}
	}

	// Fallback to stored access token
	tokenCache[key] = &cachedToken{
		accessToken:  accessToken,
		refreshToken: refreshTokenStr,
		expiresAt:    time.Now().Add(5 * time.Minute), // assume 5 min validity
	}
	return accessToken, nil
}

type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds
}

func refreshToken(ctx context.Context, site *model.RemoteSite, refreshTokenStr string) (*cachedToken, error) {
	body, _ := json.Marshal(map[string]string{
		"refresh_token": refreshTokenStr,
	})

	url := strings.TrimRight(site.BaseURL, "/") + "/api/v1/auth/refresh"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var envelope sub2apiEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("parse refresh response: %w", err)
	}
	if envelope.Code != 0 {
		return nil, fmt.Errorf("refresh failed: %s", envelope.Message)
	}

	var data refreshResponse
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, fmt.Errorf("parse refresh data: %w", err)
	}

	expiresIn := data.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 300 // default 5 min
	}

	return &cachedToken{
		accessToken:  data.AccessToken,
		refreshToken: data.RefreshToken,
		expiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
	}, nil
}

// sub2apiFetch performs a JSON API call with JWT auth and {code, message, data} envelope.
func sub2apiFetch[T any](ctx context.Context, site *model.RemoteSite, method, endpoint string, reqBody interface{}) (T, error) {
	var zero T

	token, err := getValidToken(ctx, site)
	if err != nil {
		return zero, fmt.Errorf("auth: %w", err)
	}

	var body io.Reader
	if reqBody != nil {
		b, _ := json.Marshal(reqBody)
		body = strings.NewReader(string(b))
	}

	url := strings.TrimRight(site.BaseURL, "/") + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("request %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, endpoint, truncate(string(respBody), 200))
	}

	var envelope sub2apiEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return zero, fmt.Errorf("parse response from %s: %w", endpoint, err)
	}
	if envelope.Code != 0 {
		return zero, fmt.Errorf("API error from %s (code %d): %s", endpoint, envelope.Code, envelope.Message)
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

type authMeResponse struct {
	ID       int     `json:"id"`
	Username string  `json:"username"`
	Email    string  `json:"email"`
	Balance  float64 `json:"balance"` // USD
}

func (a *Adapter) FetchUserInfo(ctx context.Context, site *model.RemoteSite) (*hub.UserInfoResult, error) {
	me, err := sub2apiFetch[authMeResponse](ctx, site, http.MethodGet, "/api/v1/auth/me", nil)
	if err != nil {
		return nil, err
	}
	// Sub2API balance is in USD, convert to quota (500000 quota = 1 USD)
	return &hub.UserInfoResult{
		ID:       me.ID,
		Username: me.Username,
		Email:    me.Email,
		Quota:    me.Balance * 500000,
	}, nil
}

func (a *Adapter) FetchCheckInStatus(_ context.Context, _ *model.RemoteSite) (*bool, error) {
	return nil, nil // Sub2API does not support check-in
}

func (a *Adapter) PerformCheckIn(_ context.Context, _ *model.RemoteSite) (*hub.CheckInResult, error) {
	return &hub.CheckInResult{Success: false, Message: "check-in not supported"}, nil
}

func (a *Adapter) FetchModels(_ context.Context, _ *model.RemoteSite) ([]string, error) {
	return nil, nil // Sub2API does not expose a model list endpoint
}

func (a *Adapter) FetchModelPricing(_ context.Context, _ *model.RemoteSite) ([]hub.ModelPricingEntry, error) {
	return nil, nil
}

// ── Tokens ──────────────────────────────────────────────────────────────────

type sub2apiKey struct {
	ID        int     `json:"id"`
	UserID    int     `json:"user_id"`
	Key       string  `json:"key"`
	Name      string  `json:"name"`
	Status    int     `json:"status"`
	Quota     float64 `json:"quota"`      // USD, 0 = unlimited
	QuotaUsed float64 `json:"quota_used"` // USD
	ExpiresAt *int64  `json:"expires_at"`
	CreatedAt string  `json:"created_at"`
}

type sub2apiKeyList struct {
	Items []sub2apiKey `json:"items"`
	Total int          `json:"total"`
}

func (a *Adapter) FetchTokens(ctx context.Context, site *model.RemoteSite) ([]hub.RemoteToken, error) {
	var allTokens []hub.RemoteToken
	page := 0
	for {
		endpoint := fmt.Sprintf("/api/v1/keys?page=%d&page_size=100", page)
		list, err := sub2apiFetch[sub2apiKeyList](ctx, site, http.MethodGet, endpoint, nil)
		if err != nil {
			// Try flat array response
			tokens, err2 := sub2apiFetch[[]sub2apiKey](ctx, site, http.MethodGet, endpoint, nil)
			if err2 != nil {
				return nil, fmt.Errorf("fetch tokens: %w (fallback: %w)", err, err2)
			}
			list.Items = tokens
		}
		for _, t := range list.Items {
			remainQuota := t.Quota * 500000 // USD to quota
			unlimited := t.Quota == 0
			var expiredTime int64
			if t.ExpiresAt != nil {
				expiredTime = *t.ExpiresAt
			}
			allTokens = append(allTokens, hub.RemoteToken{
				ID:             t.ID,
				Name:           t.Name,
				Key:            t.Key,
				Status:         t.Status,
				RemainQuota:    remainQuota,
				UsedQuota:      t.QuotaUsed * 500000,
				UnlimitedQuota: unlimited,
				ExpiredTime:    expiredTime,
			})
		}
		if len(list.Items) < 100 {
			break
		}
		page++
	}
	return allTokens, nil
}

func (a *Adapter) CreateToken(ctx context.Context, site *model.RemoteSite, req hub.CreateTokenRequest) error {
	payload := map[string]interface{}{
		"name": req.Name,
	}
	if req.ExpiredTime > 0 {
		payload["expires_in_days"] = int(time.Until(time.Unix(req.ExpiredTime, 0)).Hours() / 24)
	}
	if req.UnlimitedQuota {
		payload["quota"] = 0
	} else if req.RemainQuota > 0 {
		payload["quota"] = float64(req.RemainQuota) / 500000 // quota to USD
	}
	_, err := sub2apiFetch[interface{}](ctx, site, http.MethodPost, "/api/v1/keys", payload)
	return err
}

// ── Channels (not supported) ────────────────────────────────────────────────

func (a *Adapter) ListChannels(_ context.Context, _ *model.RemoteSite) ([]hub.RemoteChannel, error) {
	return nil, nil
}

func (a *Adapter) CreateChannel(_ context.Context, _ *model.RemoteSite, _ hub.RemoteChannelCreateReq) error {
	return fmt.Errorf("channel management not supported for Sub2API sites")
}

func (a *Adapter) UpdateChannel(_ context.Context, _ *model.RemoteSite, _ hub.RemoteChannelUpdateReq) error {
	return fmt.Errorf("channel management not supported for Sub2API sites")
}

func (a *Adapter) DeleteChannel(_ context.Context, _ *model.RemoteSite, _ int) error {
	return fmt.Errorf("channel management not supported for Sub2API sites")
}

// ── Announcements ───────────────────────────────────────────────────────────

type sub2apiAnnouncement struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Message string `json:"message"`
	Body    string `json:"body"`
}

func (a *Adapter) FetchAnnouncement(ctx context.Context, site *model.RemoteSite) (string, error) {
	announcements, err := sub2apiFetch[[]sub2apiAnnouncement](ctx, site, http.MethodGet, "/api/v1/announcements", nil)
	if err != nil {
		return "", err
	}
	if len(announcements) == 0 {
		return "", nil
	}
	// Return the most recent announcement
	a0 := announcements[0]
	content := a0.Content
	if content == "" {
		content = a0.Body
	}
	if content == "" {
		content = a0.Message
	}
	if a0.Title != "" {
		content = a0.Title + "\n\n" + content
	}
	return content, nil
}

// ── Status ──────────────────────────────────────────────────────────────────

func (a *Adapter) FetchSiteStatus(_ context.Context, _ *model.RemoteSite) (*hub.SiteStatusInfo, error) {
	return &hub.SiteStatusInfo{
		SystemName:     "Sub2API",
		CheckInEnabled: false,
	}, nil
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
