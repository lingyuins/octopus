package stats_test

import (
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/op/stats"
)

func TestNowUsesRuntimeLocationWhenTimezoneOffsetIsZero(t *testing.T) {
	cleanup := stats.SetTimeNowForTest(func() time.Time {
		return time.Date(2026, 5, 4, 15, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	})
	defer cleanup()

	setting.GetCache().Clear()
	setting.GetCache().Set(model.SettingKeyStatsTimezoneOffset, "0")

	now := stats.Now()
	if now.Hour() != 15 {
		t.Fatalf("expected runtime local hour 15 when timezone offset is 0, got %d (%s)", now.Hour(), now.Format(time.RFC3339))
	}
}

func TestNowUsesConfiguredTimezoneOffsetWhenProvided(t *testing.T) {
	cleanup := stats.SetTimeNowForTest(func() time.Time {
		return time.Date(2026, 5, 4, 7, 0, 0, 0, time.UTC)
	})
	defer cleanup()

	setting.GetCache().Clear()
	setting.GetCache().Set(model.SettingKeyStatsTimezoneOffset, "8")

	now := stats.Now()
	if now.Hour() != 15 {
		t.Fatalf("expected configured UTC+8 hour 15, got %d (%s)", now.Hour(), now.Format(time.RFC3339))
	}
}
