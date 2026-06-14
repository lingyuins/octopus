package analytics

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/apikey"
	"github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/op/group"
	"github.com/lingyuins/octopus/internal/op/navorder"
	"github.com/lingyuins/octopus/internal/op/relaylog"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/op/stats"
	"github.com/lingyuins/octopus/internal/relay/balancer"
	"github.com/lingyuins/octopus/internal/utils/semantic_cache"
	"gorm.io/gorm"
)

const analyticsRouteHealthFailureWindow = 24 * time.Hour

type analyticsAggregateMetrics struct {
	InputTokens    int64
	OutputTokens   int64
	TotalCost      float64
	RequestSuccess int64
	RequestFailed  int64
}

type analyticsSummaryRow struct {
	analyticsAggregateMetrics
	RequestCount  int64
	FallbackCount int64
}

type analyticsProviderAggregateRow struct {
	ChannelID   int
	ChannelName string
	analyticsAggregateMetrics
}

type analyticsModelAggregateRow struct {
	ModelName string
	analyticsAggregateMetrics
}

type analyticsAPIKeyAggregateRow struct {
	APIKeyID int
	Name     string
	analyticsAggregateMetrics
}

type analyticsChannelModelAggregateRow struct {
	ChannelID   int
	ChannelName string
	ModelName   string
	analyticsAggregateMetrics
}

type analyticsFailureAggregateRow struct {
	ChannelID        int
	RequestModelName string
	ActualModelName  string
	FailureCount     int64
	LastFailureAt    int64
}

func AnalyticsOverviewGet(ctx context.Context, r model.AnalyticsRange) (*model.AnalyticsOverview, error) {
	daily, err := stats.GetDaily(ctx)
	if err != nil {
		return nil, err
	}
	mergedDaily := mergeAnalyticsDailyWithToday(daily, stats.TodayGet())
	metrics := aggregateAnalyticsDailyMetrics(mergedDaily, r, stats.Now())

	channels, err := channel.List(ctx)
	if err != nil {
		return nil, err
	}
	apiKeys, err := apikey.List(ctx)
	if err != nil {
		return nil, err
	}

	providerCount := 0
	modelNames := make(map[string]struct{})
	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}
		providerCount++
		for _, modelName := range splitAnalyticsChannelModels(ch) {
			modelNames[modelName] = struct{}{}
		}
	}

	apiKeyCount := 0
	for _, apiKey := range apiKeys {
		if apiKey.Enabled {
			apiKeyCount++
		}
	}

	logSummary, err := loadAnalyticsSummary(ctx, r)
	if err != nil {
		return nil, err
	}
	fallbackRate := 0.0
	if logSummary.RequestCount > 0 {
		fallbackRate = (float64(logSummary.FallbackCount) / float64(logSummary.RequestCount)) * 100
	}

	overview := buildAnalyticsOverview(metrics, providerCount, apiKeyCount, len(modelNames), fallbackRate)
	return &overview, nil
}

func AnalyticsUtilizationGet(ctx context.Context, r model.AnalyticsRange) (*model.AnalyticsUtilization, error) {
	providerBreakdown, err := AnalyticsProviderBreakdownGet(ctx, r)
	if err != nil {
		return nil, err
	}
	modelBreakdown, err := AnalyticsModelBreakdownGet(ctx, r)
	if err != nil {
		return nil, err
	}
	apiKeyBreakdown, err := AnalyticsAPIKeyBreakdownGet(ctx, r)
	if err != nil {
		return nil, err
	}

	return &model.AnalyticsUtilization{
		ProviderBreakdown: providerBreakdown,
		ModelBreakdown:    modelBreakdown,
		APIKeyBreakdown:   apiKeyBreakdown,
	}, nil
}

func AnalyticsProviderBreakdownGet(ctx context.Context, r model.AnalyticsRange) ([]model.AnalyticsProviderBreakdownItem, error) {
	rows, err := loadAnalyticsProviderRows(ctx, r)
	if err != nil {
		return nil, err
	}

	channels, err := channel.List(ctx)
	if err != nil {
		return nil, err
	}
	channelByID := make(map[int]model.Channel, len(channels))
	for _, ch := range channels {
		channelByID[ch.ID] = ch
	}

	return buildProviderBreakdown(rows, channelByID), nil
}

func AnalyticsModelBreakdownGet(ctx context.Context, r model.AnalyticsRange) ([]model.AnalyticsModelBreakdownItem, error) {
	rows, err := loadAnalyticsModelRows(ctx, r)
	if err != nil {
		return nil, err
	}
	return buildModelBreakdown(rows), nil
}

func AnalyticsAPIKeyBreakdownGet(ctx context.Context, r model.AnalyticsRange) ([]model.AnalyticsAPIKeyBreakdownItem, error) {
	rows, err := loadAnalyticsAPIKeyRows(ctx, r)
	if err != nil {
		return nil, err
	}
	return buildAPIKeyBreakdown(rows), nil
}

