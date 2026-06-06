package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/airoute"
	"github.com/lingyuins/octopus/internal/op/analytics"
	"github.com/lingyuins/octopus/internal/op/apikey"
	"github.com/lingyuins/octopus/internal/op/cacheusage"
	"github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/op/group"
	"github.com/lingyuins/octopus/internal/op/llm"
	"github.com/lingyuins/octopus/internal/op/relaylog"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/op/stats"
	"github.com/lingyuins/octopus/internal/utils/semantic_cache"
	"github.com/lingyuins/octopus/internal/utils/telemetry"
)

const (
	opsHealthErrorWindow           = 24 * time.Hour
	opsFailingGroupLimit           = 6
	semanticCacheDefaultTTLSeconds = 3600
	semanticCacheDefaultThreshold  = 98
	semanticCacheDefaultMaxEntries = 1000
	semanticCacheDefaultTimeoutSec = 10
)

var processStartTime = time.Now()

type opsQuotaUsage struct {
	RequestCount int64
	TotalCost    float64
}

type opsProviderPromptCacheUsage struct {
	PromptTokens             int64
	TotalInputTokens         int64
	CachedTokens             int64
	CacheCreationInputTokens int64
}

type opsProviderPromptCacheAggregate struct {
	ChannelName        string
	RequestCount       int64
	CachedRequestCount int64
	TotalInputTokens   int64
	CacheReadTokens    int64
	CacheWriteTokens   int64
	EstimatedCostSaved float64
}

func OpsCacheStatusGet(ctx context.Context) (*model.OpsCacheStatus, error) {
	enabled, err := setting.GetBool(model.SettingKeySemanticCacheEnabled)
	if err != nil {
		return nil, err
	}
	ttlSeconds, err := setting.GetInt(model.SettingKeySemanticCacheTTL)
	if err != nil {
		return nil, err
	}
	threshold, err := setting.GetInt(model.SettingKeySemanticCacheThreshold)
	if err != nil {
		return nil, err
	}
	maxEntries, err := setting.GetInt(model.SettingKeySemanticCacheMaxEntries)
	if err != nil {
		return nil, err
	}

	hits, misses, size := semantic_cache.Stats()
	status := buildOpsCacheStatus(enabled, semantic_cache.RuntimeEnabled(), ttlSeconds, threshold, maxEntries, hits, misses, size)
	status.ProviderPromptCache = buildOpsProviderPromptCacheSummary(ctx)
	return &status, nil
}

func RefreshSemanticCacheRuntime() error {
	cfg, ok, err := buildSemanticCacheRuntimeConfigFromSettings()
	if err != nil {
		return err
	}
	if !ok {
		semantic_cache.Reset()
		return nil
	}
	semantic_cache.ApplyRuntimeConfig(cfg)
	return nil
}

func OpsQuotaSummaryGet(ctx context.Context) (*model.OpsQuotaSummary, error) {
	apiKeys, err := apikey.List(ctx)
	if err != nil {
		return nil, err
	}

	summary := buildOpsQuotaSummary(apiKeys, stats.APIKeyList(), time.Now())
	return &summary, nil
}

func OpsHealthStatusGet(ctx context.Context) (*model.OpsHealthStatus, error) {
	cacheStatus, err := OpsCacheStatusGet(ctx)
	if err != nil {
		return nil, err
	}

	recentErrorCount, err := loadOpsRecentErrorCount(ctx, time.Now().Add(-opsHealthErrorWindow))
	if err != nil {
		return nil, err
	}

	groupHealth, err := analytics.AnalyticsGroupHealthGet(ctx)
	if err != nil {
		return nil, err
	}

	status := buildOpsHealthStatus(
		pingDatabase(ctx),
		!cacheStatus.Enabled || cacheStatus.RuntimeEnabled,
		opsTaskRuntimeOK(),
		recentErrorCount,
		groupHealth,
		time.Now(),
	)
	return &status, nil
}

