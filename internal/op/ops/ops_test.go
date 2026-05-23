package ops

import (
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/llm"
	"github.com/lingyuins/octopus/internal/utils/telemetry"
)

func TestBuildOpsCacheStatus_ComputesRates(t *testing.T) {
	got := buildOpsCacheStatus(true, true, 3600, 98, 100, 3, 1, 25)

	if !got.Enabled || !got.RuntimeEnabled {
		t.Fatalf("expected cache to be enabled at config and runtime levels: %+v", got)
	}
	if got.HitRate != 75 {
		t.Fatalf("hit rate = %v, want 75", got.HitRate)
	}
	if got.UsageRate != 25 {
		t.Fatalf("usage rate = %v, want 25", got.UsageRate)
	}
}

func TestBuildOpsQuotaSummary_ClassifiesAndSortsKeys(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	keys := []model.APIKey{
		{
			ID:                1,
			Name:              "Budget key",
			Enabled:           true,
			MaxCost:           5,
			RateLimitRPM:      100,
			PerModelQuotaJSON: `{"gpt-4.1":1000}`,
		},
		{
			ID:           2,
			Name:         "Expired key",
			Enabled:      true,
			ExpireAt:     now.Unix() - 60,
			MaxCost:      10,
			RateLimitTPM: 2000,
		},
		{
			ID:      3,
			Name:    "Open key",
			Enabled: false,
		},
	}
	stats := []model.StatsAPIKey{
		{
			APIKeyID: 1,
			StatsMetrics: model.StatsMetrics{
				InputCost:      2,
				OutputCost:     4,
				RequestSuccess: 2,
			},
		},
		{
			APIKeyID: 2,
			StatsMetrics: model.StatsMetrics{
				InputCost:     1,
				OutputCost:    1,
				RequestFailed: 1,
			},
		},
	}

	got := buildOpsQuotaSummary(keys, stats, now)

	if got.TotalKeyCount != 3 || got.EnabledKeyCount != 2 {
		t.Fatalf("unexpected quota summary counts: %+v", got)
	}
	if got.ExhaustedKeyCount != 1 || got.ExpiredKeyCount != 1 {
		t.Fatalf("unexpected exhausted/expired counts: %+v", got)
	}
	if got.PerModelQuotaKeyCount != 1 || got.ActiveUsageKeyCount != 2 {
		t.Fatalf("unexpected quota flags: %+v", got)
	}
	if len(got.Keys) != 3 {
		t.Fatalf("keys length = %d, want 3", len(got.Keys))
	}
	if got.Keys[0].Status != "exhausted" || got.Keys[0].APIKeyID != 1 {
		t.Fatalf("expected exhausted key first, got %+v", got.Keys[0])
	}
	if got.Keys[1].Status != "expired" || got.Keys[1].APIKeyID != 2 {
		t.Fatalf("expected expired key second, got %+v", got.Keys[1])
	}
	if got.Keys[2].Status != "disabled" || got.Keys[2].APIKeyID != 3 {
		t.Fatalf("expected disabled key last, got %+v", got.Keys[2])
	}
}

func TestBuildOpsHealthStatus_CountsAndLimitsFailingGroups(t *testing.T) {
	groupHealth := []model.AnalyticsGroupHealthItem{
		{GroupID: 1, GroupName: "g1", Status: "down", FailureCount: 5, HealthScore: 10},
		{GroupID: 2, GroupName: "g2", Status: "degraded", FailureCount: 3, HealthScore: 40},
		{GroupID: 3, GroupName: "g3", Status: "warning", FailureCount: 1, HealthScore: 70},
		{GroupID: 4, GroupName: "g4", Status: "empty", FailureCount: 0, HealthScore: 0},
		{GroupID: 5, GroupName: "g5", Status: "healthy", FailureCount: 0, HealthScore: 100},
		{GroupID: 6, GroupName: "g6", Status: "down", FailureCount: 8, HealthScore: 5},
		{GroupID: 7, GroupName: "g7", Status: "warning", FailureCount: 2, HealthScore: 65},
		{GroupID: 8, GroupName: "g8", Status: "degraded", FailureCount: 4, HealthScore: 30},
	}

	got := buildOpsHealthStatus(true, true, true, 9, groupHealth, time.Unix(1_700_000_000, 0))

	if got.RecentErrorCount != 9 || !got.DatabaseOK || !got.CacheOK || !got.TaskRuntimeOK {
		t.Fatalf("unexpected base health status: %+v", got)
	}
	if got.HealthyGroupCount != 1 || got.WarningGroupCount != 2 || got.DegradedGroupCount != 2 || got.DownGroupCount != 2 || got.EmptyGroupCount != 1 {
		t.Fatalf("unexpected group counts: %+v", got)
	}
	if len(got.FailingGroups) != opsFailingGroupLimit {
		t.Fatalf("failing group length = %d, want %d", len(got.FailingGroups), opsFailingGroupLimit)
	}
	if got.FailingGroups[0].GroupID != 1 || got.FailingGroups[1].GroupID != 2 {
		t.Fatalf("expected failing groups to preserve worst-first order, got %+v", got.FailingGroups)
	}
}