// AnalyticsChannelModelBreakdownGet 返回 (渠道,模型) 交叉维度的统计。
// 成功/失败基于单次尝试（relay_log_attempts）聚合，使"渠道A 失败→重试到B 成功"的请求中
// 渠道A 的失败也反映到 A 的成功率上（issue #67）。token/cost 按请求顶层渠道归属
// （与 attempts 表的 channel 维度一致时才计入）。groupID 非空时只返回该组包含的
// (渠道,模型) 组合。
func AnalyticsChannelModelBreakdownGet(ctx context.Context, r model.AnalyticsRange, groupID *int) ([]model.AnalyticsChannelModelItem, error) {
	channels, err := channel.List(ctx)
	if err != nil {
		return nil, err
	}
	channelByID := make(map[int]model.Channel, len(channels))
	for _, ch := range channels {
		channelByID[ch.ID] = ch
	}

	// 可选：按分组 scope 过滤 (channelID, modelName) 集合。
	scope := make(map[string]struct{})
	if groupID != nil {
		groups, err := group.GroupList(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range groups {
			if g.ID != *groupID {
				continue
			}
			for _, it := range g.Items {
				scope[strconv.Itoa(it.ChannelID)+"\x00"+it.ModelName] = struct{}{}
			}
		}
	}

	rows, err := loadAnalyticsChannelModelRows(ctx, r, scope)
	if err != nil {
		return nil, err
	}
	return buildChannelModelBreakdown(rows, channelByID), nil
}

// AnalyticsAutoStrategyGet 返回 Auto 策略运行态快照（滑动窗口内的成功率/样本数/延迟）。
// groupID 非空时只返回该组包含渠道的条目；为空时返回全部。供"Auto 实时表现"展示（issue #67）。
func AnalyticsAutoStrategyGet(ctx context.Context, groupID *int) ([]model.AutoStrategySnapshotItem, error) {
	channels, err := channel.List(ctx)
	if err != nil {
		return nil, err
	}
	channelByID := make(map[int]model.Channel, len(channels))
	for _, ch := range channels {
		channelByID[ch.ID] = ch
	}

	var channelIDs []int
	if groupID != nil {
		groups, err := group.GroupList(ctx)
		if err != nil {
			return nil, err
		}
		seen := make(map[int]struct{})
		for _, g := range groups {
			if g.ID != *groupID {
				continue
			}
			for _, it := range g.Items {
				if _, ok := seen[it.ChannelID]; ok {
					continue
				}
				seen[it.ChannelID] = struct{}{}
				channelIDs = append(channelIDs, it.ChannelID)
			}
		}
	}

	snapshot := balancer.GetAutoStatsSnapshot(channelIDs)
	minSamples := balancer.GetAutoStrategyMinSamples()

	items := make([]model.AutoStrategySnapshotItem, 0, len(snapshot))
	for _, s := range snapshot {
		var lastActive int64
		if !s.LastActiveAt.IsZero() {
			lastActive = s.LastActiveAt.Unix()
		}
		chName := ""
		enabled := false
		if c, ok := channelByID[s.ChannelID]; ok {
			chName = c.Name
			enabled = c.Enabled
		}
		items = append(items, model.AutoStrategySnapshotItem{
			ChannelID:     s.ChannelID,
			ChannelName:   chName,
			Enabled:       enabled,
			ModelName:     s.ModelName,
			SuccessRate:   s.SuccessRate * 100,
			SampleCount:   s.SampleCount,
			AvgLatencyMs:  s.AvgLatencyMs,
			LastActiveAt:  lastActive,
			MinSamplesMet: s.SampleCount >= minSamples,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		// 成功率低的优先（突出问题渠道），其次样本数、渠道名、模型名。
		if items[i].SuccessRate != items[j].SuccessRate {
			return items[i].SuccessRate < items[j].SuccessRate
		}
		if items[i].SampleCount != items[j].SampleCount {
			return items[i].SampleCount > items[j].SampleCount
		}
		if items[i].ChannelName != items[j].ChannelName {
			return items[i].ChannelName < items[j].ChannelName
		}
		return items[i].ModelName < items[j].ModelName
	})

	return items, nil
}

func AnalyticsGroupHealthGet(ctx context.Context) ([]model.AnalyticsGroupHealthItem, error) {
	groups, err := group.GroupList(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := channel.List(ctx)
	if err != nil {
		return nil, err
	}

	channelByID := make(map[int]model.Channel, len(channels))
	for _, ch := range channels {
		channelByID[ch.ID] = ch
		channelByID[ch.ID] = ch
	}

	failures, err := loadAnalyticsFailureRows(ctx, stats.Now().Add(-analyticsRouteHealthFailureWindow))
	if err != nil {
		return nil, err
	}

	return buildGroupHealth(groups, channelByID, failures), nil
}

func AnalyticsEvaluationGet(_ context.Context) (*model.AnalyticsEvaluationSummary, error) {
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

	hits, misses, currentEntries := semantic_cache.Stats()
	summary := &model.AnalyticsEvaluationSummary{
		SemanticCache: navorder.BuildSemanticCacheEvaluationSummary(
			enabled,
			semantic_cache.RuntimeEnabled(),
			ttlSeconds,
			threshold,
			maxEntries,
			currentEntries,
			hits,
			misses,
			semantic_cache.GetRuntimeStats(),
		),
	}
	return summary, nil
}

func mergeAnalyticsDailyWithToday(daily []model.StatsDaily, today model.StatsDaily) []model.StatsDaily {
	if today.Date == "" {
		return daily
	}

	merged := make([]model.StatsDaily, 0, len(daily)+1)
	replaced := false
	for _, item := range daily {
		if item.Date == today.Date {
			merged = append(merged, today)
			replaced = true
			continue
		}
		merged = append(merged, item)
	}
	if !replaced {
		merged = append(merged, today)
	}
	return merged
}

func aggregateAnalyticsDailyMetrics(daily []model.StatsDaily, r model.AnalyticsRange, now time.Time) model.StatsMetrics {
	startDate := analyticsStartDate(r, now)
	var metrics model.StatsMetrics
	for _, item := range daily {
		if startDate != "" && item.Date < startDate {
			continue
		}
		metrics.Add(item.StatsMetrics)
	}
	return metrics
}

func buildAnalyticsOverview(metrics model.StatsMetrics, providerCount, apiKeyCount, modelCount int, fallbackRate float64) model.AnalyticsOverview {
	requestCount := metrics.RequestSuccess + metrics.RequestFailed
	successRate := 0.0
	if requestCount > 0 {
		successRate = (float64(metrics.RequestSuccess) / float64(requestCount)) * 100
	}

	return model.AnalyticsOverview{
		AnalyticsMetrics: model.AnalyticsMetrics{
			RequestCount: requestCount,
			TotalTokens:  metrics.InputToken + metrics.OutputToken,
			InputTokens:  metrics.InputToken,
			OutputTokens: metrics.OutputToken,
			TotalCost:    metrics.InputCost + metrics.OutputCost,
			SuccessRate:  successRate,
		},
		ProviderCount: providerCount,
		APIKeyCount:   apiKeyCount,
		ModelCount:    modelCount,
		FallbackRate:  fallbackRate,
	}
}

func buildProviderBreakdown(rows map[int]*analyticsProviderAggregateRow, channelByID map[int]model.Channel) []model.AnalyticsProviderBreakdownItem {
	items := make([]model.AnalyticsProviderBreakdownItem, 0, len(rows))
	for channelID, row := range rows {
		if row == nil {
			continue
		}

		requestCount := row.RequestSuccess + row.RequestFailed
		successRate := 0.0
		if requestCount > 0 {
			successRate = (float64(row.RequestSuccess) / float64(requestCount)) * 100
		}

		channelName := strings.TrimSpace(row.ChannelName)
		enabled := false
		if c, ok := channelByID[channelID]; ok {
			if channelName == "" {
				channelName = c.Name
			}
			enabled = c.Enabled
		}
		if channelName == "" {
			channelName = "Unknown Channel"
		}

		items = append(items, model.AnalyticsProviderBreakdownItem{
			ChannelID:   channelID,
			ChannelName: channelName,
			Enabled:     enabled,
			AnalyticsMetrics: model.AnalyticsMetrics{
				RequestCount: requestCount,
				TotalTokens:  row.InputTokens + row.OutputTokens,
				InputTokens:  row.InputTokens,
				OutputTokens: row.OutputTokens,
				TotalCost:    row.TotalCost,
				SuccessRate:  successRate,
			},
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].RequestCount != items[j].RequestCount {
			return items[i].RequestCount > items[j].RequestCount
		}
		if items[i].TotalCost != items[j].TotalCost {
			return items[i].TotalCost > items[j].TotalCost
		}
		return items[i].ChannelName < items[j].ChannelName
	})

	return items
}

func buildModelBreakdown(rows map[string]*analyticsModelAggregateRow) []model.AnalyticsModelBreakdownItem {
	items := make([]model.AnalyticsModelBreakdownItem, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		modelName := strings.TrimSpace(row.ModelName)
		if modelName == "" {
			continue
		}

		requestCount := row.RequestSuccess + row.RequestFailed
		successRate := 0.0
		if requestCount > 0 {
			successRate = (float64(row.RequestSuccess) / float64(requestCount)) * 100
		}

		items = append(items, model.AnalyticsModelBreakdownItem{
			ModelName: modelName,
			AnalyticsMetrics: model.AnalyticsMetrics{
				RequestCount: requestCount,
				TotalTokens:  row.InputTokens + row.OutputTokens,
				InputTokens:  row.InputTokens,
				OutputTokens: row.OutputTokens,
				TotalCost:    row.TotalCost,
				SuccessRate:  successRate,
			},
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].RequestCount != items[j].RequestCount {
			return items[i].RequestCount > items[j].RequestCount
		}
		if items[i].TotalCost != items[j].TotalCost {
			return items[i].TotalCost > items[j].TotalCost
		}
		return items[i].ModelName < items[j].ModelName
	})

	return items
}

func buildAPIKeyBreakdown(rows map[string]*analyticsAPIKeyAggregateRow) []model.AnalyticsAPIKeyBreakdownItem {
	items := make([]model.AnalyticsAPIKeyBreakdownItem, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		name := strings.TrimSpace(row.Name)
		if name == "" {
			if row.APIKeyID > 0 {
				name = "Key #" + strconv.Itoa(row.APIKeyID)
			} else {
				name = "Unknown Key"
			}
		}

		requestCount := row.RequestSuccess + row.RequestFailed
		successRate := 0.0
		if requestCount > 0 {
			successRate = (float64(row.RequestSuccess) / float64(requestCount)) * 100
		}

		item := model.AnalyticsAPIKeyBreakdownItem{
			Name: name,
			AnalyticsMetrics: model.AnalyticsMetrics{
				RequestCount: requestCount,
				TotalTokens:  row.InputTokens + row.OutputTokens,
				InputTokens:  row.InputTokens,
				OutputTokens: row.OutputTokens,
				TotalCost:    row.TotalCost,
				SuccessRate:  successRate,
			},
		}
		if row.APIKeyID > 0 {
			id := row.APIKeyID
			item.APIKeyID = &id
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].RequestCount != items[j].RequestCount {
			return items[i].RequestCount > items[j].RequestCount
		}
		if items[i].TotalCost != items[j].TotalCost {
			return items[i].TotalCost > items[j].TotalCost
		}
		return items[i].Name < items[j].Name
	})

	return items
}

func buildChannelModelBreakdown(rows map[string]*analyticsChannelModelAggregateRow, channelByID map[int]model.Channel) []model.AnalyticsChannelModelItem {
	items := make([]model.AnalyticsChannelModelItem, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		channelName := strings.TrimSpace(row.ChannelName)
		enabled := false
		if c, ok := channelByID[row.ChannelID]; ok {
			if channelName == "" {
				channelName = c.Name
			}
			enabled = c.Enabled
		}
		if channelName == "" {
			channelName = "Unknown Channel"
		}

		requestCount := row.RequestSuccess + row.RequestFailed
		successRate := 0.0
		if requestCount > 0 {
			successRate = (float64(row.RequestSuccess) / float64(requestCount)) * 100
		}

		items = append(items, model.AnalyticsChannelModelItem{
			ChannelID:   row.ChannelID,
			ChannelName: channelName,
			ModelName:   row.ModelName,
			Enabled:     enabled,
			AnalyticsMetrics: model.AnalyticsMetrics{
				RequestCount: requestCount,
				TotalTokens:  row.InputTokens + row.OutputTokens,
				InputTokens:  row.InputTokens,
				OutputTokens: row.OutputTokens,
				TotalCost:    row.TotalCost,
				SuccessRate:  successRate,
			},
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		// 失败数大的优先（突出问题渠道），其次请求数、费用、名称。
		// 失败数 = round(RequestCount * (1 - SuccessRate/100))。
		ifailed := int64(float64(items[i].RequestCount) * (1 - items[i].SuccessRate/100))
		jfailed := int64(float64(items[j].RequestCount) * (1 - items[j].SuccessRate/100))
		if ifailed != jfailed {
			return ifailed > jfailed
		}
		if items[i].RequestCount != items[j].RequestCount {
			return items[i].RequestCount > items[j].RequestCount
		}
		if items[i].ChannelName != items[j].ChannelName {
			return items[i].ChannelName < items[j].ChannelName
		}
		return items[i].ModelName < items[j].ModelName
	})

	return items
}

func buildGroupHealth(groups []model.Group, channelByID map[int]model.Channel, failures map[string]*analyticsFailureAggregateRow) []model.AnalyticsGroupHealthItem {
	items := make([]model.AnalyticsGroupHealthItem, 0, len(groups))
	for _, group := range groups {
		itemCount := len(group.Items)
		enabledItemCount := 0
		disabledItemCount := 0
		failureCount := int64(0)
		lastFailureAt := int64(0)

		// 按 (channelID,modelName) 聚合失败，供下钻展示。
		type chanFail struct {
			ChannelID     int
			ChannelName   string
			ModelName     string
			FailureCount  int64
			LastFailureAt int64
		}
		chanFailures := make(map[string]*chanFail)
		seenFailureKeys := make(map[string]struct{})
		for _, item := range group.Items {
			c, ok := channelByID[item.ChannelID]
			if ok && c.Enabled {
				enabledItemCount++
			} else {
				disabledItemCount++
			}

			for _, key := range []string{
				makeAnalyticsFailureKey(item.ChannelID, item.ModelName, item.ModelName),
				makeAnalyticsFailureKey(item.ChannelID, item.ModelName, group.Name),
			} {
				if _, ok := seenFailureKeys[key]; ok {
					continue
				}
				seenFailureKeys[key] = struct{}{}
				failure, ok := failures[key]
				if !ok || failure == nil {
					continue
				}
				failureCount += failure.FailureCount
				if failure.LastFailureAt > lastFailureAt {
					lastFailureAt = failure.LastFailureAt
				}

				// 按 (channelID, model) 聚合到下钻 map。model 优先用 attempt 的 actual，
				// 没有则用 item.ModelName。
				cfKey := strconv.Itoa(item.ChannelID) + "\x00" + item.ModelName
				cf, ok := chanFailures[cfKey]
				if !ok {
					cf = &chanFail{
						ChannelID: item.ChannelID,
						ModelName: item.ModelName,
					}
					if c, ok := channelByID[item.ChannelID]; ok {
						cf.ChannelName = c.Name
					}
					chanFailures[cfKey] = cf
				}
				cf.FailureCount += failure.FailureCount
				if failure.LastFailureAt > cf.LastFailureAt {
					cf.LastFailureAt = failure.LastFailureAt
				}
			}
		}

		status := "healthy"
		score := 100
		switch {
		case itemCount == 0:
			status = "empty"
			score = 0
		case enabledItemCount == 0:
			status = "down"
			score = 20
		default:
			score -= (disabledItemCount * 40) / itemCount
			if failureCount > 0 {
				penalty := int(failureCount * 12)
				if penalty > 48 {
					penalty = 48
				}
				score -= penalty
			}
			if disabledItemCount > 0 || failureCount >= 3 {
				status = "degraded"
			} else if failureCount > 0 {
				status = "warning"
			}
		}

		if score < 0 {
			score = 0
		}

		// 仅保留有失败的渠道，按失败数降序，取前 10 供下钻展示。
		failingChannels := make([]model.FailingChannelItem, 0, len(chanFailures))
		for _, cf := range chanFailures {
			if cf.FailureCount <= 0 {
				continue
			}
			failingChannels = append(failingChannels, model.FailingChannelItem{
				ChannelID:     cf.ChannelID,
				ChannelName:   cf.ChannelName,
				ModelName:     cf.ModelName,
				FailureCount:  cf.FailureCount,
				LastFailureAt: cf.LastFailureAt,
			})
		}
		sort.SliceStable(failingChannels, func(i, j int) bool {
			if failingChannels[i].FailureCount != failingChannels[j].FailureCount {
				return failingChannels[i].FailureCount > failingChannels[j].FailureCount
			}
			return failingChannels[i].ChannelName < failingChannels[j].ChannelName
		})
		if len(failingChannels) > 10 {
			failingChannels = failingChannels[:10]
		}

		items = append(items, model.AnalyticsGroupHealthItem{
			GroupID:           group.ID,
			GroupName:         group.Name,
			EndpointType:      group.EndpointType,
			ItemCount:         itemCount,
			EnabledItemCount:  enabledItemCount,
			DisabledItemCount: disabledItemCount,
			FailureCount:      failureCount,
			LastFailureAt:     lastFailureAt,
			HealthScore:       score,
			Status:            status,
			Mode:              int(group.Mode),
			FailingChannels:   failingChannels,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].HealthScore != items[j].HealthScore {
			return items[i].HealthScore < items[j].HealthScore
		}
		if items[i].FailureCount != items[j].FailureCount {
			return items[i].FailureCount > items[j].FailureCount
		}
		return items[i].GroupName < items[j].GroupName
	})

	return items
}

func loadAnalyticsSummary(ctx context.Context, r model.AnalyticsRange) (*analyticsSummaryRow, error) {
	startUnix := analyticsRangeStartUnix(r, stats.Now())
	row := &analyticsSummaryRow{}

	keepEnabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return nil, err
	}

	if keepEnabled {
		query := db.GetDB().WithContext(ctx).
			Model(&model.RelayLog{}).
			Select(`
				COUNT(*) AS request_count,
				COALESCE(SUM(CASE WHEN total_attempts > 1 THEN 1 ELSE 0 END), 0) AS fallback_count
			`)
		if startUnix != nil {
			query = query.Where("time >= ?", *startUnix)
		}
		if err := query.Scan(row).Error; err != nil {
			return nil, err
		}
	}

	cache, lock := relaylog.GetCacheAndLock()
	lock.Lock()
	for _, logItem := range cache {
		if startUnix != nil && logItem.Time < *startUnix {
			continue
		}
		row.RequestCount++
		if logItem.TotalAttempts > 1 {
			row.FallbackCount++
		}
	}
	lock.Unlock()

	return row, nil
}

// loadAnalyticsChannelModelRows 聚合 (渠道,模型) 维度的成功/失败/token/cost。
// 成功/失败按单次尝试（relay_log_attempts）统计；token/cost 取自 relay_logs 且仅在
// 该请求最终成功时计入（避免把整体失败的请求 token 重复计入多个渠道）。
// scope 非空时只保留其中的 (channelID,modelName) 组合。
func loadAnalyticsChannelModelRows(ctx context.Context, r model.AnalyticsRange, scope map[string]struct{}) (map[string]*analyticsChannelModelAggregateRow, error) {
	startUnix := analyticsRangeStartUnix(r, stats.Now())
	rows := make(map[string]*analyticsChannelModelAggregateRow)

	keepEnabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return nil, err
	}

	inScope := func(channelID int, modelName string) bool {
		if len(scope) == 0 {
			return true
		}
		_, ok := scope[strconv.Itoa(channelID)+"\x00"+modelName]
		return ok
	}

	if keepEnabled {
		conn := db.GetLogDB()
		if conn != nil && connHasRelayLogAttempts(conn) {
			// 成功/失败：按尝试维度聚合。
			type attRow struct {
				ChannelID    int     `gorm:"column:channel_id"`
				ModelName    string  `gorm:"column:model_name"`
				Success      int64   `gorm:"column:request_success"`
				Failed       int64   `gorm:"column:request_failed"`
				InputTokens  int64   `gorm:"column:input_tokens"`
				OutputTokens int64   `gorm:"column:output_tokens"`
				TotalCost    float64 `gorm:"column:total_cost"`
			}
			var aRows []attRow
			query := conn.WithContext(ctx).
				Table("relay_log_attempts AS a").
				Select(`
					a.channel_id,
					COALESCE(NULLIF(a.model_name, ''), l.request_model_name) AS model_name,
					COALESCE(SUM(CASE WHEN a.status = ? THEN 1 ELSE 0 END), 0) AS request_success,
					COALESCE(SUM(CASE WHEN a.status = ? THEN 1 ELSE 0 END), 0) AS request_failed,
					COALESCE(SUM(CASE WHEN a.status = ? THEN l.input_tokens ELSE 0 END), 0) AS input_tokens,
					COALESCE(SUM(CASE WHEN a.status = ? THEN l.output_tokens ELSE 0 END), 0) AS output_tokens,
					COALESCE(SUM(CASE WHEN a.status = ? THEN l.cost ELSE 0 END), 0) AS total_cost
				`, string(model.AttemptSuccess), string(model.AttemptFailed),
					string(model.AttemptSuccess), string(model.AttemptSuccess), string(model.AttemptSuccess)).
				Joins("JOIN relay_logs AS l ON l.id = a.relay_log_id").
				Group("a.channel_id, COALESCE(NULLIF(a.model_name, ''), l.request_model_name)")
			if startUnix != nil {
				query = query.Where("a.time >= ?", *startUnix)
			}
			if err := query.Scan(&aRows).Error; err != nil {
				return nil, err
			}
			for _, ar := range aRows {
				if !inScope(ar.ChannelID, ar.ModelName) {
					continue
				}
				key := strconv.Itoa(ar.ChannelID) + "\x00" + ar.ModelName
				rows[key] = &analyticsChannelModelAggregateRow{
					ChannelID: ar.ChannelID,
					ModelName: ar.ModelName,
					analyticsAggregateMetrics: analyticsAggregateMetrics{
						InputTokens:    ar.InputTokens,
						OutputTokens:   ar.OutputTokens,
						TotalCost:      ar.TotalCost,
						RequestSuccess: ar.Success,
						RequestFailed:  ar.Failed,
					},
				}
			}
		} else {
			// 回退：无 attempts 表时用顶层列（与历史利用率一致）。
			mainConn := db.GetDB()
			if mainConn != nil {
				var dbRows []analyticsChannelModelAggregateRow
				modelExpr := "COALESCE(NULLIF(actual_model_name, ''), request_model_name)"
				query := mainConn.WithContext(ctx).
					Model(&model.RelayLog{}).
					Select(`
						channel_id,
						channel_name,
						` + modelExpr + ` AS model_name,
						COALESCE(SUM(input_tokens), 0) AS input_tokens,
						COALESCE(SUM(output_tokens), 0) AS output_tokens,
						COALESCE(SUM(cost), 0) AS total_cost,
						COALESCE(SUM(CASE WHEN error = '' THEN 1 ELSE 0 END), 0) AS request_success,
						COALESCE(SUM(CASE WHEN error <> '' THEN 1 ELSE 0 END), 0) AS request_failed
					`).
					Group("channel_id, channel_name, " + modelExpr)
				if startUnix != nil {
					query = query.Where("time >= ?", *startUnix)
				}
				if err := query.Scan(&dbRows).Error; err != nil {
					return nil, err
				}
				for _, row := range dbRows {
					modelName := strings.TrimSpace(row.ModelName)
					if modelName == "" || !inScope(row.ChannelID, modelName) {
						continue
					}
					key := strconv.Itoa(row.ChannelID) + "\x00" + modelName
					rowCopy := row
					rowCopy.ModelName = modelName
					rows[key] = &rowCopy
				}
			}
		}
	}

	// 合并内存缓存（含尚未落库的失败尝试维度）。
	cache, lock := relaylog.GetCacheAndLock()
	lock.Lock()
	for _, logItem := range cache {
		if startUnix != nil && logItem.Time < *startUnix {
			continue
		}
		success := logItem.Error == ""
		for _, a := range logItem.Attempts {
			if a.ChannelID == 0 {
				continue
			}
			modelName := strings.TrimSpace(a.ModelName)
			if modelName == "" {
				modelName = strings.TrimSpace(logItem.ActualModelName)
			}
			if modelName == "" {
				modelName = strings.TrimSpace(logItem.RequestModelName)
			}
			if !inScope(a.ChannelID, modelName) {
				continue
			}
			key := strconv.Itoa(a.ChannelID) + "\x00" + modelName
			row, ok := rows[key]
			if !ok {
				row = &analyticsChannelModelAggregateRow{ChannelID: a.ChannelID, ModelName: modelName}
				rows[key] = row
			}
			if a.Status == model.AttemptFailed {
				row.RequestFailed++
				continue
			}
			if a.Status == model.AttemptSuccess {
				row.RequestSuccess++
				// token/cost 仅在整体成功时计入该渠道（避免重复计入）。
				if success {
					row.InputTokens += int64(logItem.InputTokens)
					row.OutputTokens += int64(logItem.OutputTokens)
					row.TotalCost += logItem.Cost
				}
			}
			if row.ChannelName == "" {
				row.ChannelName = a.ChannelName
			}
		}
	}
	lock.Unlock()

	return rows, nil
}

func loadAnalyticsProviderRows(ctx context.Context, r model.AnalyticsRange) (map[int]*analyticsProviderAggregateRow, error) {
	startUnix := analyticsRangeStartUnix(r, stats.Now())
	rows := make(map[int]*analyticsProviderAggregateRow)

	keepEnabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return nil, err
	}

	if keepEnabled {
		var dbRows []analyticsProviderAggregateRow
		query := db.GetDB().WithContext(ctx).
			Model(&model.RelayLog{}).
			Select(`
				channel_id,
				channel_name,
				COALESCE(SUM(input_tokens), 0) AS input_tokens,
				COALESCE(SUM(output_tokens), 0) AS output_tokens,
				COALESCE(SUM(cost), 0) AS total_cost,
				COALESCE(SUM(CASE WHEN error = '' THEN 1 ELSE 0 END), 0) AS request_success,
				COALESCE(SUM(CASE WHEN error <> '' THEN 1 ELSE 0 END), 0) AS request_failed
			`).
			Group("channel_id, channel_name")
		if startUnix != nil {
			query = query.Where("time >= ?", *startUnix)
		}
		if err := query.Scan(&dbRows).Error; err != nil {
			return nil, err
		}
		for _, row := range dbRows {
			rowCopy := row
			rows[row.ChannelID] = &rowCopy
		}
	}

	cache, lock := relaylog.GetCacheAndLock()
	lock.Lock()
	for _, logItem := range cache {
		if startUnix != nil && logItem.Time < *startUnix {
			continue
		}
		row, ok := rows[logItem.ChannelId]
		if !ok {
			row = &analyticsProviderAggregateRow{
				ChannelID:   logItem.ChannelId,
				ChannelName: logItem.ChannelName,
			}
			rows[logItem.ChannelId] = row
		}
		row.InputTokens += int64(logItem.InputTokens)
		row.OutputTokens += int64(logItem.OutputTokens)
		row.TotalCost += logItem.Cost
		if logItem.Error == "" {
			row.RequestSuccess++
		} else {
			row.RequestFailed++
		}
		if row.ChannelName == "" {
			row.ChannelName = logItem.ChannelName
		}
	}
	lock.Unlock()

	return rows, nil
}

