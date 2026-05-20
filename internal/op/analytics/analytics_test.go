package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/relaylog"
	"github.com/lingyuins/octopus/internal/op/setting"
)

// ── buildAnalyticsOverview ──

func TestBuildAnalyticsOverview_NoData(t *testing.T) {
	got := buildAnalyticsOverview(model.StatsMetrics{}, 0, 0, 0, 0)
	if got.RequestCount != 0 || got.TotalTokens != 0 || got.TotalCost != 0 {
		t.Fatalf("unexpected non-zero overview: %+v", got)
	}
	if got.ProviderCount != 0 || got.APIKeyCount != 0 || got.ModelCount != 0 {
		t.Fatalf("unexpected counts: %+v", got)
	}
}

func TestBuildAnalyticsOverview_WithData(t *testing.T) {
	metrics := model.StatsMetrics{
		RequestSuccess: 8,
		RequestFailed:  2,
		InputToken:     500,
		OutputToken:    300,
		InputCost:      0.10,
		OutputCost:     0.15,
	}
	got := buildAnalyticsOverview(metrics, 5, 10, 20, 25.0)
	if got.RequestCount != 10 {
		t.Fatalf("expected request count 10, got %d", got.RequestCount)
	}
	if got.TotalTokens != 800 {
		t.Fatalf("expected total tokens 800, got %d", got.TotalTokens)
	}
	if got.InputTokens != 500 || got.OutputTokens != 300 {
		t.Fatalf("unexpected token breakdown: %+v", got)
	}
	if got.TotalCost != 0.25 {
		t.Fatalf("expected total cost 0.25, got %f", got.TotalCost)
	}
	// 8/10 = 80% success
	if got.SuccessRate < 79.9 || got.SuccessRate > 80.1 {
		t.Fatalf("expected success rate ~80%%, got %f", got.SuccessRate)
	}
	if got.ProviderCount != 5 || got.APIKeyCount != 10 || got.ModelCount != 20 {
		t.Fatalf("unexpected counts: %+v", got)
	}
	if got.FallbackRate != 25.0 {
		t.Fatalf("expected fallback rate 25.0, got %f", got.FallbackRate)
	}
}

func TestBuildAnalyticsOverview_ZeroDivision(t *testing.T) {
	metrics := model.StatsMetrics{}
	got := buildAnalyticsOverview(metrics, 0, 0, 0, 0)
	if got.SuccessRate != 0 {
		t.Fatalf("expected success rate 0 when request count is 0, got %f", got.SuccessRate)
	}
}

// ── buildProviderBreakdown ──

func TestBuildProviderBreakdown_SortsByRequestsDesc(t *testing.T) {
	rows := map[int]*analyticsProviderAggregateRow{
		2: {
			ChannelID:   2,
			ChannelName: "beta",
			analyticsAggregateMetrics: analyticsAggregateMetrics{
				InputTokens:    40,
				OutputTokens:   60,
				TotalCost:      2,
				RequestSuccess: 2,
				RequestFailed:  1,
			},
		},
		1: {
			ChannelID:   1,
			ChannelName: "alpha",
			analyticsAggregateMetrics: analyticsAggregateMetrics{
				InputTokens:    10,
				OutputTokens:   20,
				TotalCost:      5,
				RequestSuccess: 1,
				RequestFailed:  0,
			},
		},
	}

	got := buildProviderBreakdown(rows, map[int]model.Channel{
		1: {ID: 1, Name: "alpha", Enabled: true},
		2: {ID: 2, Name: "beta", Enabled: false},
	})

	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
	if got[0].ChannelID != 2 {
		t.Fatalf("expected channel 2 first by request count, got %+v", got[0])
	}
	if got[0].RequestCount != 3 || got[0].TotalTokens != 100 {
		t.Fatalf("unexpected aggregate for first item: %+v", got[0])
	}
	if got[0].Enabled {
		t.Fatalf("expected channel 2 to be disabled: %+v", got[0])
	}
	if got[1].ChannelID != 1 || got[1].ChannelName != "alpha" {
		t.Fatalf("expected channel 1 second, got %+v", got[1])
	}
}