func TestBuildOpsAIRouteServices_ReturnsEmptySliceForNilConfigs(t *testing.T) {
	got := buildOpsAIRouteServices(nil)

	if got == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected no services, got %d", len(got))
	}
}

func TestParseOpsProviderPromptCacheUsage_OpenAIStyle(t *testing.T) {
	usage, ok := parseOpsProviderPromptCacheUsage(`{"usage":{"input_tokens":1000,"input_tokens_details":{"cached_tokens":250},"output_tokens":10}}`)
	if !ok {
		t.Fatal("expected provider prompt cache usage to be parsed")
	}
	if usage.PromptTokens != 1000 {
		t.Fatalf("PromptTokens = %d, want 1000", usage.PromptTokens)
	}
	if usage.CachedTokens != 250 {
		t.Fatalf("CachedTokens = %d, want 250", usage.CachedTokens)
	}
	if usage.CacheCreationInputTokens != 0 {
		t.Fatalf("CacheCreationInputTokens = %d, want 0", usage.CacheCreationInputTokens)
	}
	if usage.TotalInputTokens != 1000 {
		t.Fatalf("TotalInputTokens = %d, want 1000", usage.TotalInputTokens)
	}
}

func TestParseOpsProviderPromptCacheUsage_PromptTokensDetailsStyle(t *testing.T) {
	usage, ok := parseOpsProviderPromptCacheUsage(`{"usage":{"prompt_tokens":254,"prompt_tokens_details":{"cached_tokens":192},"output_tokens":161}}`)
	if !ok {
		t.Fatal("expected provider prompt cache usage to be parsed")
	}
	if usage.PromptTokens != 254 {
		t.Fatalf("PromptTokens = %d, want 254", usage.PromptTokens)
	}
	if usage.CachedTokens != 192 {
		t.Fatalf("CachedTokens = %d, want 192", usage.CachedTokens)
	}
	if usage.CacheCreationInputTokens != 0 {
		t.Fatalf("CacheCreationInputTokens = %d, want 0", usage.CacheCreationInputTokens)
	}
	if usage.TotalInputTokens != 254 {
		t.Fatalf("TotalInputTokens = %d, want 254", usage.TotalInputTokens)
	}
}

func TestParseOpsProviderPromptCacheUsage_AnthropicStyle(t *testing.T) {
	usage, ok := parseOpsProviderPromptCacheUsage(`{"usage":{"input_tokens":600,"input_tokens_details":{"cached_tokens":300},"cache_creation_input_tokens":120}}`)
	if !ok {
		t.Fatal("expected provider prompt cache usage to be parsed")
	}
	if usage.PromptTokens != 600 {
		t.Fatalf("PromptTokens = %d, want 600", usage.PromptTokens)
	}
	if usage.CachedTokens != 300 {
		t.Fatalf("CachedTokens = %d, want 300", usage.CachedTokens)
	}
	if usage.CacheCreationInputTokens != 120 {
		t.Fatalf("CacheCreationInputTokens = %d, want 120", usage.CacheCreationInputTokens)
	}
	if usage.TotalInputTokens != 1020 {
		t.Fatalf("TotalInputTokens = %d, want 1020", usage.TotalInputTokens)
	}
}

func TestParseOpsProviderPromptCacheUsage_PromptCacheHitTokensFallback(t *testing.T) {
	usage, ok := parseOpsProviderPromptCacheUsage(`{"usage":{"prompt_tokens":512,"prompt_cache_hit_tokens":128,"output_tokens":64}}`)
	if !ok {
		t.Fatal("expected provider prompt cache usage to be parsed")
	}
	if usage.PromptTokens != 512 {
		t.Fatalf("PromptTokens = %d, want 512", usage.PromptTokens)
	}
	if usage.CachedTokens != 128 {
		t.Fatalf("CachedTokens = %d, want 128", usage.CachedTokens)
	}
	if usage.TotalInputTokens != 512 {
		t.Fatalf("TotalInputTokens = %d, want 512", usage.TotalInputTokens)
	}
}