func loadAnalyticsModelRows(ctx context.Context, r model.AnalyticsRange) (map[string]*analyticsModelAggregateRow, error) {
	startUnix := analyticsRangeStartUnix(r, stats.Now())
	rows := make(map[string]*analyticsModelAggregateRow)

	keepEnabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return nil, err
	}

	if keepEnabled {
		var dbRows []analyticsModelAggregateRow
		modelExpr := "COALESCE(NULLIF(actual_model_name, ''), request_model_name)"
		query := db.GetDB().WithContext(ctx).
			Model(&model.RelayLog{}).
			Select(`
				` + modelExpr + ` AS model_name,
				COALESCE(SUM(input_tokens), 0) AS input_tokens,
				COALESCE(SUM(output_tokens), 0) AS output_tokens,
				COALESCE(SUM(cost), 0) AS total_cost,
				COALESCE(SUM(CASE WHEN error = '' THEN 1 ELSE 0 END), 0) AS request_success,
				COALESCE(SUM(CASE WHEN error <> '' THEN 1 ELSE 0 END), 0) AS request_failed
			`).
			Group(modelExpr)
		if startUnix != nil {
			query = query.Where("time >= ?", *startUnix)
		}
		if err := query.Scan(&dbRows).Error; err != nil {
			return nil, err
		}
		for _, row := range dbRows {
			modelName := strings.TrimSpace(row.ModelName)
			if modelName == "" {
				continue
			}
			rowCopy := row
			rowCopy.ModelName = modelName
			rows[modelName] = &rowCopy
		}
	}

	cache, lock := relaylog.GetCacheAndLock()
	lock.Lock()
	for _, logItem := range cache {
		if startUnix != nil && logItem.Time < *startUnix {
			continue
		}
		modelName := strings.TrimSpace(logItem.ActualModelName)
		if modelName == "" {
			modelName = strings.TrimSpace(logItem.RequestModelName)
		}
		if modelName == "" {
			continue
		}

		row, ok := rows[modelName]
		if !ok {
			row = &analyticsModelAggregateRow{ModelName: modelName}
			rows[modelName] = row
		}
		row.InputTokens += int64(logItem.InputTokens)
		row.OutputTokens += int64(logItem.OutputTokens)
		row.TotalCost += logItem.Cost
		if logItem.Error == "" {
			row.RequestSuccess++
		} else {
			row.RequestFailed++
		}
	}
	lock.Unlock()

	return rows, nil
}

