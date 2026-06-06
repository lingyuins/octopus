// Package axonhub implements the SiteAdapter for AxonHub-type remote sites.
// AxonHub uses a GraphQL admin API at /admin/graphql with email/password JWT auth.
package axonhub

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
	hub.Register(model.SiteTypeAxonHub, &Adapter{})
}

// Adapter implements hub.SiteAdapter for AxonHub-type remote sites.
type Adapter struct{}

// ── JWT auth ────────────────────────────────────────────────────────────────

type cachedToken struct {
	token     string
	expiresAt time.Time
}

var (
	tokenMu    sync.Mutex
	tokenCache = make(map[string]*cachedToken) // key: "baseURL|email"
)

func cacheKey(site *model.RemoteSite) string {
	return strings.TrimRight(site.BaseURL, "/") + "|" + site.Username
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

	body, _ := json.Marshal(map[string]string{
		"email":    site.Username, // AxonHub uses email as username
		"password": password,
	})

	url := strings.TrimRight(site.BaseURL, "/") + "/admin/auth/signin"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("signin request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("signin failed (HTTP %d): %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse signin response: %w", err)
	}
	if result.Token == "" {
		return "", fmt.Errorf("signin returned empty token")
	}

	tokenCache[key] = &cachedToken{
		token:     result.Token,
		expiresAt: time.Now().Add(15 * time.Minute),
	}
	return result.Token, nil
}

// graphqlRequest sends a GraphQL request to /admin/graphql.
func graphqlRequest[T any](ctx context.Context, site *model.RemoteSite, query string, variables map[string]interface{}) (T, error) {
	var zero T

	token, err := getValidToken(ctx, site)
	if err != nil {
		return zero, fmt.Errorf("auth: %w", err)
	}

	gqlBody, _ := json.Marshal(map[string]interface{}{
		"query":     query,
		"variables": variables,
	})

	url := strings.TrimRight(site.BaseURL, "/") + "/admin/graphql"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(gqlBody)))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("graphql request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, err
	}

	// On 401/403, retry once with fresh token
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		tokenMu.Lock()
		delete(tokenCache, cacheKey(site))
		tokenMu.Unlock()

		token, err = getValidToken(ctx, site)
		if err != nil {
			return zero, fmt.Errorf("re-auth: %w", err)
		}

		req, _ = http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(gqlBody)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			return zero, fmt.Errorf("graphql retry request: %w", err)
		}
		defer resp.Body.Close()
		respBody, _ = io.ReadAll(resp.Body)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var gqlResp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return zero, fmt.Errorf("parse graphql response: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		return zero, fmt.Errorf("graphql error: %s", gqlResp.Errors[0].Message)
	}

	var result T
	if err := json.Unmarshal(gqlResp.Data, &result); err != nil {
		return zero, fmt.Errorf("unmarshal graphql data: %w", err)
	}
	return result, nil
}

// ── SiteAdapter implementation ──────────────────────────────────────────────

func (a *Adapter) FetchUserInfo(_ context.Context, _ *model.RemoteSite) (*hub.UserInfoResult, error) {
	return nil, nil // AxonHub does not expose user info via GraphQL
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
	return fmt.Errorf("token management not supported for AxonHub sites")
}

// ── Channels (via GraphQL) ──────────────────────────────────────────────────

type axonChannel struct {
	ID              string   `json:"id"` // opaque GraphQL ID
	Name            string   `json:"name"`
	Type            string   `json:"type"` // "openai", "anthropic", etc.
	Status          string   `json:"status"`
	BaseURL         string   `json:"baseURL"`
	SupportedModels []string `json:"supportedModels"`
	ManualModels    []string `json:"manualModels"`
	Remark          string   `json:"remark"`
}

type channelEdge struct {
	Node   axonChannel `json:"node"`
	Cursor string      `json:"cursor"`
}

type queryChannelsResult struct {
	QueryChannels struct {
		Edges    []channelEdge `json:"edges"`
		PageInfo struct {
			HasNextPage bool   `json:"hasNextPage"`
			EndCursor   string `json:"endCursor"`
		} `json:"pageInfo"`
		TotalCount int `json:"totalCount"`
	} `json:"queryChannels"`
}

const queryChannelsGQL = `
query QueryChannels($input: QueryChannelInput!) {
  queryChannels(input: $input) {
    edges {
      node {
        id, name, type, status, baseURL,
        supportedModels, manualModels, remark
      }
      cursor
    }
    pageInfo { hasNextPage, endCursor }
    totalCount
  }
}`

// graphqlIDMap stores bidirectional mapping between numeric IDs and GraphQL opaque IDs.
var (
	gqlIDMu       sync.RWMutex
	numToGraphql  = make(map[int]string)
	graphqlToNum  = make(map[string]int)
	nextNumericID = 1000000 // start from a high number to avoid conflicts
)

