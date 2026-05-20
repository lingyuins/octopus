package op

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/semantic_cache"
)

func TestNormalizeNavOrder_AppendsMissingRoutesAndDropsUnknown(t *testing.T) {
	defaults := []string{"home", "channel", "group", "model", "analytics", "log", "alert", "ops", "apikey", "setting", "user"}
	got := NormalizeNavOrder(`["group","group","unknown","setting"]`, defaults)
	want := []string{"group", "setting", "home", "channel", "model", "analytics", "log", "alert", "ops", "apikey", "user"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeNavOrder() = %v, want %v", got, want)
	}
}

func TestBuildSemanticCacheEvaluationSummary_ComputesRates(t *testing.T) {
	stats := semantic_cache.RuntimeStats{
		EvaluatedRequests: 12,
		CacheHitResponses: 8,
		CacheMissRequests: 3,
		BypassedRequests:  1,
		StoredResponses:   3,
	}
	got := buildSemanticCacheEvaluationSummary(
		true, true, 3600, 98, 1000, 120, 80, 40, stats,
	)
	if got.HitRate != 66.66666666666666 {
		t.Fatalf("HitRate = %v", got.HitRate)
	}
	if got.UsageRate != 12 {
		t.Fatalf("UsageRate = %v", got.UsageRate)
	}
}

func TestRefreshSemanticCacheRuntime_ResetsDisabledOrIncompleteConfig(t *testing.T) {
	restore := snapshotSettingCache()
	defer restore()
	semantic_cache.Reset()
	semantic_cache.ResetRuntimeStats()
	defer semantic_cache.Reset()
	defer semantic_cache.ResetRuntimeStats()

	semantic_cache.ApplyRuntimeConfig(semantic_cache.RuntimeConfig{
		Enabled:          true,
		MaxEntries:       8,
		Threshold:        0.98,
		TTL:              time.Hour,
		EmbeddingBaseURL: "https://example.com",
		EmbeddingModel:   "text-embedding-3-small",
	})
	if !semantic_cache.Enabled() {
		t.Fatal("expected seeded runtime cache to be enabled")
	}

	seedDefaultSettingsForTest(map[model.SettingKey]string{
		model.SettingKeySemanticCacheEnabled: "false",
	})
	if err := RefreshSemanticCacheRuntime(); err != nil {
		t.Fatalf("RefreshSemanticCacheRuntime() disabled config error = %v", err)
	}
	if semantic_cache.RuntimeEnabled() {
		t.Fatal("expected disabled setting to clear semantic cache runtime")
	}

	semantic_cache.ApplyRuntimeConfig(semantic_cache.RuntimeConfig{
		Enabled:          true,
		MaxEntries:       8,
		Threshold:        0.98,
		TTL:              time.Hour,
		EmbeddingBaseURL: "https://example.com",
		EmbeddingModel:   "text-embedding-3-small",
	})
	seedDefaultSettingsForTest(map[model.SettingKey]string{
		model.SettingKeySemanticCacheEnabled:          "true",
		model.SettingKeySemanticCacheEmbeddingBaseURL: "",
		model.SettingKeySemanticCacheEmbeddingModel:   "text-embedding-3-small",
	})
	if err := RefreshSemanticCacheRuntime(); err != nil {
		t.Fatalf("RefreshSemanticCacheRuntime() incomplete config error = %v", err)
	}
	if semantic_cache.RuntimeEnabled() {
		t.Fatal("expected incomplete config to clear semantic cache runtime")
	}
}

