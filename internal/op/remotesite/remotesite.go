// Package remotesite provides CRUD and refresh operations for remote AI relay sites.
package remotesite

import (
	"context"
	"fmt"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/hub"
	_ "github.com/lingyuins/octopus/internal/hub/aihubmix"
	_ "github.com/lingyuins/octopus/internal/hub/axonhub"
	_ "github.com/lingyuins/octopus/internal/hub/claudecodehub"
	_ "github.com/lingyuins/octopus/internal/hub/common"
	_ "github.com/lingyuins/octopus/internal/hub/octopus"
	_ "github.com/lingyuins/octopus/internal/hub/sapi"
	_ "github.com/lingyuins/octopus/internal/hub/sub2api"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/crypto"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// List returns all remote sites ordered by sort_order, pinned first.
func List(ctx context.Context) ([]model.RemoteSite, error) {
	var sites []model.RemoteSite
	err := db.GetDB().WithContext(ctx).
		Order("pinned DESC, sort_order ASC, id ASC").
		Find(&sites).Error
	if err != nil {
		return nil, fmt.Errorf("list remote sites: %w", err)
	}
	for i := range sites {
		maskSecrets(&sites[i])
	}
	return sites, nil
}

// Get returns a single remote site by ID.
func Get(ctx context.Context, id int) (*model.RemoteSite, error) {
	var site model.RemoteSite
	if err := db.GetDB().WithContext(ctx).First(&site, id).Error; err != nil {
		return nil, fmt.Errorf("get remote site %d: %w", id, err)
	}
	return &site, nil
}

// GetMasked returns a single remote site with secrets masked.
func GetMasked(ctx context.Context, id int) (*model.RemoteSite, error) {
	site, err := Get(ctx, id)
	if err != nil {
		return nil, err
	}
	maskSecrets(site)
	return site, nil
}

// Create persists a new remote site, encrypting sensitive fields.
func Create(ctx context.Context, req *model.RemoteSiteCreateRequest) (*model.RemoteSite, error) {
	accessToken, err := crypto.Encrypt(req.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("encrypt access token: %w", err)
	}
	password, err := crypto.Encrypt(req.Password)
	if err != nil {
		return nil, fmt.Errorf("encrypt password: %w", err)
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	exchangeRate := req.ExchangeRate
	if exchangeRate == 0 {
		exchangeRate = 7.0
	}
	authType := req.AuthType
	if authType == "" {
		if req.SiteType == model.SiteTypeOctopus {
			authType = model.AuthTypeAccessToken // uses JWT login internally
		} else {
			authType = model.AuthTypeAccessToken
		}
	}

	site := model.RemoteSite{
		Name:         req.Name,
		BaseURL:      req.BaseURL,
		SiteType:     req.SiteType,
		AuthType:     authType,
		AccessToken:  accessToken,
		Username:     req.Username,
		Password:     password,
		ExchangeRate: exchangeRate,
		Enabled:      enabled,
		Tags:         req.Tags,
		Notes:        req.Notes,
		HealthStatus: model.HealthStatusUnknown,
	}

	if err := db.GetDB().WithContext(ctx).Create(&site).Error; err != nil {
		return nil, fmt.Errorf("create remote site: %w", err)
	}

	masked := site
	maskSecrets(&masked)
	return &masked, nil
}

// Update applies partial updates to a remote site.
func Update(ctx context.Context, req *model.RemoteSiteUpdateRequest) (*model.RemoteSite, error) {
	site, err := Get(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})

	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.BaseURL != nil {
		updates["base_url"] = *req.BaseURL
	}
	if req.SiteType != nil {
		updates["site_type"] = *req.SiteType
	}
	if req.AuthType != nil {
		updates["auth_type"] = *req.AuthType
	}
	if req.AccessToken != nil {
		enc, err := crypto.Encrypt(*req.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("encrypt access token: %w", err)
		}
		updates["access_token"] = enc
	}
	if req.Username != nil {
		updates["username"] = *req.Username
	}
	if req.Password != nil {
		enc, err := crypto.Encrypt(*req.Password)
		if err != nil {
			return nil, fmt.Errorf("encrypt password: %w", err)
		}
		updates["password"] = enc
	}
	if req.ExchangeRate != nil {
		updates["exchange_rate"] = *req.ExchangeRate
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Tags != nil {
		updates["tags"] = *req.Tags
	}
	if req.Notes != nil {
		updates["notes"] = *req.Notes
	}
	if req.Pinned != nil {
		updates["pinned"] = *req.Pinned
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}

	if len(updates) > 0 {
		if err := db.GetDB().WithContext(ctx).Model(site).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("update remote site %d: %w", req.ID, err)
		}
	}

	updated, err := Get(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	maskSecrets(updated)
	return updated, nil
}

// Delete removes a remote site by ID.
func Delete(ctx context.Context, id int) error {
	result := db.GetDB().WithContext(ctx).Delete(&model.RemoteSite{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete remote site %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("remote site %d not found", id)
	}
	return nil
}

// Refresh fetches the latest data from a remote site and updates the local record.
func Refresh(ctx context.Context, id int) (*hub.RefreshResult, error) {
	site, err := Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !site.Enabled {
		return nil, fmt.Errorf("remote site %d is disabled", id)
	}

	adapter, err := hub.Get(site.SiteType)
	if err != nil {
		return nil, err
	}

	result := &hub.RefreshResult{
		HealthStatus: model.HealthStatusHealthy,
		SyncedAt:     time.Now(),
	}

	userInfo, err := adapter.FetchUserInfo(ctx, site)
	if err != nil {
		log.Warnf("refresh remote site %d user info: %v", id, err)
		result.HealthStatus = model.HealthStatusError
		result.HealthMsg = err.Error()
	} else if userInfo != nil {
		result.UserInfo = userInfo
		result.Quota = userInfo.Quota
	}

	siteStatus, err := adapter.FetchSiteStatus(ctx, site)
	if err != nil {
		log.Warnf("refresh remote site %d status: %v", id, err)
	} else {
		result.SiteStatus = siteStatus
	}

	now := time.Now()
	updates := map[string]interface{}{
		"health_status":  result.HealthStatus,
		"health_message": result.HealthMsg,
		"last_sync_at":   now,
	}
	if userInfo != nil {
		updates["quota"] = userInfo.Quota
		updates["remote_user_id"] = userInfo.ID
		updates["remote_username"] = userInfo.Username
	}

	if err := db.GetDB().WithContext(ctx).Model(&model.RemoteSite{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		log.Warnf("failed to save refresh result for site %d: %v", id, err)
	}

	go func() {
		bgCtx := context.Background()
		if _, err := SyncTokens(bgCtx, id); err != nil {
			log.Warnf("sync tokens during refresh for site %d: %v", id, err)
		}
		if err := FetchAndStoreAnnouncement(bgCtx, id); err != nil {
			log.Warnf("fetch announcement during refresh for site %d: %v", id, err)
		}
	}()

	return result, nil
}

// RefreshAll refreshes all enabled remote sites.
func RefreshAll(ctx context.Context) (map[int]*hub.RefreshResult, error) {
	var sites []model.RemoteSite
	if err := db.GetDB().WithContext(ctx).Where("enabled = ?", true).Find(&sites).Error; err != nil {
		return nil, fmt.Errorf("list enabled sites: %w", err)
	}

	results := make(map[int]*hub.RefreshResult, len(sites))
	for _, site := range sites {
		r, err := Refresh(ctx, site.ID)
		if err != nil {
			log.Warnf("refresh site %d (%s): %v", site.ID, site.Name, err)
			results[site.ID] = &hub.RefreshResult{
				HealthStatus: model.HealthStatusError,
				HealthMsg:    err.Error(),
				SyncedAt:     time.Now(),
			}
			continue
		}
		results[site.ID] = r
	}
	return results, nil
}

// DetectSiteType attempts to determine the site type from the base URL by
// probing known API endpoints.
func DetectSiteType(ctx context.Context, baseURL, accessToken string) (string, error) {
	tmpSite := &model.RemoteSite{
		BaseURL:     baseURL,
		AuthType:    model.AuthTypeAccessToken,
		AccessToken: accessToken,
	}

	// Try Octopus: POST /api/v1/user/login returns a specific format
	// (we can't test without credentials, so check for /api/v1/channel/list 404 pattern)

	// Try common New API: GET /api/status
	type statusResp struct {
		SystemName string `json:"system_name"`
	}
	noAuth := *tmpSite
	noAuth.AuthType = model.AuthTypeNone
	status, err := hub.FetchJSON[statusResp](ctx, &noAuth, "GET", "/api/status", nil)
	if err == nil {
		name := status.SystemName
		switch {
		case containsAny(name, "veloera"):
			return model.SiteTypeVeloera, nil
		case containsAny(name, "donehub", "done-hub", "done hub"):
			return model.SiteTypeDoneHub, nil
		case containsAny(name, "onehub", "one-hub", "one hub"):
			return model.SiteTypeOneHub, nil
		case containsAny(name, "aihubmix"):
			return model.SiteTypeAIHubMix, nil
		default:
			return model.SiteTypeNewAPI, nil
		}
	}

	return model.SiteTypeUnknown, nil
}

// maskSecrets replaces sensitive fields with masked values for API responses.
func maskSecrets(site *model.RemoteSite) {
	if site.AccessToken != "" {
		site.AccessToken = "***"
	}
	if site.Password != "" {
		site.Password = "***"
	}
}

func containsAny(s string, substrs ...string) bool {
	lower := toLower(s)
	for _, sub := range substrs {
		if contains(lower, toLower(sub)) {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
