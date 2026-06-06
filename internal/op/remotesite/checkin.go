package remotesite

import (
	"context"
	"fmt"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/hub"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// ExecuteCheckIn performs a check-in on a remote site and records the result.
func ExecuteCheckIn(ctx context.Context, siteID int) (*model.CheckInRecord, error) {
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

	today := time.Now().Format("2006-01-02")

	result, err := adapter.PerformCheckIn(ctx, site)
	if err != nil {
		record := &model.CheckInRecord{
			RemoteSiteID: siteID,
			CheckInDate:  today,
			Status:       model.CheckInStatusFailed,
			Message:      err.Error(),
			ExecutedAt:   time.Now(),
		}
		_ = db.GetDB().WithContext(ctx).Create(record).Error
		return record, err
	}

	status := model.CheckInStatusSuccess
	if result.AlreadyDone {
		status = model.CheckInStatusAlreadyChecked
	} else if !result.Success {
		status = model.CheckInStatusFailed
	}

	record := &model.CheckInRecord{
		RemoteSiteID: siteID,
		CheckInDate:  today,
		Status:       status,
		Message:      result.Message,
		QuotaAwarded: result.QuotaAwarded,
		ExecutedAt:   time.Now(),
	}

	if err := db.GetDB().WithContext(ctx).Create(record).Error; err != nil {
		return nil, fmt.Errorf("save check-in record: %w", err)
	}
	return record, nil
}

// ExecuteCheckInAll performs check-in for all enabled sites.
func ExecuteCheckInAll(ctx context.Context) []model.CheckInRecord {
	var sites []model.RemoteSite
	if err := db.GetDB().WithContext(ctx).Where("enabled = ?", true).Find(&sites).Error; err != nil {
		log.Warnf("list sites for check-in: %v", err)
		return nil
	}

	var records []model.CheckInRecord
	for _, site := range sites {
		record, err := ExecuteCheckIn(ctx, site.ID)
		if err != nil {
			log.Warnf("check-in site %d (%s): %v", site.ID, site.Name, err)
		}
		if record != nil {
			records = append(records, *record)
		}
	}
	return records
}

// ListCheckInHistory returns check-in records for a site.
func ListCheckInHistory(ctx context.Context, siteID int, limit int) ([]model.CheckInRecord, error) {
	if limit <= 0 {
		limit = 30
	}
	var records []model.CheckInRecord
	err := db.GetDB().WithContext(ctx).
		Where("remote_site_id = ?", siteID).
		Order("executed_at DESC").
		Limit(limit).
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("list check-in history: %w", err)
	}
	return records, nil
}

// GetTodayCheckInStatus returns whether a site has already checked in today.
func GetTodayCheckInStatus(ctx context.Context, siteID int) (*model.CheckInRecord, error) {
	today := time.Now().Format("2006-01-02")
	var record model.CheckInRecord
	err := db.GetDB().WithContext(ctx).
		Where("remote_site_id = ? AND check_in_date = ?", siteID, today).
		Order("executed_at DESC").
		First(&record).Error
	if err != nil {
		return nil, nil // not found = not checked in
	}
	return &record, nil
}
