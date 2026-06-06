package remotesite

import (
	"context"
	"crypto/md5"
	"fmt"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/hub"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/log"
)

const (
	usageLogPageSize = 100
	usageLogMaxPages = 50 // safety limit
	usageLogMaxDays  = 7  // look back at most 7 days
)

// SyncUsageHistory pulls usage logs from a remote site and stores them locally with dedup.
func SyncUsageHistory(ctx context.Context, siteID int) (int, error) {
	site, err := Get(ctx, siteID)
	if err != nil {
		return 0, err
	}
	if !site.Enabled {
		return 0, fmt.Errorf("remote site %d is disabled", siteID)
	}

	adapter, err := hub.Get(site.SiteType)
	if err != nil {
		return 0, err
	}

	// Find the latest synced record to know where to stop
	var latestRecord model.RemoteUsageRecord
	err = db.GetDB().WithContext(ctx).
		Where("remote_site_id = ?", siteID).
		Order("remote_log_id DESC").
		First(&latestRecord).Error
	var lastSyncedLogID int64
	if err == nil {
		lastSyncedLogID = latestRecord.RemoteLogID
	}

	now := time.Now()
	totalInserted := 0

	for page := 0; page < usageLogMaxPages; page++ {
		logs, err := adapter.FetchUsageLogs(ctx, site, page, usageLogPageSize)
		if err != nil {
			return totalInserted, fmt.Errorf("fetch usage logs page %d: %w", page, err)
		}
		if len(logs) == 0 {
			break
		}

		var batch []model.RemoteUsageRecord
		for _, entry := range logs {
			// Stop if we've seen this log ID before
			if entry.ID > 0 && entry.ID <= lastSyncedLogID {
				goto done
			}

			// Skip entries older than maxDays
			if entry.CreatedAt > 0 {
				entryTime := time.Unix(entry.CreatedAt, 0)
				if now.Sub(entryTime) > time.Duration(usageLogMaxDays)*24*time.Hour {
					goto done
				}
			}

			dayKey := ""
			hour := 0
			if entry.CreatedAt > 0 {
				t := time.Unix(entry.CreatedAt, 0)
				dayKey = t.Format("2006-01-02")
				hour = t.Hour()
			}

			fingerprint := fmt.Sprintf("%d-%d-%d", siteID, entry.ID, entry.CreatedAt)
			hash := fmt.Sprintf("%x", md5.Sum([]byte(fingerprint)))

			batch = append(batch, model.RemoteUsageRecord{
				RemoteSiteID:     siteID,
				DayKey:           dayKey,
				Hour:             hour,
				ModelName:        entry.ModelName,
				TokenName:        entry.TokenName,
				RequestCount:     1,
				PromptTokens:     entry.PromptTokens,
				CompletionTokens: entry.CompletionTokens,
				TotalTokens:      entry.TotalTokens,
				QuotaConsumed:    entry.Quota,
				RemoteLogID:      entry.ID,
				Fingerprint:      hash,
				SyncedAt:         now,
			})
		}

		if len(batch) > 0 {
			inserted := insertUsageRecordsBatch(ctx, batch)
			totalInserted += inserted
		}

		if len(logs) < usageLogPageSize {
			break
		}
	}

done:
	return totalInserted, nil
}

// insertUsageRecordsBatch inserts records, skipping duplicates by fingerprint.
func insertUsageRecordsBatch(ctx context.Context, records []model.RemoteUsageRecord) int {
	if len(records) == 0 {
		return 0
	}

	inserted := 0
	for _, record := range records {
		var count int64
		db.GetDB().WithContext(ctx).
			Model(&model.RemoteUsageRecord{}).
			Where("fingerprint = ?", record.Fingerprint).
			Count(&count)
		if count > 0 {
			continue
		}
		if err := db.GetDB().WithContext(ctx).Create(&record).Error; err != nil {
			log.Warnf("insert usage record: %v", err)
			continue
		}
		inserted++
	}
	return inserted
}

// SyncAllUsageHistory syncs usage history for all enabled sites.
func SyncAllUsageHistory(ctx context.Context) int {
	var sites []model.RemoteSite
	if err := db.GetDB().WithContext(ctx).Where("enabled = ?", true).Find(&sites).Error; err != nil {
		log.Warnf("list sites for usage sync: %v", err)
		return 0
	}

	total := 0
	for _, site := range sites {
		n, err := SyncUsageHistory(ctx, site.ID)
		if err != nil {
			log.Warnf("sync usage for site %d (%s): %v", site.ID, site.Name, err)
			continue
		}
		total += n
	}
	return total
}