func TestAnalyticsEvaluationGet_ReturnsSemanticCacheSummary(t *testing.T) {
	restore := snapshotSettingCache()
	defer restore()
	semantic_cache.Reset()
	semantic_cache.ResetRuntimeStats()
	defer semantic_cache.Reset()
	defer semantic_cache.ResetRuntimeStats()

	seedDefaultSettingsForTest(map[model.SettingKey]string{
		model.SettingKeySemanticCacheEnabled:                 "true",
		model.SettingKeySemanticCacheTTL:                     "7200",
		model.SettingKeySemanticCacheThreshold:               "97",
		model.SettingKeySemanticCacheMaxEntries:              "50",
		model.SettingKeySemanticCacheEmbeddingBaseURL:        "https://example.com/v1",
		model.SettingKeySemanticCacheEmbeddingAPIKey:         "test-key",
		model.SettingKeySemanticCacheEmbeddingModel:          "text-embedding-3-small",
		model.SettingKeySemanticCacheEmbeddingTimeoutSeconds: "12",
	})

	if err := RefreshSemanticCacheRuntime(); err != nil {
		t.Fatalf("RefreshSemanticCacheRuntime() error = %v", err)
	}

	embedding := []float64{1, 0}
	semantic_cache.Store("ns", "req-1", []byte(`{"ok":true}`), embedding)
	if _, ok := semantic_cache.Lookup("ns", embedding); !ok {
		t.Fatal("expected cached lookup hit")
	}
	if _, ok := semantic_cache.Lookup("ns", []float64{0, 1}); ok {
		t.Fatal("expected cache miss for different embedding")
	}

	for i := 0; i < 4; i++ {
		semantic_cache.RecordEvaluated()
	}
	semantic_cache.RecordHit()
	semantic_cache.RecordMiss()
	semantic_cache.RecordMiss()
	semantic_cache.RecordBypass()
	semantic_cache.RecordStored()
	semantic_cache.RecordStored()

	got, err := AnalyticsEvaluationGet(context.Background())
	if err != nil {
		t.Fatalf("AnalyticsEvaluationGet() error = %v", err)
	}

	summary := got.SemanticCache
	if !summary.Enabled || !summary.RuntimeEnabled {
		t.Fatalf("expected semantic cache to be enabled in summary: %+v", summary)
	}
	if summary.TTLSeconds != 7200 || summary.Threshold != 97 || summary.MaxEntries != 50 {
		t.Fatalf("unexpected config summary: %+v", summary)
	}
	if summary.CurrentEntries != 1 || summary.Hits != 1 || summary.Misses != 1 {
		t.Fatalf("unexpected cache stats summary: %+v", summary)
	}
	if summary.HitRate != 50 || summary.UsageRate != 2 {
		t.Fatalf("unexpected rates summary: %+v", summary)
	}
	if summary.EvaluatedRequests != 4 || summary.CacheHitResponses != 1 || summary.CacheMissRequests != 2 || summary.BypassedRequests != 1 || summary.StoredResponses != 2 {
		t.Fatalf("unexpected runtime stats summary: %+v", summary)
	}
}

func TestTelemetrySummaryGet_ReturnsValidData(t *testing.T) {
	ctx := context.Background()
	summary, err := TelemetrySummaryGet(ctx)
	if err != nil {
		t.Fatalf("TelemetrySummaryGet returned error: %v", err)
	}
	if summary == nil {
		t.Fatal("TelemetrySummaryGet returned nil summary")
	}
	if summary.Hero.UptimeSeconds < 0 {
		t.Error("uptime_seconds should be >= 0")
	}
	if summary.Hero.ActiveConnections < 0 {
		t.Error("active_connections should be >= 0")
	}
	if len(summary.DrilldownShortcuts) != 5 {
		t.Errorf("expected 5 drilldown shortcuts, got %d", len(summary.DrilldownShortcuts))
	}
}

func TestTelemetrySummaryGet_DrilldownKeys(t *testing.T) {
	ctx := context.Background()
	summary, err := TelemetrySummaryGet(ctx)
	if err != nil {
		t.Fatalf("TelemetrySummaryGet returned error: %v", err)
	}
	expectedKeys := []string{"cache", "quota", "health", "system", "audit"}
	for i, sc := range summary.DrilldownShortcuts {
		if sc.Key != expectedKeys[i] {
			t.Errorf("shortcut[%d]: expected key %q, got %q", i, expectedKeys[i], sc.Key)
		}
	}
}

func TestTelemetrySummaryGet_DatabaseHealthDefaults(t *testing.T) {
	ctx := context.Background()
	summary, err := TelemetrySummaryGet(ctx)
	if err != nil {
		t.Fatalf("TelemetrySummaryGet returned error: %v", err)
	}
	if summary.DatabaseHealth.Repairs != 0 {
		t.Error("database_health.repairs should be 0 in phase 1")
	}
	validStatuses := map[string]bool{"healthy": true, "degraded": true}
	if !validStatuses[summary.DatabaseHealth.Status] {
		t.Errorf("unexpected database_health.status: %q", summary.DatabaseHealth.Status)
	}
}

func TestTelemetrySummaryGet_ProviderHealthDefaults(t *testing.T) {
	ctx := context.Background()
	summary, err := TelemetrySummaryGet(ctx)
	if err != nil {
		t.Fatalf("TelemetrySummaryGet returned error: %v", err)
	}
	if summary.ProviderHealth.Monitored != len(summary.ProviderHealth.Providers) {
		t.Errorf("monitored=%d but providers len=%d", summary.ProviderHealth.Monitored, len(summary.ProviderHealth.Providers))
	}
	if summary.ProviderHealth.Active > summary.ProviderHealth.Monitored {
		t.Error("active should be <= monitored")
	}
}

func snapshotSettingCache() func() {
	snapshot := settingCache.GetAll()
	return func() {
		settingCache.Clear()
		for key, value := range snapshot {
			settingCache.Set(key, value)
		}
	}
}

func seedDefaultSettingsForTest(overrides map[model.SettingKey]string) {
	settingCache.Clear()
	for _, setting := range model.DefaultSettings() {
		value := setting.Value
		if override, ok := overrides[setting.Key]; ok {
			value = override
		}
		settingCache.Set(setting.Key, value)
	}
	for key, value := range overrides {
		if _, exists := settingCache.Get(key); exists {
			continue
		}
		settingCache.Set(key, value)
	}
}