func loadAnalyticsAPIKeyRows(ctx context.Context, r model.AnalyticsRange) (map[string]*analyticsAPIKeyAggregateRow, error) {
	startUnix := analyticsRangeStartUnix(r, stats.Now())
	rows := make(map[string]*analyticsAPIKeyAggregateRow)

	keepEnabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return nil, err
	}

	if keepEnabled {
		var dbRows []analyticsAPIKeyAggregateRow
		query := db.GetDB().WithContext(ctx).
			Model(&model.RelayLog{}).
			Select(`
				request_api_key_id AS api_key_id,
				request_api_key_name AS name,
				COALESCE(SUM(input_tokens), 0) AS input_tokens,
				COALESCE(SUM(output_tokens), 0) AS output_tokens,
				COALESCE(SUM(cost), 0) AS total_cost,
				COALESCE(SUM(CASE WHEN error = '' THEN 1 ELSE 0 END), 0) AS request_success,
				COALESCE(SUM(CASE WHEN error <> '' THEN 1 ELSE 0 END), 0) AS request_failed
			`).
			Group("request_api_key_id, request_api_key_name")
		if startUnix != nil {
			query = query.Where("time >= ?", *startUnix)
		}
		if err := query.Scan(&dbRows).Error; err != nil {
			return nil, err
		}
		for _, row := range dbRows {
			rowCopy := row
			rowCopy.Name = strings.TrimSpace(row.Name)
			rows[makeAnalyticsAPIKeyAggregateKey(row.APIKeyID, rowCopy.Name)] = &rowCopy
		}
	}

	cache, lock := relaylog.GetCacheAndLock()
	lock.Lock()
	for _, logItem := range cache {
		if startUnix != nil && logItem.Time < *startUnix {
			continue
		}
		apiKeyID := logItem.RequestAPIKeyID
		keyName := strings.TrimSpace(logItem.RequestAPIKeyName)
		aggregateKey := makeAnalyticsAPIKeyAggregateKey(apiKeyID, keyName)
		row, ok := rows[aggregateKey]
		if !ok {
			row = &analyticsAPIKeyAggregateRow{
				APIKeyID: apiKeyID,
				Name:     keyName,
			}
			rows[aggregateKey] = row
		}
		row.InputTokens += int64(logItem.InputTokens)
		row.OutputTokens += int64(logItem.OutputTokens)
		row.TotalCost += logItem.Cost
		if logItem.Error == "" {
			row.RequestSuccess++
		} else {
			row.RequestFailed++
		}
		if row.Name == "" {
			row.Name = keyName
		}
	}
	lock.Unlock()

	return rows, nil
}

