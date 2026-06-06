package model

import "time"

// SiteType constants for known remote AI relay site backends.
const (
	SiteTypeNewAPI        = "new-api"
	SiteTypeVeloera       = "veloera"
	SiteTypeDoneHub       = "done-hub"
	SiteTypeOneHub        = "one-hub"
	SiteTypeSub2API       = "sub2api"
	SiteTypeOctopus       = "octopus"
	SiteTypeAnyRouter     = "anyrouter"
	SiteTypeAIHubMix      = "aihubmix"
	SiteTypeAxonHub       = "axonhub"
	SiteTypeClaudeCodeHub = "claude-code-hub"
	SiteTypeSAPI          = "sapi"
	SiteTypeUnknown       = "unknown"
)

// AuthType constants for remote site authentication methods.
const (
	AuthTypeAccessToken = "access_token"
	AuthTypeNone        = "none"
)

// Health status constants.
const (
	HealthStatusUnknown = "unknown"
	HealthStatusHealthy = "healthy"
	HealthStatusWarning = "warning"
	HealthStatusError   = "error"
)

// RemoteSite represents a remote AI relay site managed by the Hub feature.
type RemoteSite struct {
	ID             int        `json:"id" gorm:"primaryKey"`
	Name           string     `json:"name" gorm:"not null"`
	BaseURL        string     `json:"base_url" gorm:"not null"`
	SiteType       string     `json:"site_type" gorm:"not null;default:'new-api'"`
	AuthType       string     `json:"auth_type" gorm:"not null;default:'access_token'"`
	AccessToken    string     `json:"access_token,omitempty"`
	Username       string     `json:"username,omitempty"`
	Password       string     `json:"password,omitempty"`
	ExchangeRate   float64    `json:"exchange_rate" gorm:"default:7.0"`
	Enabled        bool       `json:"enabled" gorm:"default:true"`
	Tags           string     `json:"tags"`
	Notes          string     `json:"notes"`
	Pinned         bool       `json:"pinned" gorm:"default:false"`
	SortOrder      int        `json:"sort_order" gorm:"default:0"`
	RemoteUserID   int        `json:"remote_user_id"`
	RemoteUsername string     `json:"remote_username"`
	Quota          float64    `json:"quota"`
	HealthStatus   string     `json:"health_status" gorm:"default:'unknown'"`
	HealthMessage  string     `json:"health_message"`
	LastSyncAt     *time.Time `json:"last_sync_at"`
	CreatedAt      time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// RemoteSiteCreateRequest is the payload for creating a remote site.
type RemoteSiteCreateRequest struct {
	Name         string  `json:"name" binding:"required"`
	BaseURL      string  `json:"base_url" binding:"required"`
	SiteType     string  `json:"site_type" binding:"required"`
	AuthType     string  `json:"auth_type"`
	AccessToken  string  `json:"access_token"`
	Username     string  `json:"username"`
	Password     string  `json:"password"`
	ExchangeRate float64 `json:"exchange_rate"`
	Enabled      *bool   `json:"enabled"`
	Tags         string  `json:"tags"`
	Notes        string  `json:"notes"`
}

func (RemoteSiteCreateRequest) TableName() string { return "-" }

// RemoteSiteUpdateRequest is the payload for updating a remote site.
type RemoteSiteUpdateRequest struct {
	ID           int      `json:"id" binding:"required"`
	Name         *string  `json:"name,omitempty"`
	BaseURL      *string  `json:"base_url,omitempty"`
	SiteType     *string  `json:"site_type,omitempty"`
	AuthType     *string  `json:"auth_type,omitempty"`
	AccessToken  *string  `json:"access_token,omitempty"`
	Username     *string  `json:"username,omitempty"`
	Password     *string  `json:"password,omitempty"`
	ExchangeRate *float64 `json:"exchange_rate,omitempty"`
	Enabled      *bool    `json:"enabled,omitempty"`
	Tags         *string  `json:"tags,omitempty"`
	Notes        *string  `json:"notes,omitempty"`
	Pinned       *bool    `json:"pinned,omitempty"`
	SortOrder    *int     `json:"sort_order,omitempty"`
}

func (RemoteSiteUpdateRequest) TableName() string { return "-" }

// RemoteSiteDetectRequest is the payload for auto-detecting a site type.
type RemoteSiteDetectRequest struct {
	BaseURL     string `json:"base_url" binding:"required"`
	AccessToken string `json:"access_token"`
}

func (RemoteSiteDetectRequest) TableName() string { return "-" }

// ── Balance tracking ────────────────────────────────────────────────────────

// BalanceSnapshot records a point-in-time balance for a remote site.
type BalanceSnapshot struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	RemoteSiteID int       `json:"remote_site_id" gorm:"not null;index"`
	DayKey       string    `json:"day_key" gorm:"not null;index"` // YYYY-MM-DD
	Quota        float64   `json:"quota"`
	CapturedAt   time.Time `json:"captured_at"`
	Source       string    `json:"source"` // "refresh" | "scheduled"
}

// BalanceChartPoint is a lightweight DTO for chart rendering.
type BalanceChartPoint struct {
	DayKey string  `json:"day_key"`
	Quota  float64 `json:"quota"`
}

func (BalanceChartPoint) TableName() string { return "-" }

// BalancePrediction is the response type for balance consumption prediction.
type BalancePrediction struct {
	DailyBurnRate    float64             `json:"daily_burn_rate"`     // average daily consumption (7-day weighted)
	DaysRemaining    int                 `json:"days_remaining"`      // estimated days until quota reaches 0
	EstimatedZeroAt  string              `json:"estimated_zero_at"`   // YYYY-MM-DD when quota hits 0
	SevenDayAvgBurn  float64             `json:"seven_day_avg_burn"`  // 7-day average daily burn
	ThirtyDayAvgBurn float64             `json:"thirty_day_avg_burn"` // 30-day average daily burn
	CurrentQuota     float64             `json:"current_quota"`       // current quota
	TrendPoints      []BalanceChartPoint `json:"trend_points"`        // future prediction data points
}

func (BalancePrediction) TableName() string { return "-" }

// ── Check-in ────────────────────────────────────────────────────────────────

// CheckInRecord stores the result of a check-in attempt for a remote site.
type CheckInRecord struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	RemoteSiteID int       `json:"remote_site_id" gorm:"not null;index"`
	CheckInDate  string    `json:"check_in_date" gorm:"not null"` // YYYY-MM-DD
	Status       string    `json:"status"`                        // success, already_checked, failed
	Message      string    `json:"message"`
	QuotaAwarded float64   `json:"quota_awarded"`
	ExecutedAt   time.Time `json:"executed_at"`
}

// Check-in status constants.
const (
	CheckInStatusSuccess        = "success"
	CheckInStatusAlreadyChecked = "already_checked"
	CheckInStatusFailed         = "failed"
)

// AllSiteTypes returns a list of all known site types.
func AllSiteTypes() []string {
	return []string{
		SiteTypeNewAPI,
		SiteTypeVeloera,
		SiteTypeDoneHub,
		SiteTypeOneHub,
		SiteTypeSub2API,
		SiteTypeOctopus,
		SiteTypeAnyRouter,
		SiteTypeAIHubMix,
		SiteTypeAxonHub,
		SiteTypeClaudeCodeHub,
		SiteTypeSAPI,
		SiteTypeUnknown,
	}
}