func OpsSystemSummaryGet(ctx context.Context) (*model.OpsSystemSummary, error) {
	proxyURL, err := setting.GetString(model.SettingKeyProxyURL)
	if err != nil {
		return nil, err
	}
	publicAPIBaseURL, err := setting.GetString(model.SettingKeyPublicAPIBaseURL)
	if err != nil {
		return nil, err
	}
	relayLogKeepEnabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return nil, err
	}
	relayLogKeepDays, err := setting.GetInt(model.SettingKeyRelayLogKeepPeriod)
	if err != nil {
		return nil, err
	}
	relayLogKeepCount, err := setting.GetInt(model.SettingKeyRelayLogKeepCount)
	if err != nil {
		return nil, err
	}
	statsSaveIntervalMinutes, err := setting.GetInt(model.SettingKeyStatsSaveInterval)
	if err != nil {
		return nil, err
	}
	syncLLMIntervalHours, err := setting.GetInt(model.SettingKeySyncLLMInterval)
	if err != nil {
		return nil, err
	}
	modelInfoUpdateIntervalHours, err := setting.GetInt(model.SettingKeyModelInfoUpdateInterval)
	if err != nil {
		return nil, err
	}
	aiRouteGroupID, err := setting.GetInt(model.SettingKeyAIRouteGroupID)
	if err != nil {
		return nil, err
	}
	aiRouteTimeoutSeconds, err := setting.GetInt(model.SettingKeyAIRouteTimeoutSeconds)
	if err != nil {
		return nil, err
	}
	aiRouteParallelism, err := setting.GetInt(model.SettingKeyAIRouteParallelism)
	if err != nil {
		return nil, err
	}

	channels, err := channel.List(ctx)
	if err != nil {
		return nil, err
	}
	groups, err := group.GroupList(ctx)
	if err != nil {
		return nil, err
	}
	apiKeys, err := apikey.List(ctx)
	if err != nil {
		return nil, err
	}

	aiRouteServices, legacyMode := loadOpsAIRouteServicesSummary()
	enabledServiceCount := 0
	for _, service := range aiRouteServices {
		if service.Enabled {
			enabledServiceCount++
		}
	}

	summary := &model.OpsSystemSummary{
		Version:                      conf.Version,
		Commit:                       conf.Commit,
		BuildTime:                    conf.BuildTime,
		Repo:                         conf.Repo,
		DatabaseType:                 conf.AppConfig.Database.Type,
		PublicAPIBaseURL:             strings.TrimSpace(publicAPIBaseURL),
		ProxyURL:                     strings.TrimSpace(proxyURL),
		RelayLogKeepEnabled:          relayLogKeepEnabled,
		RelayLogKeepDays:             relayLogKeepDays,
		RelayLogKeepCount:            relayLogKeepCount,
		StatsSaveIntervalMinutes:     statsSaveIntervalMinutes,
		SyncLLMIntervalHours:         syncLLMIntervalHours,
		ModelInfoUpdateIntervalHours: modelInfoUpdateIntervalHours,
		ImportEnabled:                true,
		ExportEnabled:                true,
		AIRouteGroupID:               aiRouteGroupID,
		AIRouteTimeoutSeconds:        aiRouteTimeoutSeconds,
		AIRouteParallelism:           aiRouteParallelism,
		AIRouteLegacyMode:            legacyMode,
		AIRouteServiceCount:          len(aiRouteServices),
		AIRouteEnabledServiceCount:   enabledServiceCount,
		AIRouteServices:              aiRouteServices,
		ChannelCount:                 len(channels),
		GroupCount:                   len(groups),
		APIKeyCount:                  len(apiKeys),
	}
	return summary, nil
}

func buildOpsCacheStatus(
	enabled bool,
	runtimeEnabled bool,
	ttlSeconds int,
	threshold int,
	maxEntries int,
	hits int64,
	misses int64,
	size int,
) model.OpsCacheStatus {
	totalLookups := hits + misses
	hitRate := 0.0
	if totalLookups > 0 {
		hitRate = (float64(hits) / float64(totalLookups)) * 100
	}

	usageRate := 0.0
	if maxEntries > 0 {
		usageRate = (float64(size) / float64(maxEntries)) * 100
	}

	return model.OpsCacheStatus{
		Enabled:        enabled,
		RuntimeEnabled: runtimeEnabled,
		TTLSeconds:     ttlSeconds,
		Threshold:      threshold,
		MaxEntries:     maxEntries,
		CurrentEntries: size,
		Hits:           hits,
		Misses:         misses,
		HitRate:        hitRate,
		UsageRate:      usageRate,
	}
}

func buildOpsProviderPromptCacheSummary(ctx context.Context) model.OpsProviderPromptCacheSummary {
	start := opsHourlyWindowStart(time.Now(), configuredStatsTimezoneOffsetHours())
	logs := loadOpsProviderPromptCacheLogs(ctx, start)
	return buildOpsProviderPromptCacheSummaryFromLogs(logs, start)
}

func configuredStatsTimezoneOffsetHours() int {
	offset, err := setting.GetInt(model.SettingKeyStatsTimezoneOffset)
	if err != nil || offset < -12 || offset > 14 {
		return 0
	}
	return offset
}

func opsHourlyWindowStart(now time.Time, offsetHours int) time.Time {
	offset := time.Duration(offsetHours) * time.Hour
	localNow := now.UTC().Add(offset)
	localStart := localNow.Add(-23 * time.Hour).Truncate(time.Hour)
	return localStart.Add(-offset)
}