func makeAnalyticsAPIKeyAggregateKey(apiKeyID int, name string) string {
	if apiKeyID > 0 {
		return "id:" + strconv.Itoa(apiKeyID)
	}
	return "name:" + strings.TrimSpace(name)
}

func loadAnalyticsFailureRows(ctx context.Context, since time.Time) (map[string]*analyticsFailureAggregateRow, error) {
	startUnix := since.Unix()
	rows := make(map[string]*analyticsFailureAggregateRow)

	keepEnabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return nil, err
	}

	if keepEnabled {
		// 优先从 relay_log_attempts 聚合失败尝试，使"渠道A 失败→重试到B 成功"
		// 的请求中渠道A 的失败也被计入（issue #67）。join relay_logs 取
		// request_model_name（分组名）以保留与 GroupItem.ModelName 的匹配维度。
		conn := db.GetLogDB()
		if conn != nil && connHasRelayLogAttempts(conn) {
			var dbRows []analyticsFailureAggregateRow
			query := conn.WithContext(ctx).
				Table("relay_log_attempts AS a").
				Select(`
					a.channel_id,
					l.request_model_name,
					a.model_name AS actual_model_name,
					COUNT(*) AS failure_count,
					MAX(a.time) AS last_failure_at
				`).
				Joins("JOIN relay_logs AS l ON l.id = a.relay_log_id").
				Where("a.status = ?", string(model.AttemptFailed)).
				Where("a.time >= ?", startUnix).
				Group("a.channel_id, l.request_model_name, a.model_name")
			if err := query.Scan(&dbRows).Error; err != nil {
				return nil, err
			}
			for _, row := range dbRows {
				key := makeAnalyticsFailureKey(row.ChannelID, row.ActualModelName, row.RequestModelName)
				rowCopy := row
				rows[key] = &rowCopy
			}
		} else {
			// 回退：relay_log_attempts 表不存在（旧库未迁移）时用顶层 relay_logs 列。
			conn := db.GetDB()
			if conn != nil {
				var dbRows []analyticsFailureAggregateRow
				query := conn.WithContext(ctx).
					Model(&model.RelayLog{}).
					Select(`
						channel_id,
						request_model_name,
						actual_model_name,
						COUNT(*) AS failure_count,
						MAX(time) AS last_failure_at
					`).
					Where("error <> ''").
					Where("time >= ?", startUnix).
					Group("channel_id, request_model_name, actual_model_name")
				if err := query.Scan(&dbRows).Error; err != nil {
					return nil, err
				}
				for _, row := range dbRows {
					key := makeAnalyticsFailureKey(row.ChannelID, row.ActualModelName, row.RequestModelName)
					rowCopy := row
					rows[key] = &rowCopy
				}
			}
		}
	}

	// 内存缓存中尚未落库的失败尝试同样按尝试维度聚合。
	cache, lock := relaylog.GetCacheAndLock()
	lock.Lock()
	for _, logItem := range cache {
		if logItem.Time < startUnix {
			continue
		}
		// 整体失败：用顶层渠道记一次（与历史行为一致）。
		if logItem.Error != "" {
			key := makeAnalyticsFailureKey(logItem.ChannelId, logItem.ActualModelName, logItem.RequestModelName)
			row, ok := rows[key]
			if !ok {
				row = &analyticsFailureAggregateRow{
					ChannelID:        logItem.ChannelId,
					RequestModelName: logItem.RequestModelName,
					ActualModelName:  logItem.ActualModelName,
				}
				rows[key] = row
			}
			row.FailureCount++
			if logItem.Time > row.LastFailureAt {
				row.LastFailureAt = logItem.Time
			}
			continue
		}
		// 整体成功但含失败尝试：把每个失败尝试计入对应渠道（issue #67 关键修复）。
		for _, a := range logItem.Attempts {
			if a.Status != model.AttemptFailed || a.ChannelID == 0 {
				continue
			}
			key := makeAnalyticsFailureKey(a.ChannelID, a.ModelName, logItem.RequestModelName)
			row, ok := rows[key]
			if !ok {
				row = &analyticsFailureAggregateRow{
					ChannelID:        a.ChannelID,
					RequestModelName: logItem.RequestModelName,
					ActualModelName:  a.ModelName,
				}
				rows[key] = row
			}
			row.FailureCount++
			if logItem.Time > row.LastFailureAt {
				row.LastFailureAt = logItem.Time
			}
		}
	}
	lock.Unlock()

	return rows, nil
}

