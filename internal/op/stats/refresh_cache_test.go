package stats

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

func TestRefreshCacheSkipsNonTodayHourlyStats(t *testing.T) {
	cleanupNow := SetTimeNowForTest(func() time.Time {
		return time.Date(2026, 5, 20, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	})
	defer cleanupNow()

	setting.GetCache().Clear()
	setting.GetCache().Set(model.SettingKeyStatsTimezoneOffset, "8")

	testName := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", testName)
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()

	dbConn := db.GetDB().WithContext(context.Background())
	if err := dbConn.Create(&model.StatsHourly{
		Hour: 9,
		Date: "20260519",
		StatsMetrics: model.StatsMetrics{RequestSuccess: 99, InputToken: 999},
	}).Error; err != nil {
		t.Fatalf("create yesterday hourly: %v", err)
	}

	if err := RefreshCache(context.Background()); err != nil {
		t.Fatalf("refresh cache: %v", err)
	}

	hourly := HourlyGet()
	if len(hourly) != 11 {
		t.Fatalf("expected 11 hourly entries up to current hour, got %d", len(hourly))
	}
	if hourly[9].Date != "20260520" {
		t.Fatalf("expected hour 9 placeholder to use today's date, got %q", hourly[9].Date)
	}
	if hourly[9].RequestSuccess != 0 || hourly[9].InputToken != 0 {
		t.Fatalf("expected hour 9 placeholder to be empty, got %+v", hourly[9])
	}
}

func TestStatsHourlyCompositePrimaryKeyAllowsMultipleDatesPerHour(t *testing.T) {
	cleanupNow := SetTimeNowForTest(func() time.Time {
		return time.Date(2026, 5, 20, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	})
	defer cleanupNow()

	setting.GetCache().Clear()
	setting.GetCache().Set(model.SettingKeyStatsTimezoneOffset, "8")

	testName := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", testName)
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()

	dbConn := db.GetDB().WithContext(context.Background())
	entries := []model.StatsHourly{
		{Hour: 9, Date: "20260519", StatsMetrics: model.StatsMetrics{RequestSuccess: 1}},
		{Hour: 9, Date: "20260520", StatsMetrics: model.StatsMetrics{RequestSuccess: 2}},
	}
	for _, entry := range entries {
		if err := dbConn.Create(&entry).Error; err != nil {
			t.Fatalf("create hourly entry %+v: %v", entry, err)
		}
	}

	var count int64
	if err := dbConn.Model(&model.StatsHourly{}).Where("hour = ?", 9).Count(&count).Error; err != nil {
		t.Fatalf("count hourly entries: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 hourly entries for hour 9 across dates, got %d", count)
	}
}