func buildOpsProviderPromptCacheSummaryFromLogs(
	logs []model.RelayLog,
	start time.Time,
) model.OpsProviderPromptCacheSummary {
	const bucketCount = 24

	summary := model.OpsProviderPromptCacheSummary{
		Providers: []model.OpsProviderPromptCacheProviderItem{},
		Trend:     make([]model.OpsProviderPromptCacheTrendPoint, bucketCount),
	}

	for i := 0; i < bucketCount; i++ {
		summary.Trend[i] = model.OpsProviderPromptCacheTrendPoint{
			Timestamp: start.Add(time.Duration(i) * time.Hour).Unix(),
		}
	}

	providers := make(map[int]opsProviderPromptCacheAggregate)
	var totalInputTokens int64

	summary.SampledLogCount = int64(len(logs))

	for _, relayLog := range logs {
		usage, ok := parseOpsProviderPromptCacheUsage(relayLog.ResponseContent)
		if !ok {
			continue
		}
		summary.ParsedLogCount++
		if signals, ok := cacheusage.ParseProviderPromptCacheUsageSignals(relayLog.ResponseContent); ok && signals.SemanticCacheHit {
			continue
		}
		// Count as cached if provider returned cached_tokens (>0) or has cache_write tokens (Anthropic prompt cache)
		isCached := usage.CachedTokens > 0 || usage.CacheCreationInputTokens > 0

		aggregate := providers[relayLog.ChannelId]
		if aggregate.ChannelName == "" {
			aggregate.ChannelName = strings.TrimSpace(relayLog.ChannelName)
		}
		aggregate.RequestCount++
		if isCached {
			aggregate.CachedRequestCount++
		}
		aggregate.TotalInputTokens += usage.TotalInputTokens
		totalInputTokens += usage.TotalInputTokens
		aggregate.CacheReadTokens += usage.CachedTokens
		aggregate.CacheWriteTokens += usage.CacheCreationInputTokens
		aggregate.EstimatedCostSaved += estimateOpsProviderPromptCacheSaved(relayLog.ActualModelName, usage)
		providers[relayLog.ChannelId] = aggregate

		bucketIndex := int((time.Unix(relayLog.Time, 0).Sub(start)) / time.Hour)
		if bucketIndex >= 0 && bucketIndex < bucketCount {
			summary.Trend[bucketIndex].RequestCount++
			if isCached {
				summary.Trend[bucketIndex].CachedRequestCount++
			}
			summary.Trend[bucketIndex].CacheReadTokens += usage.CachedTokens
			summary.Trend[bucketIndex].CacheWriteTokens += usage.CacheCreationInputTokens
			summary.Trend[bucketIndex].EstimatedCostSaved += estimateOpsProviderPromptCacheSaved(relayLog.ActualModelName, usage)
		}
	}

	for channelID, aggregate := range providers {
		summary.RequestCount += aggregate.RequestCount
		summary.CachedRequestCount += aggregate.CachedRequestCount
		summary.CacheReadTokens += aggregate.CacheReadTokens
		summary.CacheWriteTokens += aggregate.CacheWriteTokens
		summary.EstimatedCostSaved += aggregate.EstimatedCostSaved

		item := model.OpsProviderPromptCacheProviderItem{
			ChannelID:          channelID,
			ChannelName:        resolveOpsProviderPromptCacheChannelName(channelID, aggregate.ChannelName),
			RequestCount:       aggregate.RequestCount,
			CachedRequestCount: aggregate.CachedRequestCount,
			CacheRate:          percent(aggregate.CachedRequestCount, aggregate.RequestCount),
			CacheReuseRatio:    percent(aggregate.CacheReadTokens, aggregate.TotalInputTokens),
			CacheReadTokens:    aggregate.CacheReadTokens,
			CacheWriteTokens:   aggregate.CacheWriteTokens,
			EstimatedCostSaved: aggregate.EstimatedCostSaved,
		}
		summary.Providers = append(summary.Providers, item)
	}

	sort.SliceStable(summary.Providers, func(i, j int) bool {
		if summary.Providers[i].EstimatedCostSaved != summary.Providers[j].EstimatedCostSaved {
			return summary.Providers[i].EstimatedCostSaved > summary.Providers[j].EstimatedCostSaved
		}
		if summary.Providers[i].CacheReadTokens != summary.Providers[j].CacheReadTokens {
			return summary.Providers[i].CacheReadTokens > summary.Providers[j].CacheReadTokens
		}
		if summary.Providers[i].RequestCount != summary.Providers[j].RequestCount {
			return summary.Providers[i].RequestCount > summary.Providers[j].RequestCount
		}
		return summary.Providers[i].ChannelName < summary.Providers[j].ChannelName
	})

	summary.CacheRate = percent(summary.CachedRequestCount, summary.RequestCount)
	summary.CacheReuseRatio = percent(summary.CacheReadTokens, totalInputTokens)
	summary.UsageSignalAvailable = summary.ParsedLogCount > 0
	for i := range summary.Trend {
		summary.Trend[i].CacheRate = percent(summary.Trend[i].CachedRequestCount, summary.Trend[i].RequestCount)
	}

	return summary
}

func resolveOpsProviderPromptCacheChannelName(channelID int, channelName string) string {
	if name := strings.TrimSpace(channelName); name != "" {
		return name
	}
	if channelID == 0 {
		return "Unknown"
	}
	return fmt.Sprintf("Channel %d", channelID)
}

func loadOpsProviderPromptCacheLogs(ctx context.Context, since time.Time) []model.RelayLog {
	logs := make([]model.RelayLog, 0)
	seen := make(map[int64]struct{})
	relayLogCache, relayLogCacheLock := relaylog.GetCacheAndLock()
	relayLogCacheLock.Lock()
	for _, relayLog := range relayLogCache {
		if relayLog.Time < since.Unix() {
			continue
		}
		logs = append(logs, relayLog)
		seen[relayLog.ID] = struct{}{}
	}
	relayLogCacheLock.Unlock()

	keepEnabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil || !keepEnabled || db.GetDB() == nil {
		return logs
	}

	var dbLogs []model.RelayLog
	if err := db.GetDB().WithContext(ctx).
		Select("id", "time", "channel_id", "channel_name", "actual_model_name", "response_content").
		Where("time >= ?", since.Unix()).
		Order("time ASC").
		Find(&dbLogs).Error; err != nil {
		return logs
	}

	for _, relayLog := range dbLogs {
		if _, ok := seen[relayLog.ID]; ok {
			continue
		}
		logs = append(logs, relayLog)
	}
	return logs
}