func mapGraphqlID(gqlID string) int {
	gqlIDMu.Lock()
	defer gqlIDMu.Unlock()
	if numID, ok := graphqlToNum[gqlID]; ok {
		return numID
	}
	nextNumericID++
	numToGraphql[nextNumericID] = gqlID
	graphqlToNum[gqlID] = nextNumericID
	return nextNumericID
}

func resolveGraphqlID(numericID int) (string, bool) {
	gqlIDMu.RLock()
	defer gqlIDMu.RUnlock()
	gqlID, ok := numToGraphql[numericID]
	return gqlID, ok
}

func (a *Adapter) ListChannels(ctx context.Context, site *model.RemoteSite) ([]hub.RemoteChannel, error) {
	var allChannels []hub.RemoteChannel
	var cursor string

	for {
		input := map[string]interface{}{
			"first": 100,
		}
		if cursor != "" {
			input["after"] = cursor
		}

		result, err := graphqlRequest[queryChannelsResult](ctx, site, queryChannelsGQL, map[string]interface{}{
			"input": input,
		})
		if err != nil {
			return nil, err
		}

		for _, edge := range result.QueryChannels.Edges {
			numID := mapGraphqlID(edge.Node.ID)
			status := 2 // disabled
			if edge.Node.Status == "enabled" {
				status = 1
			}
			models := strings.Join(edge.Node.SupportedModels, ",")
			if len(edge.Node.ManualModels) > 0 {
				models = strings.Join(edge.Node.ManualModels, ",")
			}
			allChannels = append(allChannels, hub.RemoteChannel{
				ID:      numID,
				Name:    edge.Node.Name,
				Status:  status,
				Models:  models,
				BaseURL: edge.Node.BaseURL,
			})
		}

		if !result.QueryChannels.PageInfo.HasNextPage {
			break
		}
		cursor = result.QueryChannels.PageInfo.EndCursor
	}
	return allChannels, nil
}

const createChannelGQL = `
mutation CreateChannel($input: CreateChannelInput!) {
  createChannel(input: $input) {
    id, name, type, status, baseURL
  }
}`

func (a *Adapter) CreateChannel(ctx context.Context, site *model.RemoteSite, ch hub.RemoteChannelCreateReq) error {
	models := strings.Split(ch.Models, ",")
	input := map[string]interface{}{
		"name":            ch.Name,
		"type":            "openai",
		"baseURL":         ch.BaseURL,
		"supportedModels": models,
		"credentials": map[string]interface{}{
			"apiKeys": []string{ch.Key},
		},
	}
	_, err := graphqlRequest[interface{}](ctx, site, createChannelGQL, map[string]interface{}{
		"input": input,
	})
	return err
}

const updateChannelGQL = `
mutation UpdateChannel($id: ID!, $input: UpdateChannelInput!) {
  updateChannel(id: $id, input: $input) {
    id, name, status
  }
}`

func (a *Adapter) UpdateChannel(ctx context.Context, site *model.RemoteSite, ch hub.RemoteChannelUpdateReq) error {
	gqlID, ok := resolveGraphqlID(ch.ID)
	if !ok {
		return fmt.Errorf("channel %d not found in ID map (try listing channels first)", ch.ID)
	}
	input := map[string]interface{}{}
	if ch.Models != "" {
		input["supportedModels"] = strings.Split(ch.Models, ",")
	}
	_, err := graphqlRequest[interface{}](ctx, site, updateChannelGQL, map[string]interface{}{
		"id":    gqlID,
		"input": input,
	})
	return err
}

const deleteChannelGQL = `
mutation DeleteChannel($id: ID!) {
  deleteChannel(id: $id)
}`

func (a *Adapter) DeleteChannel(ctx context.Context, site *model.RemoteSite, channelID int) error {
	gqlID, ok := resolveGraphqlID(channelID)
	if !ok {
		return fmt.Errorf("channel %d not found in ID map (try listing channels first)", channelID)
	}
	_, err := graphqlRequest[interface{}](ctx, site, deleteChannelGQL, map[string]interface{}{
		"id": gqlID,
	})
	return err
}

// ── Announcements / Status ──────────────────────────────────────────────────

func (a *Adapter) FetchAnnouncement(_ context.Context, _ *model.RemoteSite) (string, error) {
	return "", nil
}

func (a *Adapter) FetchSiteStatus(_ context.Context, _ *model.RemoteSite) (*hub.SiteStatusInfo, error) {
	return &hub.SiteStatusInfo{
		SystemName:     "AxonHub",
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
