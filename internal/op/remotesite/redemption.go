package remotesite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/hub"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// RedeemCode redeems a single code on a remote site and records the result.
func RedeemCode(ctx context.Context, siteID int, code string) (*model.RedemptionRecord, error) {
	site, err := Get(ctx, siteID)
	if err != nil {
		return nil, err
	}
	if !site.Enabled {
		return nil, fmt.Errorf("remote site %d is disabled", siteID)
	}

	adapter, err := hub.Get(site.SiteType)
	if err != nil {
		return nil, err
	}

	result, err := adapter.RedeemCode(ctx, site, code)
	if err != nil {
		record := &model.RedemptionRecord{
			RemoteSiteID: siteID,
			Code:         code,
			Status:       model.RedemptionStatusFailed,
			Message:      err.Error(),
			ExecutedAt:   time.Now(),
		}
		_ = db.GetDB().WithContext(ctx).Create(record).Error
		return record, err
	}

	// nil result means the adapter does not support redemption
	if result == nil {
		return nil, fmt.Errorf("site type %s does not support redemption", site.SiteType)
	}

	status := model.RedemptionStatusSuccess
	if result.AlreadyUsed {
		status = model.RedemptionStatusAlreadyUsed
	} else if !result.Success {
		// Distinguish "invalid code" from generic failure by message heuristics.
		if isInvalidCodeMessage(result.Message) {
			status = model.RedemptionStatusInvalid
		} else {
			status = model.RedemptionStatusFailed
		}
	}

	record := &model.RedemptionRecord{
		RemoteSiteID: siteID,
		Code:         code,
		Status:       status,
		QuotaAwarded: result.QuotaAwarded,
		Message:      result.Message,
		ExecutedAt:   time.Now(),
	}

	if err := db.GetDB().WithContext(ctx).Create(record).Error; err != nil {
		return nil, fmt.Errorf("save redemption record: %w", err)
	}
	return record, nil
}

// isInvalidCodeMessage returns true when the error message indicates the code itself is
// invalid or unknown (rather than a transient failure).
func isInvalidCodeMessage(msg string) bool {
	if msg == "" {
		return false
	}
	for _, kw := range []string{"invalid", "not found", "not exist", "unknown", "不存在", "无效", "错误"} {
		if strings.Contains(strings.ToLower(msg), kw) {
			return true
		}
	}
	return false
}

// RedeemCodes redeems multiple codes on a remote site and returns aggregated results.
func RedeemCodes(ctx context.Context, req *model.RedemptionRequest) (*model.RedemptionBatchResult, error) {
	var results []model.RedemptionRecord
	successCount := 0
	failedCount := 0

	for _, code := range req.Codes {
		record, err := RedeemCode(ctx, req.SiteID, code)
		if err != nil && record == nil {
			// Create a synthetic failure record
			record = &model.RedemptionRecord{
				RemoteSiteID: req.SiteID,
				Code:         code,
				Status:       model.RedemptionStatusFailed,
				Message:      err.Error(),
				ExecutedAt:   time.Now(),
			}
		}
		if record != nil {
			results = append(results, *record)
			if record.Status == model.RedemptionStatusSuccess {
				successCount++
			} else {
				failedCount++
			}
		}
	}

	return &model.RedemptionBatchResult{
		TotalCodes:   len(req.Codes),
		SuccessCount: successCount,
		FailedCount:  failedCount,
		Results:      results,
	}, nil
}

// ListRedemptionHistory returns redemption records for a site.
func ListRedemptionHistory(ctx context.Context, siteID int, limit int) ([]model.RedemptionRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	var records []model.RedemptionRecord
	err := db.GetDB().WithContext(ctx).
		Where("remote_site_id = ?", siteID).
		Order("executed_at DESC").
		Limit(limit).
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("list redemption history: %w", err)
	}
	return records, nil
}

// RedeemAllSites redeems the same code(s) on all enabled sites.
func RedeemAllSites(ctx context.Context, codes []string) []model.RedemptionRecord {
	var sites []model.RemoteSite
	if err := db.GetDB().WithContext(ctx).Where("enabled = ?", true).Find(&sites).Error; err != nil {
		log.Warnf("list sites for redemption: %v", err)
		return nil
	}

	var records []model.RedemptionRecord
	for _, site := range sites {
		for _, code := range codes {
			record, err := RedeemCode(ctx, site.ID, code)
			if err != nil {
				log.Warnf("redeem site %d (%s) code %s: %v", site.ID, site.Name, code, err)
			}
			if record != nil {
				records = append(records, *record)
			}
		}
	}
	return records
}