func parseOpsProviderPromptCacheUsage(responseContent string) (opsProviderPromptCacheUsage, bool) {
	signals, ok := cacheusage.ParseProviderPromptCacheUsageSignals(responseContent)
	if !ok {
		return opsProviderPromptCacheUsage{}, false
	}

	usage := opsProviderPromptCacheUsage{
		PromptTokens:             signals.PromptTokens,
		CachedTokens:             signals.CachedTokens,
		CacheCreationInputTokens: signals.CacheCreationInputTokens,
	}
	usage.TotalInputTokens = usage.PromptTokens
	if usage.CacheCreationInputTokens > 0 {
		usage.TotalInputTokens += usage.CachedTokens + usage.CacheCreationInputTokens
	}

	if usage.TotalInputTokens <= 0 && usage.CachedTokens <= 0 && usage.CacheCreationInputTokens <= 0 {
		return opsProviderPromptCacheUsage{}, false
	}
	return usage, true
}

func estimateOpsProviderPromptCacheSaved(modelName string, usage opsProviderPromptCacheUsage) float64 {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return 0
	}

	price, err := llm.Get(strings.ToLower(modelName))
	if err != nil || price.Input <= 0 {
		return 0
	}

	cacheReadSavings := float64(usage.CachedTokens) * (price.Input - price.CacheRead) * 1e-6
	cacheWriteCost := 0.0
	if price.CacheWrite > 0 {
		cacheWriteCost = float64(usage.CacheCreationInputTokens) * (price.CacheWrite - price.Input) * 1e-6
	}

	saved := cacheReadSavings - cacheWriteCost
	if saved < 0 {
		return 0
	}
	return saved
}