func TestParseOpsProviderPromptCacheUsage_TopLevelCachedTokensFallback(t *testing.T) {
	usage, ok := parseOpsProviderPromptCacheUsage(`{"usage":{"input_tokens":900,"cached_tokens":180,"output_tokens":32}}`)
	if !ok {
		t.Fatal("expected provider prompt cache usage to be parsed")
	}
	if usage.PromptTokens != 900 {
		t.Fatalf("PromptTokens = %d, want 900", usage.PromptTokens)
	}
	if usage.CachedTokens != 180 {
		t.Fatalf("CachedTokens = %d, want 180", usage.CachedTokens)
	}
	if usage.TotalInputTokens != 900 {
		t.Fatalf("TotalInputTokens = %d, want 900", usage.TotalInputTokens)
	}
}

func TestParseOpsProviderPromptCacheUsage_InputTokenDetailsAlias(t *testing.T) {
	usage, ok := parseOpsProviderPromptCacheUsage(`{"usage":{"input_tokens":1000,"input_token_details":{"cached_tokens":320},"output_tokens":10}}`)
	if !ok {
		t.Fatal("expected provider prompt cache usage to be parsed")
	}
	if usage.CachedTokens != 320 {
		t.Fatalf("CachedTokens = %d, want 320", usage.CachedTokens)
	}
	if usage.TotalInputTokens != 1000 {
		t.Fatalf("TotalInputTokens = %d, want 1000", usage.TotalInputTokens)
	}
}

func TestBuildOpsProviderPromptCacheSummaryFromLogs_AggregatesByChannelAndTrend(t *testing.T) {
	llmCache := llm.GetCache()
	oldLLMs := llmCache.GetAll()
	llmCache.Clear()
	llmCache.Set("claude-3-5-sonnet-20241022", model.LLMPrice{
		Input:      3,
		Output:     15,
		CacheRead:  0.3,
		CacheWrite: 3.75,
	})
	llmCache.Set("gpt-4o", model.LLMPrice{
		Input:      2.5,
		Output:     10,
		CacheRead:  1.25,
		CacheWrite: 0,
	})
	defer func() {
		llmCache.Clear()
		for k, v := range oldLLMs {
			llmCache.Set(k, v)
		}
	}()

	start := time.Unix(1_700_000_000, 0).UTC().Truncate(time.Hour)
	logs := []model.RelayLog{
		{
			Time:            start.Add(1 * time.Hour).Unix(),
			ChannelId:       1,
			ChannelName:     "anthropic",
			ActualModelName: "claude-3-5-sonnet-20241022",
			ResponseContent: `{"usage":{"input_tokens":600,"input_tokens_details":{"cached_tokens":300},"cache_creation_input_tokens":120}}`,
		},
		{
			Time:            start.Add(1 * time.Hour).Unix(),
			ChannelId:       1,
			ChannelName:     "anthropic",
			ActualModelName: "claude-3-5-sonnet-20241022",
			ResponseContent: `{"usage":{"input_tokens":400,"output_tokens":20}}`,
		},
		{
			Time:            start.Add(3 * time.Hour).Unix(),
			ChannelId:       2,
			ChannelName:     "openai",
			ActualModelName: "gpt-4o",
			ResponseContent: `{"usage":{"input_tokens":1000,"input_tokens_details":{"cached_tokens":250},"output_tokens":10}}`,
		},
	}

	summary := buildOpsProviderPromptCacheSummaryFromLogs(logs, start)

	if summary.RequestCount != 3 {
		t.Fatalf("RequestCount = %d, want 3", summary.RequestCount)
	}
	if summary.CachedRequestCount != 2 {
		t.Fatalf("CachedRequestCount = %d, want 2", summary.CachedRequestCount)
	}
	if summary.CacheReadTokens != 550 {
		t.Fatalf("CacheReadTokens = %d, want 550", summary.CacheReadTokens)
	}
	if summary.CacheWriteTokens != 120 {
		t.Fatalf("CacheWriteTokens = %d, want 120", summary.CacheWriteTokens)
	}
	if len(summary.Providers) != 2 {
		t.Fatalf("Providers len = %d, want 2", len(summary.Providers))
	}
	if summary.Providers[0].ChannelName != "openai" && summary.Providers[0].ChannelName != "anthropic" {
		t.Fatalf("unexpected provider ordering: %+v", summary.Providers)
	}
	if summary.Trend[1].RequestCount != 2 {
		t.Fatalf("trend[1].RequestCount = %d, want 2", summary.Trend[1].RequestCount)
	}
	if summary.Trend[1].CachedRequestCount != 1 {
		t.Fatalf("trend[1].CachedRequestCount = %d, want 1", summary.Trend[1].CachedRequestCount)
	}
	if summary.Trend[3].CacheReadTokens != 250 {
		t.Fatalf("trend[3].CacheReadTokens = %d, want 250", summary.Trend[3].CacheReadTokens)
	}
}