// connHasRelayLogAttempts 报告连接上是否已存在 relay_log_attempts 表（迁移后才有）。
// 用于在 DB 与 LogDB 分离、或旧库尚未迁移时优雅回退到顶层列聚合。
func connHasRelayLogAttempts(conn *gorm.DB) bool {
	if conn == nil || conn.Migrator() == nil {
		return false
	}
	return conn.Migrator().HasTable(&model.RelayLogAttempt{})
}

func analyticsRangeStartUnix(r model.AnalyticsRange, now time.Time) *int64 {
	startDate := analyticsStartTime(r, now)
	if startDate == nil {
		return nil
	}
	unix := startDate.Unix()
	return &unix
}

func analyticsStartDate(r model.AnalyticsRange, now time.Time) string {
	start := analyticsStartTime(r, now)
	if start == nil {
		return ""
	}
	return start.Format("20060102")
}

func analyticsStartTime(r model.AnalyticsRange, now time.Time) *time.Time {
	location := now.Location()
	// dayStart uses now.Location() which reflects the container TZ, not stats offset.
	// Future: if stats_timezone is promoted to IANA, consider whether analytics
	// should also switch or remain on server local time.
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)

	switch r {
	case model.AnalyticsRange1D:
		return &dayStart
	case model.AnalyticsRange7D:
		start := dayStart.AddDate(0, 0, -6)
		return &start
	case model.AnalyticsRange30D:
		start := dayStart.AddDate(0, 0, -29)
		return &start
	case model.AnalyticsRange90D:
		start := dayStart.AddDate(0, 0, -89)
		return &start
	case model.AnalyticsRangeYTD:
		start := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, location)
		return &start
	case model.AnalyticsRangeAll:
		return nil
	default:
		start := dayStart.AddDate(0, 0, -6)
		return &start
	}
}