func TestBuildProviderBreakdown_PreservesHistoricalUsageFromStats(t *testing.T) {
	rows := map[int]*analyticsProviderAggregateRow{
		1: {
			ChannelID:   1,
			ChannelName: "alpha",
			analyticsAggregateMetrics: analyticsAggregateMetrics{
				InputTokens:    120,
				OutputTokens:   80,
				TotalCost:      3.5,
				RequestSuccess: 5,
				RequestFailed:  1,
			},
		},
	}

	got := buildProviderBreakdown(rows, map[int]model.Channel{
		1: {ID: 1, Name: "alpha", Enabled: true},
		2: {ID: 2, Name: "beta", Enabled: true},
	})

	if len(got) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got))
	}
	if got[0].ChannelID != 1 {
		t.Fatalf("expected channel 1, got %+v", got[0])
	}
	if got[0].RequestCount != 6 {
		t.Fatalf("expected historical request count to be preserved, got %+v", got[0])
	}
	if got[0].TotalTokens != 200 || got[0].TotalCost != 3.5 {
		t.Fatalf("expected historical token/cost totals to be preserved, got %+v", got[0])
	}
}

func TestBuildProviderBreakdown_UnknownChannel(t *testing.T) {
	rows := map[int]*analyticsProviderAggregateRow{
		99: {
			ChannelID:   99,
			ChannelName: "",
			analyticsAggregateMetrics: analyticsAggregateMetrics{
				RequestSuccess: 1,
			},
		},
	}

	got := buildProviderBreakdown(rows, map[int]model.Channel{})
	if len(got) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got))
	}
	if got[0].ChannelName != "Unknown Channel" {
		t.Fatalf("expected 'Unknown Channel' for unknown channel, got %q", got[0].ChannelName)
	}
	if got[0].Enabled {
		t.Fatalf("expected unknown channel to be disabled")
	}
}

// ── buildModelBreakdown ──

func TestBuildModelBreakdown_SortsByRequestsDesc(t *testing.T) {
	rows := map[string]*analyticsModelAggregateRow{
		"gpt-4": {
			ModelName: "gpt-4",
			analyticsAggregateMetrics: analyticsAggregateMetrics{
				RequestSuccess: 5,
				RequestFailed:  1,
				InputTokens:    100,
				OutputTokens:   50,
				TotalCost:      1.0,
			},
		},
		"gpt-3.5": {
			ModelName: "gpt-3.5",
			analyticsAggregateMetrics: analyticsAggregateMetrics{
				RequestSuccess: 2,
				RequestFailed:  0,
				InputTokens:    20,
				OutputTokens:   10,
				TotalCost:      0.1,
			},
		},
	}

	got := buildModelBreakdown(rows)
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
	if got[0].ModelName != "gpt-4" {
		t.Fatalf("expected gpt-4 first by request count, got %s", got[0].ModelName)
	}
	if got[0].RequestCount != 6 {
		t.Fatalf("expected request count 6 for gpt-4, got %d", got[0].RequestCount)
	}
}

func TestBuildModelBreakdown_SkipsEmptyName(t *testing.T) {
	rows := map[string]*analyticsModelAggregateRow{
		"": {
			ModelName: "",
			analyticsAggregateMetrics: analyticsAggregateMetrics{
				RequestSuccess: 1,
			},
		},
	}

	got := buildModelBreakdown(rows)
	if len(got) != 0 {
		t.Fatalf("expected empty result when model name is empty, got %d items", len(got))
	}
}

// ── buildAPIKeyBreakdown ──

func TestBuildAPIKeyBreakdown_KeepsDuplicateNamesSeparatedByID(t *testing.T) {
	rows := map[string]*analyticsAPIKeyAggregateRow{
		"id:11": {
			APIKeyID: 11,
			Name:     "shared",
			analyticsAggregateMetrics: analyticsAggregateMetrics{
				RequestSuccess: 3,
			},
		},
		"id:22": {
			APIKeyID: 22,
			Name:     "shared",
			analyticsAggregateMetrics: analyticsAggregateMetrics{
				RequestSuccess: 1,
				RequestFailed:  1,
			},
		},
	}

	got := buildAPIKeyBreakdown(rows)
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
	if got[0].APIKeyID == nil || got[1].APIKeyID == nil {
		t.Fatalf("expected API key ids to be present, got %+v", got)
	}
	if *got[0].APIKeyID == *got[1].APIKeyID {
		t.Fatalf("expected duplicate names to remain separate, got %+v", got)
	}
}