func TestBuildOpsProviderPromptCacheSummaryFromLogs_UsesTimezoneAlignedBuckets(t *testing.T) {
	now := time.Date(2026, 5, 24, 10, 15, 0, 0, time.UTC)
	start := opsHourlyWindowStart(now, 8)
	wantStart := time.Date(2026, 5, 23, 11, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Fatalf("start = %s, want %s", start.Format(time.RFC3339), wantStart.Format(time.RFC3339))
	}

	logTime := time.Date(2026, 5, 24, 2, 30, 0, 0, time.UTC)
	summary := buildOpsProviderPromptCacheSummaryFromLogs([]model.RelayLog{{
		Time:            logTime.Unix(),
		ChannelId:       1,
		ChannelName:     "openai",
		ActualModelName: "gpt-4o",
		ResponseContent: `{"usage":{"input_tokens":1000,"input_token_details":{"cached_tokens":250}}}`,
	}}, start)

	if summary.RequestCount != 1 || summary.CacheReadTokens != 250 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if summary.Trend[15].RequestCount != 1 {
		t.Fatalf("trend[15].RequestCount = %d, want 1", summary.Trend[15].RequestCount)
	}
	if summary.Trend[15].Timestamp != time.Date(2026, 5, 24, 2, 0, 0, 0, time.UTC).Unix() {
		t.Fatalf("trend[15].Timestamp = %d, want 2026-05-24T02:00:00Z", summary.Trend[15].Timestamp)
	}
}

func TestBuildOpsTelemetryHeroMetrics_FallsBackToPersistedStats(t *testing.T) {
	snap := telemetry.NewStore().Snapshot()
	total := model.StatsTotal{StatsMetrics: model.StatsMetrics{
		WaitTime:       1200,
		RequestSuccess: 2,
		RequestFailed:  1,
	}}

	got := buildOpsTelemetryHeroMetrics(snap, total, 99)

	if got.TotalRequests != 3 {
		t.Fatalf("TotalRequests = %d, want 3", got.TotalRequests)
	}
	if got.AvgLatencyMs != 400 {
		t.Fatalf("AvgLatencyMs = %v, want 400", got.AvgLatencyMs)
	}
	if got.ErrorRate != float64(1)/float64(3)*100 {
		t.Fatalf("ErrorRate = %v, want %v", got.ErrorRate, float64(1)/float64(3)*100)
	}
}

func TestBuildOpsTelemetryRuntimeSignals_FallsBackToRecentLogs(t *testing.T) {
	snap := telemetry.NewStore().Snapshot()
	now := time.Date(2026, 5, 24, 10, 10, 0, 0, time.UTC)
	logs := []model.RelayLog{
		{Time: now.Add(-50 * time.Second).Unix(), UseTime: 100},
		{Time: now.Add(-40 * time.Second).Unix(), UseTime: 200, Error: "upstream error"},
		{Time: now.Add(-30 * time.Second).Unix(), UseTime: 900},
	}

	got := buildOpsTelemetryRuntimeSignals(snap, logs, now)

	if got.P95LatencyMs != 900 {
		t.Fatalf("P95LatencyMs = %v, want 900", got.P95LatencyMs)
	}
	if got.ThroughputRPS != float64(3)/60 {
		t.Fatalf("ThroughputRPS = %v, want %v", got.ThroughputRPS, float64(3)/60)
	}
	var latestWithRequests model.OpsTelemetryTrendPoint
	for _, point := range got.TrendSnapshots {
		if point.RequestDelta > 0 {
			latestWithRequests = point
		}
	}
	if latestWithRequests.RequestDelta != 3 || latestWithRequests.FailedDelta != 1 {
		t.Fatalf("unexpected trend point with requests: %+v", latestWithRequests)
	}
}