// AnalyticsLatencyDistributionGet returns latency and FTUT distribution for the given range.
func AnalyticsLatencyDistributionGet(ctx context.Context, r model.AnalyticsRange) (*model.LatencyDistribution, error) {
	return loadLatencyDistribution(ctx, r)
}

func loadLatencyDistribution(ctx context.Context, r model.AnalyticsRange) (*model.LatencyDistribution, error) {
	startUnix := analyticsRangeStartUnix(r, stats.Now())
	result := &model.LatencyDistribution{}

	keepEnabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return nil, err
	}

	// Collect all latency values from DB + in-memory cache
	var latencies []float64
	var ftuts []float64
	var totalUseTime int64
	var totalFtut int64
	var totalCount int64

	// Histogram accumulators
	var hLt100, h100to500, h500to1k, h1kto5k, hGt5k int64

	if keepEnabled {
		type latencyRow struct {
			UseTime int `json:"use_time"`
			Ftut    int `json:"ftut"`
		}
		var dbRows []latencyRow
		query := db.GetDB().WithContext(ctx).
			Model(&model.RelayLog{}).
			Select("use_time, ftut")
		if startUnix != nil {
			query = query.Where("time >= ?", *startUnix)
		}
		if err := query.Find(&dbRows).Error; err != nil {
			return nil, err
		}
		for _, row := range dbRows {
			if row.UseTime > 0 {
				latencies = append(latencies, float64(row.UseTime))
				totalUseTime += int64(row.UseTime)
				totalCount++
				switch {
				case row.UseTime < 100:
					hLt100++
				case row.UseTime < 500:
					h100to500++
				case row.UseTime < 1000:
					h500to1k++
				case row.UseTime < 5000:
					h1kto5k++
				default:
					hGt5k++
				}
			}
			if row.Ftut > 0 {
				ftuts = append(ftuts, float64(row.Ftut))
				totalFtut += int64(row.Ftut)
			}
		}
	}

	// Merge in-memory cache
	cache, lock := relaylog.GetCacheAndLock()
	lock.Lock()
	for _, logItem := range cache {
		if startUnix != nil && logItem.Time < *startUnix {
			continue
		}
		if logItem.UseTime > 0 {
			latencies = append(latencies, float64(logItem.UseTime))
			totalUseTime += int64(logItem.UseTime)
			totalCount++
			switch {
			case logItem.UseTime < 100:
				hLt100++
			case logItem.UseTime < 500:
				h100to500++
			case logItem.UseTime < 1000:
				h500to1k++
			case logItem.UseTime < 5000:
				h1kto5k++
			default:
				hGt5k++
			}
		}
		if logItem.Ftut > 0 {
			ftuts = append(ftuts, float64(logItem.Ftut))
			totalFtut += int64(logItem.Ftut)
		}
	}
	lock.Unlock()

	result.TotalRequests = totalCount
	if totalCount > 0 {
		result.AvgMs = totalUseTime / totalCount
	}

	// Compute percentiles from sorted samples
	sort.Float64s(latencies)
	result.P50Ms = int64(percentileFromSorted(latencies, 0.50))
	result.P95Ms = int64(percentileFromSorted(latencies, 0.95))
	result.P99Ms = int64(percentileFromSorted(latencies, 0.99))

	sort.Float64s(ftuts)
	if len(ftuts) > 0 {
		result.FtutAvgMs = totalFtut / int64(len(ftuts))
		result.FtutP50Ms = int64(percentileFromSorted(ftuts, 0.50))
		result.FtutP95Ms = int64(percentileFromSorted(ftuts, 0.95))
		result.FtutP99Ms = int64(percentileFromSorted(ftuts, 0.99))
	}

	result.Buckets = []model.HistogramBucket{
		{Label: "<100ms", Count: hLt100},
		{Label: "100-500ms", Count: h100to500},
		{Label: "500ms-1s", Count: h500to1k},
		{Label: "1-5s", Count: h1kto5k},
		{Label: ">5s", Count: hGt5k},
	}

	return result, nil
}

func percentileFromSorted(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func splitAnalyticsChannelModels(channel model.Channel) []string {
	parts := strings.Split(channel.Model, ",")
	if strings.TrimSpace(channel.CustomModel) != "" {
		parts = append(parts, strings.Split(channel.CustomModel, ",")...)
	}

	seen := make(map[string]struct{}, len(parts))
	models := make([]string, 0, len(parts))
	for _, part := range parts {
		modelName := strings.TrimSpace(part)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		models = append(models, modelName)
	}
	return models
}

func makeAnalyticsFailureKey(channelID int, actualModelName, requestModelName string) string {
	actualModelName = strings.TrimSpace(actualModelName)
	if actualModelName == "" {
		actualModelName = strings.TrimSpace(requestModelName)
	}
	return strings.Join([]string{
		strconv.Itoa(channelID),
		actualModelName,
		strings.TrimSpace(requestModelName),
	}, "\x00")
}