func TestBuildAPIKeyBreakdown_UnknownKey(t *testing.T) {
	rows := map[string]*analyticsAPIKeyAggregateRow{
		"name:guest": {
			APIKeyID: 0,
			Name:     "guest",
			analyticsAggregateMetrics: analyticsAggregateMetrics{
				RequestSuccess: 1,
			},
		},
	}

	got := buildAPIKeyBreakdown(rows)
	if len(got) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got))
	}
	if got[0].APIKeyID != nil {
		t.Fatalf("expected nil APIKeyID when api key id is 0, got %v", *got[0].APIKeyID)
	}
}

// ── loadAnalyticsProviderRows (with relaylog cache seeding) ──

func TestLoadAnalyticsProviderRows_UsesRangeBoundedRelayLogs(t *testing.T) {
	restoreLogs := relaylog.SetCacheForTest([]model.RelayLog{
		{
			Time:         time.Now().AddDate(0, 0, -2).Unix(),
			ChannelId:    1,
			ChannelName:  "alpha",
			InputTokens:  30,
			OutputTokens: 10,
			Cost:         0.5,
		},
		{
			Time:         time.Now().AddDate(0, 0, -40).Unix(),
			ChannelId:    1,
			ChannelName:  "alpha",
			InputTokens:  300,
			OutputTokens: 100,
			Cost:         5,
		},
	})
	defer restoreLogs()

	settingCache := setting.GetCache()
	oldSettings := settingCache.GetAll()
	settingCache.Set(model.SettingKeyRelayLogKeepEnabled, "false")
	defer func() {
		settingCache.Clear()
		for k, v := range oldSettings {
			settingCache.Set(k, v)
		}
	}()

	rows, err := loadAnalyticsProviderRows(context.Background(), model.AnalyticsRange30D)
	if err != nil {
		t.Fatalf("loadAnalyticsProviderRows() error = %v", err)
	}

	row := rows[1]
	if row == nil {
		t.Fatal("expected provider row for channel 1")
	}
	if row.RequestSuccess != 1 || row.RequestFailed != 0 {
		t.Fatalf("expected only in-range request totals, got %+v", row)
	}
	if row.InputTokens != 30 || row.OutputTokens != 10 {
		t.Fatalf("expected only in-range token totals, got %+v", row)
	}
	if row.TotalCost != 0.5 {
		t.Fatalf("expected only in-range cost total, got %+v", row)
	}
}

// ── mergeAnalyticsDailyWithToday ──

func TestMergeAnalyticsDailyWithToday_ReplacesExistingDate(t *testing.T) {
	daily := []model.StatsDaily{
		{Date: "20260101", StatsMetrics: model.StatsMetrics{RequestSuccess: 5}},
		{Date: "20260102", StatsMetrics: model.StatsMetrics{RequestSuccess: 3}},
	}
	today := model.StatsDaily{
		Date: "20260102",
		StatsMetrics: model.StatsMetrics{RequestSuccess: 10, RequestFailed: 1},
	}

	merged := mergeAnalyticsDailyWithToday(daily, today)
	if len(merged) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(merged))
	}
	found := false
	for _, item := range merged {
		if item.Date == "20260102" {
			found = true
			if item.RequestSuccess != 10 || item.RequestFailed != 1 {
				t.Fatalf("expected today's values to replace, got %+v", item)
			}
		}
	}
	if !found {
		t.Fatal("expected today's date in merged result")
	}
}

