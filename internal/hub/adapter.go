// Package hub provides adapters for communicating with remote AI relay sites.
// Each site type (new-api, veloera, octopus, etc.) implements the SiteAdapter
// interface; the common adapter covers the One API / New API family that most
// sites are compatible with.
package hub

import (
	"context"
	"time"

	"github.com/lingyuins/octopus/internal/model"
)

// UserInfoResult is the data returned by FetchUserInfo.
type UserInfoResult struct {
	ID          int     `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	Email       string  `json:"email"`
	Role        int     `json:"role"`
	Status      int     `json:"status"`
	Quota       float64 `json:"quota"`
	UsedQuota   float64 `json:"used_quota"`
	AccessToken string  `json:"access_token"`
}

// CheckInResult is returned by PerformCheckIn.
type CheckInResult struct {
	Success      bool    `json:"success"`
	Message      string  `json:"message"`
	QuotaAwarded float64 `json:"quota_awarded"`
	AlreadyDone  bool    `json:"already_done"`
}

// ModelPricingEntry describes the pricing for one model on a remote site.
type ModelPricingEntry struct {
	ModelName       string  `json:"model_name"`
	Quota           float64 `json:"quota"`
	CompletionRatio float64 `json:"completion_ratio"`
	GroupRatio      float64 `json:"group_ratio"`
}

// RemoteToken represents an API token on the remote site.
type RemoteToken struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	Key            string  `json:"key"`
	Status         int     `json:"status"`
	RemainQuota    float64 `json:"remain_quota"`
	UsedQuota      float64 `json:"used_quota"`
	UnlimitedQuota bool    `json:"unlimited_quota"`
	ModelLimits    string  `json:"model_limits"`
	ExpiredTime    int64   `json:"expired_time"`
	CreatedTime    int64   `json:"created_time"`
}

// CreateTokenRequest is the payload for creating a token on the remote site.
type CreateTokenRequest struct {
	Name           string `json:"name"`
	RemainQuota    int64  `json:"remain_quota"`
	UnlimitedQuota bool   `json:"unlimited_quota"`
	ExpiredTime    int64  `json:"expired_time"`
	ModelLimits    string `json:"model_limits_enabled"`
}

// RemoteChannel represents a channel on a remote managed site.
type RemoteChannel struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Type    int    `json:"type"`
	Status  int    `json:"status"`
	Models  string `json:"models"`
	BaseURL string `json:"base_url"`
	Group   string `json:"group"`
}

// RemoteChannelCreateReq is the payload for creating a channel on a remote site.
type RemoteChannelCreateReq struct {
	Name    string `json:"name"`
	Type    int    `json:"type"`
	Key     string `json:"key"`
	BaseURL string `json:"base_url"`
	Models  string `json:"models"`
	Group   string `json:"group"`
}

// RemoteChannelUpdateReq is the payload for updating a channel on a remote site.
type RemoteChannelUpdateReq struct {
	ID     int    `json:"id"`
	Models string `json:"models"`
}

// SiteStatusInfo holds the public status of a remote site.
type SiteStatusInfo struct {
	CheckInEnabled bool    `json:"checkin_enabled"`
	Price          float64 `json:"price"`
	SystemName     string  `json:"system_name"`
}

// RedeemResult is returned by RedeemCode.
type RedeemResult struct {
	Success      bool    `json:"success"`
	Message      string  `json:"message"`
	QuotaAwarded float64 `json:"quota_awarded"`
	AlreadyUsed  bool    `json:"already_used"`
}

// SiteAdapter is the interface that every remote site type must implement.
// The common adapter provides defaults for the One API / New API family.
type SiteAdapter interface {
	// FetchUserInfo returns the account information on the remote site.
	FetchUserInfo(ctx context.Context, site *model.RemoteSite) (*UserInfoResult, error)

	// PerformCheckIn executes a check-in on the remote site.
	PerformCheckIn(ctx context.Context, site *model.RemoteSite) (*CheckInResult, error)

	// FetchCheckInStatus returns whether check-in is available today.
	// Returns nil when the site does not support check-in.
	FetchCheckInStatus(ctx context.Context, site *model.RemoteSite) (*bool, error)

	// FetchModels returns the list of model names available on the remote site.
	FetchModels(ctx context.Context, site *model.RemoteSite) ([]string, error)

	// FetchModelPricing returns per-model pricing info.
	FetchModelPricing(ctx context.Context, site *model.RemoteSite) ([]ModelPricingEntry, error)

	// FetchTokens lists API tokens on the remote site.
	FetchTokens(ctx context.Context, site *model.RemoteSite) ([]RemoteToken, error)

	// CreateToken creates a new API token on the remote site.
	CreateToken(ctx context.Context, site *model.RemoteSite, req CreateTokenRequest) error

	// ListChannels lists channels on the remote managed site.
	ListChannels(ctx context.Context, site *model.RemoteSite) ([]RemoteChannel, error)

	// CreateChannel creates a channel on the remote managed site.
	CreateChannel(ctx context.Context, site *model.RemoteSite, ch RemoteChannelCreateReq) error

	// UpdateChannel updates a channel on the remote managed site.
	UpdateChannel(ctx context.Context, site *model.RemoteSite, ch RemoteChannelUpdateReq) error

	// DeleteChannel deletes a channel on the remote managed site.
	DeleteChannel(ctx context.Context, site *model.RemoteSite, channelID int) error

	// FetchAnnouncement returns the site notice/announcement text.
	FetchAnnouncement(ctx context.Context, site *model.RemoteSite) (string, error)

	// FetchSiteStatus returns the public status of the remote site.
	FetchSiteStatus(ctx context.Context, site *model.RemoteSite) (*SiteStatusInfo, error)

	// RedeemCode redeems a code on the remote site.
	// Returns nil when the site does not support redemption.
	RedeemCode(ctx context.Context, site *model.RemoteSite, code string) (*RedeemResult, error)

	// FetchUsageLogs fetches usage logs from the remote site with pagination.
	// Returns nil when the site does not support usage log retrieval.
	FetchUsageLogs(ctx context.Context, site *model.RemoteSite, page, pageSize int) ([]RemoteUsageLog, error)
}

// RemoteUsageLog represents a single usage/log entry from a remote site.
type RemoteUsageLog struct {
	ID               int64   `json:"id"`
	CreatedAt        int64   `json:"created_at"`
	ModelName        string  `json:"model_name"`
	TokenName        string  `json:"token_name"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	Quota            float64 `json:"quota"`
}

// RefreshResult aggregates the outcome of refreshing a remote site's data.
type RefreshResult struct {
	UserInfo     *UserInfoResult `json:"user_info,omitempty"`
	SiteStatus   *SiteStatusInfo `json:"site_status,omitempty"`
	Quota        float64         `json:"quota"`
	HealthStatus string          `json:"health_status"`
	HealthMsg    string          `json:"health_message"`
	SyncedAt     time.Time       `json:"synced_at"`
}
