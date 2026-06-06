package model

import "time"

// RemoteUsageRecord stores a usage log entry pulled from a remote site.
type RemoteUsageRecord struct {
	ID               int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	RemoteSiteID     int       `json:"remote_site_id" gorm:"not null;index"`
	DayKey           string    `json:"day_key" gorm:"not null;index"` // YYYY-MM-DD
	Hour             int       `json:"hour"`                          // 0-23
	ModelName        string    `json:"model_name" gorm:"size:255;index"`
	TokenName        string    `json:"token_name" gorm:"size:255"`
	RequestCount     int64     `json:"request_count" gorm:"default:1"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	TotalTokens      int64     `json:"total_tokens"`
	QuotaConsumed    float64   `json:"quota_consumed"`
	RemoteLogID      int64     `json:"remote_log_id"`                           // original log ID from remote site
	Fingerprint      string    `json:"fingerprint" gorm:"uniqueIndex;size:128"` // dedup key
	SyncedAt         time.Time `json:"synced_at"`
}

// RemoteUsageSummary is the aggregated usage for a given day/model/token.
type RemoteUsageSummary struct {
	DayKey           string  `json:"day_key"`
	ModelName        string  `json:"model_name,omitempty"`
	TokenName        string  `json:"token_name,omitempty"`
	RequestCount     int64   `json:"request_count"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	QuotaConsumed    float64 `json:"quota_consumed"`
}

// RemoteUsageHourly is hourly aggregated usage.
type RemoteUsageHourly struct {
	Hour             int   `json:"hour"`
	RequestCount     int64 `json:"request_count"`
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// RemoteUsageQuery defines filter parameters for querying usage history.
type RemoteUsageQuery struct {
	SiteID    int    `json:"site_id"`
	DayFrom   string `json:"day_from"` // YYYY-MM-DD
	DayTo     string `json:"day_to"`   // YYYY-MM-DD
	ModelName string `json:"model_name"`
	TokenName string `json:"token_name"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

func (RemoteUsageRecord) TableName() string { return "remote_usage_records" }