func TestMergeAnalyticsDailyWithToday_AppendsNewDate(t *testing.T) {
	daily := []model.StatsDaily{
		{Date: "20260101", StatsMetrics: model.StatsMetrics{RequestSuccess: 5}},
	}
	today := model.StatsDaily{
		Date: "20260103",
		StatsMetrics: model.StatsMetrics{RequestSuccess: 2},
	}

	merged := mergeAnalyticsDailyWithToday(daily, today)
	if len(merged) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(merged))
	}
}

func TestMergeAnalyticsDailyWithToday_EmptyToday(t *testing.T) {
	daily := []model.StatsDaily{
		{Date: "20260101", StatsMetrics: model.StatsMetrics{RequestSuccess: 5}},
	}
	today := model.StatsDaily{Date: ""}

	merged := mergeAnalyticsDailyWithToday(daily, today)
	if len(merged) != 1 {
		t.Fatalf("expected 1 entry when today is empty, got %d", len(merged))
	}
}

// ── analyticsStartTime / analyticsStartDate ──

func TestAnalyticsStartTime_Ranges(t *testing.T) {
	now := time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC)
	dayStart := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		r        model.AnalyticsRange
		expected *time.Time
	}{
		{"1D", model.AnalyticsRange1D, &dayStart},
		{"7D", model.AnalyticsRange7D, timePtr(dayStart.AddDate(0, 0, -6))},
		{"30D", model.AnalyticsRange30D, timePtr(dayStart.AddDate(0, 0, -29))},
		{"90D", model.AnalyticsRange90D, timePtr(dayStart.AddDate(0, 0, -89))},
		{"YTD", model.AnalyticsRangeYTD, timePtr(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))},
		{"All", model.AnalyticsRangeAll, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analyticsStartTime(tt.r, now)
			if tt.expected == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
			} else {
				if got == nil {
					t.Fatalf("expected %v, got nil", tt.expected)
				} else if !got.Equal(*tt.expected) {
					t.Fatalf("expected %v, got %v", tt.expected, got)
				}
			}
		})
	}
}

// ── splitAnalyticsChannelModels ──

func TestSplitAnalyticsChannelModels_Deduplicates(t *testing.T) {
	ch := model.Channel{
		Model:       "gpt-4,gpt-3.5,gpt-4",
		CustomModel: "gpt-4,claude-3",
	}

	models := splitAnalyticsChannelModels(ch)
	if len(models) != 3 {
		t.Fatalf("expected 3 unique models, got %d: %v", len(models), models)
	}
	seen := make(map[string]bool)
	for _, m := range models {
		if seen[m] {
			t.Fatalf("duplicate model %q", m)
		}
		seen[m] = true
	}
}

func TestSplitAnalyticsChannelModels_EmptyCustom(t *testing.T) {
	ch := model.Channel{
		Model:       "gpt-4,gpt-3.5",
		CustomModel: "",
	}

	models := splitAnalyticsChannelModels(ch)
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
}

// ── makeAnalyticsFailureKey ──

func TestMakeAnalyticsFailureKey_UsesActualModelWhenSet(t *testing.T) {
	key := makeAnalyticsFailureKey(1, "gpt-4", "gpt-4o")
	if key == "" {
		t.Fatal("expected non-empty key")
	}
}

func TestMakeAnalyticsFailureKey_FallsBackToRequestModel(t *testing.T) {
	key := makeAnalyticsFailureKey(1, "", "gpt-4o")
	if key == "" {
		t.Fatal("expected non-empty key when actual model is empty")
	}
}

func TestAnalyticsStartTime_UsesProvidedLocationBoundary(t *testing.T) {
	shanghai := time.FixedZone("UTC+8", 8*3600)
	now := time.Date(2026, 1, 15, 14, 30, 0, 0, shanghai)

	got := analyticsStartTime(model.AnalyticsRange1D, now)
	if got == nil {
		t.Fatal("expected non-nil start time")
	}

	expected := time.Date(2026, 1, 15, 0, 0, 0, 0, shanghai)
	if !got.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
	if got.Location() != shanghai {
		t.Fatalf("expected location %v, got %v", shanghai, got.Location())
	}
}
func timePtr(t time.Time) *time.Time { return &t }