func percent(part int64, total int64) float64 {
	if total <= 0 || part <= 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func buildSemanticCacheRuntimeConfigFromSettings() (semantic_cache.RuntimeConfig, bool, error) {
	enabled, err := setting.GetBool(model.SettingKeySemanticCacheEnabled)
	if err != nil {
		return semantic_cache.RuntimeConfig{}, false, err
	}
	if !enabled {
		return semantic_cache.RuntimeConfig{}, false, nil
	}

	ttlSeconds, err := setting.GetInt(model.SettingKeySemanticCacheTTL)
	if err != nil || ttlSeconds <= 0 {
		ttlSeconds = semanticCacheDefaultTTLSeconds
	}

	thresholdRaw, err := setting.GetInt(model.SettingKeySemanticCacheThreshold)
	if err != nil || thresholdRaw < 0 || thresholdRaw > 100 {
		thresholdRaw = semanticCacheDefaultThreshold
	}

	maxEntries, err := setting.GetInt(model.SettingKeySemanticCacheMaxEntries)
	if err != nil || maxEntries <= 0 {
		maxEntries = semanticCacheDefaultMaxEntries
	}

	baseURL, err := setting.GetString(model.SettingKeySemanticCacheEmbeddingBaseURL)
	if err != nil {
		return semantic_cache.RuntimeConfig{}, false, err
	}
	modelName, err := setting.GetString(model.SettingKeySemanticCacheEmbeddingModel)
	if err != nil {
		return semantic_cache.RuntimeConfig{}, false, err
	}
	baseURL = strings.TrimSpace(baseURL)
	modelName = strings.TrimSpace(modelName)
	if baseURL == "" || modelName == "" {
		return semantic_cache.RuntimeConfig{}, false, nil
	}

	apiKey, err := setting.GetString(model.SettingKeySemanticCacheEmbeddingAPIKey)
	if err != nil {
		return semantic_cache.RuntimeConfig{}, false, err
	}

	timeoutSeconds, err := setting.GetInt(model.SettingKeySemanticCacheEmbeddingTimeoutSeconds)
	if err != nil || timeoutSeconds <= 0 {
		timeoutSeconds = semanticCacheDefaultTimeoutSec
	}

	return semantic_cache.RuntimeConfig{
		Enabled:          true,
		MaxEntries:       maxEntries,
		Threshold:        float64(thresholdRaw) / 100.0,
		TTL:              time.Duration(ttlSeconds) * time.Second,
		EmbeddingBaseURL: baseURL,
		EmbeddingAPIKey:  strings.TrimSpace(apiKey),
		EmbeddingModel:   modelName,
		EmbeddingTimeout: time.Duration(timeoutSeconds) * time.Second,
	}, true, nil
}

func buildOpsQuotaSummary(apiKeys []model.APIKey, stats []model.StatsAPIKey, now time.Time) model.OpsQuotaSummary {
	usageByKeyID := make(map[int]opsQuotaUsage, len(stats))
	for _, stat := range stats {
		usageByKeyID[stat.APIKeyID] = opsQuotaUsage{
			RequestCount: stat.RequestSuccess + stat.RequestFailed,
			TotalCost:    stat.InputCost + stat.OutputCost,
		}
	}

	items := make([]model.OpsQuotaKeyItem, 0, len(apiKeys))
	summary := model.OpsQuotaSummary{
		TotalKeyCount: len(apiKeys),
	}

	nowUnix := now.Unix()
	for _, apiKey := range apiKeys {
		usage := usageByKeyID[apiKey.ID]
		expired := apiKey.ExpireAt > 0 && apiKey.ExpireAt <= nowUnix
		hasPerModelQuota := hasPerModelQuota(apiKey.PerModelQuotaJSON)
		supportedModelCount := countCommaSeparated(apiKey.SupportedModels)
		limited := apiKey.RateLimitRPM > 0 || apiKey.RateLimitTPM > 0 || apiKey.MaxCost > 0 || hasPerModelQuota
		exhausted := apiKey.MaxCost > 0 && usage.TotalCost >= apiKey.MaxCost

		status := "open"
		switch {
		case !apiKey.Enabled:
			status = "disabled"
		case expired:
			status = "expired"
		case exhausted:
			status = "exhausted"
		case limited:
			status = "limited"
		}

		if apiKey.Enabled {
			summary.EnabledKeyCount++
		}
		if apiKey.Enabled && !expired {
			summary.AvailableKeyCount++
		}
		if expired {
			summary.ExpiredKeyCount++
		}
		if limited {
			summary.LimitedKeyCount++
		} else {
			summary.UnlimitedKeyCount++
		}
		if exhausted {
			summary.ExhaustedKeyCount++
		}
		if hasPerModelQuota {
			summary.PerModelQuotaKeyCount++
		}
		if usage.RequestCount > 0 {
			summary.ActiveUsageKeyCount++
		}
		if apiKey.RateLimitRPM > 0 {
			summary.TotalRPM += apiKey.RateLimitRPM
		}
		if apiKey.RateLimitTPM > 0 {
			summary.TotalTPM += apiKey.RateLimitTPM
		}
		if apiKey.MaxCost > 0 {
			summary.TotalMaxCost += apiKey.MaxCost
		}

		name := strings.TrimSpace(apiKey.Name)
		if name == "" {
			name = "Key"
		}
		items = append(items, model.OpsQuotaKeyItem{
			APIKeyID:            apiKey.ID,
			Name:                name,
			Enabled:             apiKey.Enabled,
			Expired:             expired,
			Status:              status,
			SupportedModelCount: supportedModelCount,
			HasPerModelQuota:    hasPerModelQuota,
			RateLimitRPM:        apiKey.RateLimitRPM,
			RateLimitTPM:        apiKey.RateLimitTPM,
			MaxCost:             apiKey.MaxCost,
			RequestCount:        usage.RequestCount,
			TotalCost:           usage.TotalCost,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		leftRank := opsQuotaStatusRank(items[i].Status)
		rightRank := opsQuotaStatusRank(items[j].Status)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if items[i].RequestCount != items[j].RequestCount {
			return items[i].RequestCount > items[j].RequestCount
		}
		if items[i].TotalCost != items[j].TotalCost {
			return items[i].TotalCost > items[j].TotalCost
		}
		return items[i].Name < items[j].Name
	})

	summary.Keys = items
	return summary
}

func buildOpsHealthStatus(
	databaseOK bool,
	cacheOK bool,
	taskRuntimeOK bool,
	recentErrorCount int64,
	groupHealth []model.AnalyticsGroupHealthItem,
	now time.Time,
) model.OpsHealthStatus {
	status := model.OpsHealthStatus{
		DatabaseOK:       databaseOK,
		CacheOK:          cacheOK,
		TaskRuntimeOK:    taskRuntimeOK,
		RecentErrorCount: recentErrorCount,
		CheckedAt:        now.Unix(),
	}

	failingGroups := make([]model.OpsHealthGroupItem, 0, opsFailingGroupLimit)
	for _, group := range groupHealth {
		switch group.Status {
		case "healthy":
			status.HealthyGroupCount++
		case "warning":
			status.WarningGroupCount++
		case "degraded":
			status.DegradedGroupCount++
		case "down":
			status.DownGroupCount++
		case "empty":
			status.EmptyGroupCount++
		}

		if group.Status == "healthy" {
			continue
		}
		if len(failingGroups) >= opsFailingGroupLimit {
			continue
		}
		failingGroups = append(failingGroups, model.OpsHealthGroupItem{
			GroupID:      group.GroupID,
			GroupName:    group.GroupName,
			EndpointType: group.EndpointType,
			Status:       group.Status,
			FailureCount: group.FailureCount,
			HealthScore:  group.HealthScore,
		})
	}

	status.FailingGroups = failingGroups
	return status
}

func pingDatabase(ctx context.Context) bool {
	gormDB := db.GetDB()
	if gormDB == nil {
		return false
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return false
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return false
	}
	return true
}

func opsTaskRuntimeOK() bool {
	statsSaveIntervalMinutes, err := setting.GetInt(model.SettingKeyStatsSaveInterval)
	if err != nil || statsSaveIntervalMinutes < 1 {
		return false
	}
	syncLLMIntervalHours, err := setting.GetInt(model.SettingKeySyncLLMInterval)
	if err != nil || syncLLMIntervalHours < 1 {
		return false
	}
	modelInfoUpdateIntervalHours, err := setting.GetInt(model.SettingKeyModelInfoUpdateInterval)
	if err != nil || modelInfoUpdateIntervalHours < 1 {
		return false
	}
	if _, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled); err != nil {
		return false
	}
	return true
}

func loadOpsRecentErrorCount(ctx context.Context, since time.Time) (int64, error) {
	startUnix := since.Unix()
	var errorCount int64

	keepEnabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return 0, err
	}

	if keepEnabled {
		if err := db.GetDB().WithContext(ctx).
			Model(&model.RelayLog{}).
			Where("error <> ''").
			Where("time >= ?", startUnix).
			Count(&errorCount).Error; err != nil {
			return 0, err
		}
	}
	cache, lock := relaylog.GetCacheAndLock()
	lock.Lock()
	for _, logItem := range cache {
		if logItem.Error != "" && logItem.Time >= startUnix {
			errorCount++
		}
	}
	lock.Unlock()

	return errorCount, nil
}