// QueryUsageHistory returns usage records matching the query filters.
func QueryUsageHistory(ctx context.Context, q *model.RemoteUsageQuery) ([]model.RemoteUsageRecord, int64, error) {
	tx := db.GetDB().WithContext(ctx).Model(&model.RemoteUsageRecord{})

	if q.SiteID > 0 {
		tx = tx.Where("remote_site_id = ?", q.SiteID)
	}
	if q.DayFrom != "" {
		tx = tx.Where("day_key >= ?", q.DayFrom)
	}
	if q.DayTo != "" {
		tx = tx.Where("day_key <= ?", q.DayTo)
	}
	if q.ModelName != "" {
		tx = tx.Where("model_name = ?", q.ModelName)
	}
	if q.TokenName != "" {
		tx = tx.Where("token_name = ?", q.TokenName)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count usage records: %w", err)
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	var records []model.RemoteUsageRecord
	err := tx.Order("day_key DESC, hour DESC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error
	if err != nil {
		return nil, 0, fmt.Errorf("query usage records: %w", err)
	}

	return records, total, nil
}

// QueryUsageSummary returns aggregated usage grouped by day and optionally model/token.
func QueryUsageSummary(ctx context.Context, q *model.RemoteUsageQuery) ([]model.RemoteUsageSummary, error) {
	tx := db.GetDB().WithContext(ctx).Model(&model.RemoteUsageRecord{})

	if q.SiteID > 0 {
		tx = tx.Where("remote_site_id = ?", q.SiteID)
	}
	if q.DayFrom != "" {
		tx = tx.Where("day_key >= ?", q.DayFrom)
	}
	if q.DayTo != "" {
		tx = tx.Where("day_key <= ?", q.DayTo)
	}
	if q.ModelName != "" {
		tx = tx.Where("model_name = ?", q.ModelName)
	}
	if q.TokenName != "" {
		tx = tx.Where("token_name = ?", q.TokenName)
	}

	var summaries []model.RemoteUsageSummary
	err := tx.Select(`day_key, model_name, token_name,
		SUM(request_count) as request_count,
		SUM(prompt_tokens) as prompt_tokens,
		SUM(completion_tokens) as completion_tokens,
		SUM(total_tokens) as total_tokens,
		SUM(quota_consumed) as quota_consumed`).
		Group("day_key, model_name, token_name").
		Order("day_key DESC").
		Find(&summaries).Error
	if err != nil {
		return nil, fmt.Errorf("query usage summary: %w", err)
	}
	return summaries, nil
}

// QueryUsageHourly returns hourly aggregated usage for a specific day.
func QueryUsageHourly(ctx context.Context, siteID int, dayKey string) ([]model.RemoteUsageHourly, error) {
	tx := db.GetDB().WithContext(ctx).Model(&model.RemoteUsageRecord{})

	if siteID > 0 {
		tx = tx.Where("remote_site_id = ?", siteID)
	}
	if dayKey != "" {
		tx = tx.Where("day_key = ?", dayKey)
	}

	var hourly []model.RemoteUsageHourly
	err := tx.Select(`hour,
		SUM(request_count) as request_count,
		SUM(prompt_tokens) as prompt_tokens,
		SUM(completion_tokens) as completion_tokens,
		SUM(total_tokens) as total_tokens`).
		Group("hour").
		Order("hour ASC").
		Find(&hourly).Error
	if err != nil {
		return nil, fmt.Errorf("query usage hourly: %w", err)
	}
	return hourly, nil
}

// GetUsageModels returns distinct model names used in usage records for a site.
func GetUsageModels(ctx context.Context, siteID int) ([]string, error) {
	var models []string
	err := db.GetDB().WithContext(ctx).
		Model(&model.RemoteUsageRecord{}).
		Where("remote_site_id = ? AND model_name != ''", siteID).
		Distinct("model_name").
		Order("model_name").
		Pluck("model_name", &models).Error
	if err != nil {
		return nil, fmt.Errorf("get usage models: %w", err)
	}
	return models, nil
}
