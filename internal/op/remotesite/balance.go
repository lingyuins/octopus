package remotesite

import (
	"context"
	"fmt"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// ListBalanceSnapshots returns snapshots for a site within a date range.
// If startDate or endDate are empty, no date filtering is applied for that bound.
func ListBalanceSnapshots(ctx context.Context, siteID int, startDate, endDate string) ([]model.BalanceSnapshot, error) {
	q := db.GetDB().WithContext(ctx).
		Where("remote_site_id = ?", siteID).
		Order("day_key ASC, captured_at ASC")

	if startDate != "" {
		q = q.Where("day_key >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("day_key <= ?", endDate)
	}

	var snapshots []model.BalanceSnapshot
	if err := q.Find(&snapshots).Error; err != nil {
		return nil, fmt.Errorf("list balance snapshots: %w", err)
	}
	return snapshots, nil
}

// GetBalanceChartData returns one data point per day (latest snapshot per day) for chart rendering.
func GetBalanceChartData(ctx context.Context, siteID int, startDate, endDate string) ([]model.BalanceChartPoint, error) {
	snapshots, err := ListBalanceSnapshots(ctx, siteID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Keep the latest snapshot per day
	dayMap := make(map[string]model.BalanceSnapshot)
	for _, s := range snapshots {
		if existing, ok := dayMap[s.DayKey]; !ok || s.CapturedAt.After(existing.CapturedAt) {
			dayMap[s.DayKey] = s
		}
	}

	points := make([]model.BalanceChartPoint, 0, len(dayMap))
	for _, s := range snapshots {
		if best, ok := dayMap[s.DayKey]; ok && best.ID == s.ID {
			points = append(points, model.BalanceChartPoint{
				DayKey: s.DayKey,
				Quota:  s.Quota,
			})
			delete(dayMap, s.DayKey)
		}
	}
	return points, nil
}

// CaptureBalanceSnapshot creates a snapshot for a site using its current quota value.
func CaptureBalanceSnapshot(ctx context.Context, siteID int, source string) (*model.BalanceSnapshot, error) {
	site, err := Get(ctx, siteID)
	if err != nil {
		return nil, err
	}

	snapshot := model.BalanceSnapshot{
		RemoteSiteID: siteID,
		DayKey:       time.Now().Format("2006-01-02"),
		Quota:        site.Quota,
		CapturedAt:   time.Now(),
		Source:       source,
	}

	if err := db.GetDB().WithContext(ctx).Create(&snapshot).Error; err != nil {
		return nil, fmt.Errorf("create balance snapshot: %w", err)
	}
	return &snapshot, nil
}

// CaptureAllBalanceSnapshots captures balance for all enabled sites.
func CaptureAllBalanceSnapshots(ctx context.Context) int {
	var sites []model.RemoteSite
	if err := db.GetDB().WithContext(ctx).Where("enabled = ?", true).Find(&sites).Error; err != nil {
		log.Warnf("list sites for balance capture: %v", err)
		return 0
	}

	count := 0
	for _, site := range sites {
		if _, err := CaptureBalanceSnapshot(ctx, site.ID, "scheduled"); err != nil {
			log.Warnf("capture balance for site %d: %v", site.ID, err)
			continue
		}
		count++
	}
	return count
}

// CleanOldSnapshots removes snapshots older than the given number of days.
func CleanOldSnapshots(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).Format("2006-01-02")
	result := db.GetDB().WithContext(ctx).
		Where("day_key < ?", cutoff).
		Delete(&model.BalanceSnapshot{})
	if result.Error != nil {
		return 0, fmt.Errorf("clean old snapshots: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// PredictBalance analyzes historical balance data and predicts future consumption trends.
func PredictBalance(ctx context.Context, siteID int) (*model.BalancePrediction, error) {
	// Get last 30 days of snapshots
	startDate := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	points, err := GetBalanceChartData(ctx, siteID, startDate, "")
	if err != nil {
		return nil, fmt.Errorf("get chart data: %w", err)
	}

	if len(points) < 2 {
		return nil, fmt.Errorf("insufficient data for prediction (need at least 2 days)")
	}

	// Calculate daily consumption rates
	var dailyBurns []float64
	for i := 1; i < len(points); i++ {
		prev := points[i-1]
		curr := points[i]

		// Parse dates to calculate days between
		prevDate, _ := time.Parse("2006-01-02", prev.DayKey)
		currDate, _ := time.Parse("2006-01-02", curr.DayKey)
		daysBetween := currDate.Sub(prevDate).Hours() / 24

		if daysBetween > 0 {
			burn := (prev.Quota - curr.Quota) / daysBetween
			if burn > 0 { // Only count positive burns (consumption)
				dailyBurns = append(dailyBurns, burn)
			}
		}
	}

	if len(dailyBurns) == 0 {
		return nil, fmt.Errorf("no consumption data available")
	}

	// Calculate averages
	sevenDayBurn := avgFloat64(dailyBurns, 7)
	thirtyDayBurn := avgFloat64(dailyBurns, 30)

	// Use 7-day average as primary prediction (more responsive to recent trends)
	primaryBurnRate := sevenDayBurn

	// Get current quota from latest point
	currentQuota := points[len(points)-1].Quota

	// Calculate days remaining
	var daysRemaining int
	var estimatedZeroAt string
	if primaryBurnRate > 0 {
		daysRemaining = int(currentQuota / primaryBurnRate)
		zeroDate := time.Now().AddDate(0, 0, daysRemaining)
		estimatedZeroAt = zeroDate.Format("2006-01-02")
	} else {
		daysRemaining = -1 // Infinite
		estimatedZeroAt = ""
	}

	// Generate trend points for next 30 days
	var trendPoints []model.BalanceChartPoint
	lastDate := points[len(points)-1].DayKey
	lastQuota := currentQuota

	for i := 1; i <= 30; i++ {
		nextDate, _ := time.Parse("2006-01-02", lastDate)
		nextDate = nextDate.AddDate(0, 0, i)

		predictedQuota := lastQuota - (primaryBurnRate * float64(i))
		if predictedQuota < 0 {
			predictedQuota = 0
		}

		trendPoints = append(trendPoints, model.BalanceChartPoint{
			DayKey: nextDate.Format("2006-01-02"),
			Quota:  predictedQuota,
		})

		if predictedQuota <= 0 {
			break // Stop predicting after quota reaches zero
		}
	}

	return &model.BalancePrediction{
		DailyBurnRate:    primaryBurnRate,
		DaysRemaining:    daysRemaining,
		EstimatedZeroAt:  estimatedZeroAt,
		SevenDayAvgBurn:  sevenDayBurn,
		ThirtyDayAvgBurn: thirtyDayBurn,
		CurrentQuota:     currentQuota,
		TrendPoints:      trendPoints,
	}, nil
}

// avgFloat64 calculates the average of the last n elements from a slice.
func avgFloat64(values []float64, n int) float64 {
	if len(values) == 0 {
		return 0
	}

	start := 0
	if len(values) > n {
		start = len(values) - n
	}

	subset := values[start:]
	if len(subset) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range subset {
		sum += v
	}
	return sum / float64(len(subset))
}