func loadOpsAIRouteServicesSummary() ([]model.OpsAIRouteServiceSummary, bool) {
	rawServices, _ := setting.GetString(model.SettingKeyAIRouteServices)
	rawServices = strings.TrimSpace(rawServices)
	if rawServices != "" && rawServices != "[]" {
		var configs []model.AIRouteServiceConfig
		if err := json.Unmarshal([]byte(rawServices), &configs); err == nil {
			return buildOpsAIRouteServices(configs), false
		}
	}

	baseURL, _ := setting.GetString(model.SettingKeyAIRouteBaseURL)
	apiKey, _ := setting.GetString(model.SettingKeyAIRouteAPIKey)
	modelName, _ := setting.GetString(model.SettingKeyAIRouteModel)
	if strings.TrimSpace(baseURL) == "" && strings.TrimSpace(apiKey) == "" && strings.TrimSpace(modelName) == "" {
		return buildOpsAIRouteServices(nil), false
	}

	enabled := strings.TrimSpace(baseURL) != "" && strings.TrimSpace(apiKey) != "" && strings.TrimSpace(modelName) != ""
	configs := []model.AIRouteServiceConfig{{
		Name:    "legacy",
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   modelName,
		Enabled: &enabled,
	}}
	return buildOpsAIRouteServices(configs), true
}

func buildOpsAIRouteServices(configs []model.AIRouteServiceConfig) []model.OpsAIRouteServiceSummary {
	services := make([]model.OpsAIRouteServiceSummary, 0, len(configs))
	for i, cfg := range configs {
		services = append(services, model.OpsAIRouteServiceSummary{
			Name:    airoute.NormalizeAIRouteServiceName(cfg, i+1),
			BaseURL: strings.TrimSpace(cfg.BaseURL),
			Model:   strings.TrimSpace(cfg.Model),
			Enabled: cfg.IsEnabled(),
		})
	}

	sort.SliceStable(services, func(i, j int) bool {
		if services[i].Enabled != services[j].Enabled {
			return services[i].Enabled
		}
		return services[i].Name < services[j].Name
	})
	return services
}

func hasPerModelQuota(raw string) bool {
	value := strings.TrimSpace(raw)
	return value != "" && value != "{}" && value != "null"
}

func countCommaSeparated(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return 0
	}

	seen := make(map[string]struct{})
	count := 0
	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		count++
	}
	return count
}

func opsQuotaStatusRank(status string) int {
	switch status {
	case "exhausted":
		return 0
	case "expired":
		return 1
	case "disabled":
		return 2
	case "limited":
		return 3
	default:
		return 4
	}
}

func countQuotaMonitors(apiKeys []model.APIKey) int {
	count := 0
	for _, k := range apiKeys {
		if k.MaxCost > 0 || k.RateLimitRPM > 0 || k.RateLimitTPM > 0 || hasPerModelQuota(k.PerModelQuotaJSON) {
			count++
		}
	}
	return count
}

func buildOpsTelemetryHeroMetrics(snap telemetry.Snapshot, total model.StatsTotal, uptimeSeconds int64) model.OpsTelemetryHeroMetrics {
	requests := total.RequestSuccess + total.RequestFailed
	failures := total.RequestFailed
	waitTime := total.WaitTime
	if requests <= 0 {
		requests = snap.TotalRequests
		failures = snap.TotalFailures
	}

	avgLatency := snap.AvgLatencyMs
	if requests > 0 && waitTime > 0 {
		avgLatency = float64(waitTime) / float64(requests)
	}

	errorRate := snap.ErrorRate
	if requests > 0 {
		errorRate = float64(failures) / float64(requests) * 100
	}

	return model.OpsTelemetryHeroMetrics{
		UptimeSeconds:     uptimeSeconds,
		TotalRequests:     requests,
		AvgLatencyMs:      avgLatency,
		ErrorRate:         errorRate,
		ActiveConnections: snap.ActiveConnections,
		MemoryUsageMB:     snap.MemoryMB,
	}
}

