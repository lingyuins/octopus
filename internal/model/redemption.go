package model

import "time"

// RedemptionRecord stores the result of a redemption code attempt for a remote site.
type RedemptionRecord struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	RemoteSiteID int       `json:"remote_site_id" gorm:"not null;index"`
	Code         string    `json:"code" gorm:"not null;size:255"`
	Status       string    `json:"status"` // success, already_used, invalid, failed
	QuotaAwarded float64   `json:"quota_awarded"`
	Message      string    `json:"message"`
	ExecutedAt   time.Time `json:"executed_at"`
}

// Redemption status constants.
const (
	RedemptionStatusSuccess     = "success"
	RedemptionStatusAlreadyUsed = "already_used"
	RedemptionStatusInvalid     = "invalid"
	RedemptionStatusFailed      = "failed"
)

// RedemptionRequest is the payload for redeeming codes on a remote site.
type RedemptionRequest struct {
	SiteID int      `json:"site_id" binding:"required"`
	Codes  []string `json:"codes" binding:"required"`
}

// RedemptionBatchResult aggregates the outcome of redeeming multiple codes.
type RedemptionBatchResult struct {
	TotalCodes   int                `json:"total_codes"`
	SuccessCount int                `json:"success_count"`
	FailedCount  int                `json:"failed_count"`
	Results      []RedemptionRecord `json:"results"`
}
