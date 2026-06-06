// Package aihubmix implements the SiteAdapter for AIHubMix-type remote sites.
// AIHubMix uses raw token auth (NOT Bearer) and a dedicated API at aihubmix.com.
package aihubmix

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/lingyuins/octopus/internal/hub"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

func init() {
	hub.Register(model.SiteTypeAIHubMix, &Adapter{})
}

// Adapter implements hub.SiteAdapter for AIHubMix-type remote sites.
type Adapter struct{}

const apiOrigin = "https://aihubmix.com"

// aihubmixFetch performs a JSON API call with raw token auth (NOT Bearer).
func aihubmixFetch[T any](ctx context.Context, site *model.RemoteSite, method, endpoint string, reqBody interface{}) (T, error) {
	var zero T

	token, err := crypto.Decrypt(site.AccessToken)
	if err != nil {
		return zero, fmt.Errorf("decrypt access token: %w", err)
	}

	var body io.Reader
	if reqBody != nil {
		b, _ := json.Marshal(reqBody)
		body = strings.NewReader(string(b))
	}

	url := apiOrigin + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	// AIHubMix uses raw token, NOT "Bearer <token>"
	req.Header.Set("Authorization", token)

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

	var envelope struct {
		Success *bool           `json:"success,omitempty"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return zero, fmt.Errorf("parse response from %s: %w", endpoint, err)
	}
	if envelope.Success != nil && !*envelope.Success {
		return zero, fmt.Errorf("API error from %s: %s", endpoint, envelope.Message)
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

type userSelfResponse struct {
	ID        int     `json:"id"`
	Username  string  `json:"username"`
	Quota     float64 `json:"quota"`
	UsedQuota float64 `json:"used_quota"`
}

func (a *Adapter) FetchUserInfo(ctx context.Context, site *model.RemoteSite) (*hub.UserInfoResult, error) {
	u, err := aihubmixFetch[userSelfResponse](ctx, site, http.MethodGet, "/api/user/self", nil)
	if err != nil {
		return nil, err
	}
	return &hub.UserInfoResult{
		ID:        u.ID,
		Username:  u.Username,
		Quota:     u.Quota,
		UsedQuota: u.UsedQuota,
	}, nil
}

func (a *Adapter) FetchCheckInStatus(_ context.Context, _ *model.RemoteSite) (*bool, error) {
	return nil, nil // AIHubMix does not support check-in
}

func (a *Adapter) PerformCheckIn(_ context.Context, _ *model.RemoteSite) (*hub.CheckInResult, error) {
	return &hub.CheckInResult{Success: false, Message: "check-in not supported"}, nil
}

// ── Models & Pricing ────────────────────────────────────────────────────────

type aihubmixModel struct {
	ModelID     string `json:"model_id"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Pricing     *struct {
		CacheRead float64 `json:"cache_read"`
		Input     float64 `json:"input"`
		Output    float64 `json:"output"`
	} `json:"pricing"`
}

func (a *Adapter) FetchModels(ctx context.Context, site *model.RemoteSite) ([]string, error) {
	models, err := aihubmixFetch[[]aihubmixModel](ctx, site, http.MethodGet, "/api/v1/models", nil)
	if err != nil {
		// Fallback: try user-scoped model list
		type availModel struct {
			Name string `json:"name"`
		}
		avail, err2 := aihubmixFetch[[]availModel](ctx, site, http.MethodGet, "/api/user/available_models", nil)
		if err2 != nil {
			return nil, fmt.Errorf("fetch models: %w (fallback: %w)", err, err2)
		}
		names := make([]string, 0, len(avail))
		for _, m := range avail {
			names = append(names, m.Name)
		}
		return names, nil
	}
	names := make([]string, 0, len(models))
	for _, m := range models {
		name := m.Name
		if name == "" {
			name = m.ModelID
		}
		if name == "" {
			name = m.ID
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func (a *Adapter) FetchModelPricing(ctx context.Context, site *model.RemoteSite) ([]hub.ModelPricingEntry, error) {
	models, err := aihubmixFetch[[]aihubmixModel](ctx, site, http.MethodGet, "/api/v1/models", nil)
	if err != nil {
		return nil, err
	}
	result := make([]hub.ModelPricingEntry, 0, len(models))
	for _, m := range models {
		name := m.Name
		if name == "" {
			name = m.ModelID
		}
		if name == "" {
			continue
		}
		entry := hub.ModelPricingEntry{ModelName: name}
		if m.Pricing != nil {
			entry.Quota = m.Pricing.Input
			if m.Pricing.Output > 0 {
				entry.CompletionRatio = m.Pricing.Output / m.Pricing.Input
			}
		}
		result = append(result, entry)
	}
	return result, nil
}

// ── Tokens ──────────────────────────────────────────────────────────────────

type aihubmixToken struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	Key            string  `json:"key"`
	Status         int     `json:"status"`
	RemainQuota    float64 `json:"remain_quota"`
	UsedQuota      float64 `json:"used_quota"`
	UnlimitedQuota bool    `json:"unlimited_quota"`
	ModelLimits    string  `json:"models"`
	ExpiredTime    int64   `json:"expired_time"`
	CreatedTime    int64   `json:"created_time"`
}

func (a *Adapter) FetchTokens(ctx context.Context, site *model.RemoteSite) ([]hub.RemoteToken, error) {
	tokens, err := aihubmixFetch[[]aihubmixToken](ctx, site, http.MethodGet, "/api/token/", nil)
	if err != nil {
		return nil, err
	}
	result := make([]hub.RemoteToken, 0, len(tokens))
	for _, t := range tokens {
		result = append(result, hub.RemoteToken{
			ID:             t.ID,
			Name:           t.Name,
			Key:            t.Key,
			Status:         t.Status,
			RemainQuota:    t.RemainQuota,
			UsedQuota:      t.UsedQuota,
			UnlimitedQuota: t.UnlimitedQuota,
			ModelLimits:    t.ModelLimits,
			ExpiredTime:    t.ExpiredTime,
			CreatedTime:    t.CreatedTime,
		})
	}
	return result, nil
}

func (a *Adapter) CreateToken(ctx context.Context, site *model.RemoteSite, req hub.CreateTokenRequest) error {
	payload := map[string]interface{}{
		"name":            req.Name,
		"unlimited_quota": req.UnlimitedQuota,
	}
	if req.RemainQuota > 0 {
		payload["remain_quota"] = req.RemainQuota
	}
	if req.ExpiredTime > 0 {
		payload["expired_time"] = req.ExpiredTime
	}
	if req.ModelLimits != "" {
		payload["models"] = req.ModelLimits
	}
	_, err := aihubmixFetch[interface{}](ctx, site, http.MethodPost, "/api/token/", payload)
	return err
}

// ── Channels (not supported) ────────────────────────────────────────────────

func (a *Adapter) ListChannels(_ context.Context, _ *model.RemoteSite) ([]hub.RemoteChannel, error) {
	return nil, nil
}

func (a *Adapter) CreateChannel(_ context.Context, _ *model.RemoteSite, _ hub.RemoteChannelCreateReq) error {
	return fmt.Errorf("channel management not supported for AIHubMix sites")
}

func (a *Adapter) UpdateChannel(_ context.Context, _ *model.RemoteSite, _ hub.RemoteChannelUpdateReq) error {
	return fmt.Errorf("channel management not supported for AIHubMix sites")
}

func (a *Adapter) DeleteChannel(_ context.Context, _ *model.RemoteSite, _ int) error {
	return fmt.Errorf("channel management not supported for AIHubMix sites")
}

// ── Announcements / Status ──────────────────────────────────────────────────

func (a *Adapter) FetchAnnouncement(_ context.Context, _ *model.RemoteSite) (string, error) {
	return "", nil
}

func (a *Adapter) FetchSiteStatus(_ context.Context, _ *model.RemoteSite) (*hub.SiteStatusInfo, error) {
	return &hub.SiteStatusInfo{
		SystemName:     "AIHubMix",
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