func buildOpsTelemetryRuntimeSignals(snap telemetry.Snapshot, logs []model.RelayLog, now time.Time) model.OpsTelemetryRuntimeSignals {
	trends := make([]model.OpsTelemetryTrendPoint, 0, len(snap.TrendSnapshots))
	for _, tp := range snap.TrendSnapshots {
		trends = append(trends, model.OpsTelemetryTrendPoint{
			Timestamp:    tp.Timestamp,
			RequestDelta: tp.RequestDelta,
			FailedDelta:  tp.FailedDelta,
			AvgLatencyMs: tp.AvgLatencyMs,
			MemoryMB:     tp.MemoryMB,
		})
	}

	p95 := snap.P95LatencyMs
	if p95 <= 0 {
		p95 = opsTelemetryP95FromLogs(logs)
	}

	throughput := opsTelemetryThroughputFromLogs(logs, now, time.Minute)
	if throughput <= 0 {
		throughput = snap.ThroughputRPS
	}

	if len(logs) > 0 {
		trends = buildOpsTelemetryTrendFromLogs(logs, now, snap.MemoryMB)
	}

	return model.OpsTelemetryRuntimeSignals{
		P95LatencyMs:   p95,
		ThroughputRPS:  throughput,
		MemoryMB:       snap.MemoryMB,
		TrendSnapshots: trends,
	}
}

func opsTelemetryP95FromLogs(logs []model.RelayLog) float64 {
	latencies := make([]int, 0, len(logs))
	for _, logItem := range logs {
		if logItem.UseTime <= 0 {
			continue
		}
		latencies = append(latencies, logItem.UseTime)
	}
	if len(latencies) == 0 {
		return 0
	}
	sort.Ints(latencies)
	idx := (95*len(latencies) + 99) / 100
	if idx < 1 {
		idx = 1
	}
	if idx > len(latencies) {
		idx = len(latencies)
	}
	return float64(latencies[idx-1])
}

func opsTelemetryThroughputFromLogs(logs []model.RelayLog, now time.Time, window time.Duration) float64 {
	if window <= 0 {
		return 0
	}
	since := now.Add(-window).Unix()
	var count int64
	for _, logItem := range logs {
		if logItem.Time >= since && logItem.Time <= now.Unix() {
			count++
		}
	}
	return float64(count) / window.Seconds()
}

func buildOpsTelemetryTrendFromLogs(logs []model.RelayLog, now time.Time, memoryMB int64) []model.OpsTelemetryTrendPoint {
	const bucketCount = 12
	const bucketDuration = 5 * time.Minute

	start := now.Add(-time.Duration(bucketCount-1) * bucketDuration).Truncate(bucketDuration)
	points := make([]model.OpsTelemetryTrendPoint, bucketCount)
	for i := 0; i < bucketCount; i++ {
		points[i] = model.OpsTelemetryTrendPoint{
			Timestamp: start.Add(time.Duration(i) * bucketDuration).Unix(),
			MemoryMB:  memoryMB,
		}
	}

	waitTimes := make([]int64, bucketCount)
	for _, logItem := range logs {
		bucketIndex := int((time.Unix(logItem.Time, 0).Sub(start)) / bucketDuration)
		if bucketIndex < 0 || bucketIndex >= bucketCount {
			continue
		}
		points[bucketIndex].RequestDelta++
		if logItem.Error != "" {
			points[bucketIndex].FailedDelta++
		}
		if logItem.UseTime > 0 {
			waitTimes[bucketIndex] += int64(logItem.UseTime)
		}
	}
	for i := range points {
		if points[i].RequestDelta > 0 && waitTimes[i] > 0 {
			points[i].AvgLatencyMs = float64(waitTimes[i]) / float64(points[i].RequestDelta)
		}
	}
	return points
}

func loadOpsTelemetryLogs(ctx context.Context, since time.Time) []model.RelayLog {
	logs := make([]model.RelayLog, 0)
	seen := make(map[int64]struct{})

	cache, lock := relaylog.GetCacheAndLock()
	lock.Lock()
	for _, logItem := range cache {
		if logItem.Time < since.Unix() {
			continue
		}
		logs = append(logs, logItem)
		if logItem.ID != 0 {
			seen[logItem.ID] = struct{}{}
		}
	}
	lock.Unlock()

	keepEnabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil || !keepEnabled || db.GetDB() == nil {
		return logs
	}

	var dbLogs []model.RelayLog
	if err := db.GetDB().WithContext(ctx).
		Select("id", "time", "use_time", "error").
		Where("time >= ?", since.Unix()).
		Order("time ASC").
		Find(&dbLogs).Error; err != nil {
		return logs
	}
	for _, logItem := range dbLogs {
		if logItem.ID != 0 {
			if _, ok := seen[logItem.ID]; ok {
				continue
			}
		}
		logs = append(logs, logItem)
	}
	return logs
}

