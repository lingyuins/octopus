// Package claudecodehub implements the SiteAdapter for ClaudeCodeHub-type remote sites.
// ClaudeCodeHub uses a static admin token and an action-based API at /api/actions/providers/{action}.
package claudecodehub

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
	hub.Register(model.SiteTypeClaudeCodeHub, &Adapter{})
}

// Adapter implements hub.SiteAdapter for ClaudeCodeHub-type remote sites.
type Adapter struct{}

// cchFetch performs a JSON API call with static admin token auth.
func cchFetch[T any](ctx context.Context, site *model.RemoteSite, method, endpoint string, reqBody interface{}) (T, error) {
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

	// ClaudeCodeHub response envelope: {ok: bool, data?: T, error?: unknown}
	var envelope struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data"`
		Error interface{}     `json:"error"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return zero, fmt.Errorf("parse response from %s: %w", endpoint, err)
	}
	if !envelope.OK {
		return zero, fmt.Errorf("API error from %s: %v", endpoint, envelope.Error)
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

// doAction posts to the action API endpoint.
func doAction[T any](ctx context.Context, site *model.RemoteSite, action string, payload interface{}) (T, error) {
	endpoint := fmt.Sprintf("/api/actions/providers/%s", action)
	return cchFetch[T](ctx, site, http.MethodPost, endpoint, payload)
}

// ── SiteAdapter implementation ──────────────────────────────────────────────

// Provider represents a ClaudeCodeHub provider (their equivalent of a "channel").
type Provider struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	URL           string   `json:"url"`
	MaskedKey     string   `json:"maskedKey"`
	Key           string   `json:"key"`
	IsEnabled     bool     `json:"isEnabled"`
	Weight        int      `json:"weight"`
	Priority      int      `json:"priority"`
	GroupTag      string   `json:"groupTag"`
	ProviderType  string   `json:"providerType"`
	AllowedModels []string `json:"allowedModels"`
}

func (a *Adapter) FetchUserInfo(_ context.Context, _ *model.RemoteSite) (*hub.UserInfoResult, error) {
	return nil, nil // ClaudeCodeHub does not expose user info
}

func (a *Adapter) FetchCheckInStatus(_ context.Context, _ *model.RemoteSite) (*bool, error) {
	return nil, nil
}

func (a *Adapter) PerformCheckIn(_ context.Context, _ *model.RemoteSite) (*hub.CheckInResult, error) {
	return &hub.CheckInResult{Success: false, Message: "check-in not supported"}, nil
}

func (a *Adapter) FetchModels(_ context.Context, _ *model.RemoteSite) ([]string, error) {
	return nil, nil
}

func (a *Adapter) FetchModelPricing(_ context.Context, _ *model.RemoteSite) ([]hub.ModelPricingEntry, error) {
	return nil, nil
}

func (a *Adapter) FetchTokens(_ context.Context, _ *model.RemoteSite) ([]hub.RemoteToken, error) {
	return nil, nil
}

func (a *Adapter) CreateToken(_ context.Context, _ *model.RemoteSite, _ hub.CreateTokenRequest) error {
	return fmt.Errorf("token management not supported for ClaudeCodeHub sites")
}

// ── Channels (via providers) ────────────────────────────────────────────────

func (a *Adapter) ListChannels(ctx context.Context, site *model.RemoteSite) ([]hub.RemoteChannel, error) {
	providers, err := doAction[[]Provider](ctx, site, "getProviders", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	result := make([]hub.RemoteChannel, 0, len(providers))
	for _, p := range providers {
		status := 2 // disabled
		if p.IsEnabled {
			status = 1
		}
		models := strings.Join(p.AllowedModels, ",")
		result = append(result, hub.RemoteChannel{
			ID:      p.ID,
			Name:    p.Name,
			Status:  status,
			Models:  models,
			BaseURL: p.URL,
			Group:   p.GroupTag,
		})
	}
	return result, nil
}

func (a *Adapter) CreateChannel(ctx context.Context, site *model.RemoteSite, ch hub.RemoteChannelCreateReq) error {
	allowedModels := strings.Split(ch.Models, ",")
	payload := map[string]interface{}{
		"name":           ch.Name,
		"url":            ch.BaseURL,
		"key":            ch.Key,
		"provider_type":  "openai_compatible",
		"allowed_models": allowedModels,
		"is_enabled":     true,
	}
	_, err := doAction[interface{}](ctx, site, "addProvider", payload)
	return err
}

func (a *Adapter) UpdateChannel(ctx context.Context, site *model.RemoteSite, ch hub.RemoteChannelUpdateReq) error {
	payload := map[string]interface{}{
		"providerId": ch.ID,
	}
	if ch.Models != "" {
		payload["allowed_models"] = strings.Split(ch.Models, ",")
	}
	_, err := doAction[interface{}](ctx, site, "editProvider", payload)
	return err
}

func (a *Adapter) DeleteChannel(ctx context.Context, site *model.RemoteSite, channelID int) error {
	payload := map[string]interface{}{
		"providerId": channelID,
	}
	_, err := doAction[interface{}](ctx, site, "removeProvider", payload)
	return err
}

// ── Announcements / Status ──────────────────────────────────────────────────

func (a *Adapter) FetchAnnouncement(_ context.Context, _ *model.RemoteSite) (string, error) {
	return "", nil
}

func (a *Adapter) FetchSiteStatus(_ context.Context, _ *model.RemoteSite) (*hub.SiteStatusInfo, error) {
	return &hub.SiteStatusInfo{
		SystemName:     "ClaudeCodeHub",
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