func TelemetrySummaryGet(ctx context.Context) (*model.OpsTelemetrySummary, error) {
	summary := &model.OpsTelemetrySummary{}
	snap := telemetry.Global().Snapshot()
	now := time.Now()
	totalStats := stats.TotalGet()
	telemetryLogs := loadOpsTelemetryLogs(ctx, now.Add(-time.Hour))

	// ── Hero ──
	summary.Hero = buildOpsTelemetryHeroMetrics(snap, totalStats, int64(time.Since(processStartTime).Seconds()))

	// ── RuntimeSignals ──
	summary.RuntimeSignals = buildOpsTelemetryRuntimeSignals(snap, telemetryLogs, now)

	// ── DatabaseHealth ──
	dbOK := pingDatabase(ctx)
	taskOK := opsTaskRuntimeOK()
	dbStatus := "healthy"
	var dbIssues []string
	if !dbOK {
		dbStatus = "degraded"
		dbIssues = append(dbIssues, "Database connection failed")
	}
	if !taskOK {
		if dbStatus == "healthy" {
			dbStatus = "degraded"
		}
		dbIssues = append(dbIssues, "Background task runtime degraded")
	}

	summary.DatabaseHealth = model.OpsTelemetryDatabaseHealth{
		Status:  dbStatus,
		Issues:  dbIssues,
		Repairs: 0,
	}

	// ── SessionQuotaActivity ──
	apiKeys, err := apikey.List(ctx)
	sessionsByAPIKey := 0
	if err == nil {
		sessionsByAPIKey = len(apiKeys)
	}

	summary.SessionQuotaActivity = model.OpsTelemetrySessionQuotaActivity{
		ActiveSessions:      int(snap.ActiveSessions),
		StickyBoundSessions: int(snap.StickyBoundSessions),
		QuotaAlerts:         int(snap.QuotaAlerts),
		SessionsByAPIKey:    sessionsByAPIKey,
		QuotaMonitors:       countQuotaMonitors(apiKeys),
	}

	// ── PromptCache ──
	cacheStatus, err := OpsCacheStatusGet(ctx)
	if err == nil {
		summary.PromptCache = model.OpsTelemetryPromptCache{
			Entries:    cacheStatus.CurrentEntries,
			HitRate:    cacheStatus.HitRate,
			Hits:       cacheStatus.Hits,
			Misses:     cacheStatus.Misses,
			MaxEntries: cacheStatus.MaxEntries,
			UsageRate:  cacheStatus.UsageRate,
		}
	}

	// ── ProviderHealth ──
	channels, err := channel.List(ctx)
	if err == nil {
		statsChannels := stats.ChannelList()
		statsMap := make(map[int]model.StatsChannel, len(statsChannels))
		for _, sc := range statsChannels {
			statsMap[sc.ChannelID] = sc
		}

		providers := make([]model.OpsTelemetryProviderItem, 0, len(channels))
		active := 0
		for _, ch := range channels {
			stats := statsMap[ch.ID]
			requests := stats.RequestSuccess + stats.RequestFailed
			var successRate float64
			if requests > 0 {
				successRate = float64(stats.RequestSuccess) / float64(requests) * 100
			}
			var avgLat float64
			if requests > 0 {
				avgLat = float64(stats.WaitTime) / float64(requests)
			}

			healthStatus := "healthy"
			healthHint := "Normal"
			if !ch.Enabled {
				healthStatus = "disabled"
				healthHint = "Channel disabled"
			} else if requests > 0 && float64(stats.RequestFailed)/float64(requests) > 0.5 {
				healthStatus = "degraded"
				healthHint = "High failure rate"
			} else if requests > 0 && stats.RequestFailed > 0 {
				healthStatus = "warning"
				healthHint = "Some failures"
			}

			baseURL := ""
			if len(ch.BaseUrls) > 0 {
				baseURL = ch.BaseUrls[0].URL
			}

			providers = append(providers, model.OpsTelemetryProviderItem{
				ChannelID:        ch.ID,
				ChannelName:      ch.Name,
				Enabled:          ch.Enabled,
				BaseURL:          baseURL,
				RequestCount:     requests,
				SuccessRate:      successRate,
				AverageLatencyMs: avgLat,
				HealthStatus:     healthStatus,
				HealthHint:       healthHint,
			})

			if ch.Enabled {
				active++
			}
		}

		sort.SliceStable(providers, func(i, j int) bool {
			aStatus := providers[i].HealthStatus
			bStatus := providers[j].HealthStatus
			aBad := aStatus == "degraded" || aStatus == "disabled"
			bBad := bStatus == "degraded" || bStatus == "disabled"
			if aBad != bBad {
				return aBad
			}
			if providers[i].RequestCount != providers[j].RequestCount {
				return providers[i].RequestCount > providers[j].RequestCount
			}
			return providers[i].ChannelName < providers[j].ChannelName
		})

		summary.ProviderHealth = model.OpsTelemetryProviderHealth{
			Providers: providers,
			Active:    active,
			Monitored: len(channels),
		}
	}

	// ── DrilldownShortcuts ──
	summary.DrilldownShortcuts = []model.OpsTelemetryDrilldownShortcut{
		{Key: "cache", Label: "Prompt Cache"},
		{Key: "quota", Label: "Quota Summary"},
		{Key: "health", Label: "Health Status"},
		{Key: "system", Label: "System Config"},
		{Key: "audit", Label: "Audit Log"},
	}

	return summary, nil
}

func processMemoryMB() int64 {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return int64(mem.Alloc / (1024 * 1024))
}
