package model

import (
	"fmt"
	"strings"
)

type AnalyticsRange string

const (
	AnalyticsRange1D  AnalyticsRange = "1d"
	AnalyticsRange7D  AnalyticsRange = "7d"
	AnalyticsRange30D AnalyticsRange = "30d"
	AnalyticsRange90D AnalyticsRange = "90d"
	AnalyticsRangeYTD AnalyticsRange = "ytd"
	AnalyticsRangeAll AnalyticsRange = "all"
)

func ParseAnalyticsRange(raw string) (AnalyticsRange, error) {
	value := AnalyticsRange(strings.ToLower(strings.TrimSpace(raw)))
	if value == "" {
		return AnalyticsRange7D, nil
	}

	switch value {
	case AnalyticsRange1D, AnalyticsRange7D, AnalyticsRange30D, AnalyticsRange90D, AnalyticsRangeYTD, AnalyticsRangeAll:
		return value, nil
	default:
		return "", fmt.Errorf("invalid analytics range: %s", raw)
	}
}

type AnalyticsMetrics struct {
	RequestCount int64   `json:"request_count"`
	TotalTokens  int64   `json:"total_tokens"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalCost    float64 `json:"total_cost"`
	SuccessRate  float64 `json:"success_rate"`
}

type AnalyticsOverview struct {
	AnalyticsMetrics
	ProviderCount int     `json:"provider_count"`
	APIKeyCount   int     `json:"api_key_count"`
	ModelCount    int     `json:"model_count"`
	FallbackRate  float64 `json:"fallback_rate"`
}

type AnalyticsEvaluationSummary struct {
	SemanticCache SemanticCacheEvaluationSummary `json:"semantic_cache"`
}

type SemanticCacheEvaluationSummary struct {
	Enabled           bool    `json:"enabled"`
	RuntimeEnabled    bool    `json:"runtime_enabled"`
	TTLSeconds        int     `json:"ttl_seconds"`
	Threshold         int     `json:"threshold"`
	MaxEntries        int     `json:"max_entries"`
	CurrentEntries    int     `json:"current_entries"`
	Hits              int64   `json:"hits"`
	Misses            int64   `json:"misses"`
	HitRate           float64 `json:"hit_rate"`
	UsageRate         float64 `json:"usage_rate"`
	EvaluatedRequests int64   `json:"evaluated_requests"`
	CacheHitResponses int64   `json:"cache_hit_responses"`
	CacheMissRequests int64   `json:"cache_miss_requests"`
	BypassedRequests  int64   `json:"bypassed_requests"`
	StoredResponses   int64   `json:"stored_responses"`
}

type AnalyticsProviderBreakdownItem struct {
	ChannelID   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Enabled     bool   `json:"enabled"`
	AnalyticsMetrics
}

type AnalyticsModelBreakdownItem struct {
	ModelName string `json:"model_name"`
	AnalyticsMetrics
}

type AnalyticsAPIKeyBreakdownItem struct {
	APIKeyID *int   `json:"api_key_id,omitempty"`
	Name     string `json:"name"`
	AnalyticsMetrics
}

// AnalyticsChannelModelItem 描述某个 (渠道,模型) 维度的统计，用于"渠道×模型"分析视图。
// 与 AnalyticsProviderBreakdownItem 的区别：这里是渠道与模型的交叉维度，且失败数
// 基于单次尝试（relay_log_attempts）聚合，使"渠道A 失败→重试到B 成功"的请求中
// 渠道A 的失败也能反映到 A 的成功率上（issue #67）。
type AnalyticsChannelModelItem struct {
	ChannelID   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	ModelName   string `json:"model_name"`
	Enabled     bool   `json:"enabled"`
	AnalyticsMetrics
}

// AutoStrategySnapshotItem 是 Auto 策略运行态某个 (渠道,模型) 维度的实时快照，
// 反映滑动窗口内的成功率/样本数/延迟。供分组健康中的"Auto 实时表现"展示（issue #67）。
type AutoStrategySnapshotItem struct {
	ChannelID     int     `json:"channel_id"`
	ChannelName   string  `json:"channel_name"`
	Enabled       bool    `json:"enabled"`
	ModelName     string  `json:"model_name"`
	SuccessRate   float64 `json:"success_rate"` // 0-100
	SampleCount   int     `json:"sample_count"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	LastActiveAt  int64   `json:"last_active_at"`  // unix 秒，0 表示无活动
	MinSamplesMet bool    `json:"min_samples_met"` // 样本数是否达到最小阈值
}

type AnalyticsUtilization struct {
	ProviderBreakdown []AnalyticsProviderBreakdownItem `json:"provider_breakdown"`
	ModelBreakdown    []AnalyticsModelBreakdownItem    `json:"model_breakdown"`
	APIKeyBreakdown   []AnalyticsAPIKeyBreakdownItem   `json:"apikey_breakdown"`
}

type AnalyticsGroupHealthItem struct {
	GroupID           int                  `json:"group_id"`
	GroupName         string               `json:"group_name"`
	EndpointType      string               `json:"endpoint_type"`
	ItemCount         int                  `json:"item_count"`
	EnabledItemCount  int                  `json:"enabled_item_count"`
	DisabledItemCount int                  `json:"disabled_item_count"`
	FailureCount      int64                `json:"failure_count"`
	LastFailureAt     int64                `json:"last_failure_at"`
	HealthScore       int                  `json:"health_score"`
	Status            string               `json:"status"`
	FailingChannels   []FailingChannelItem `json:"failing_channels"`
	Mode              int                  `json:"mode"`
}

// FailingChannelItem 描述组内某个 (渠道,模型) 维度的失败情况，供分组健康下钻展示。
type FailingChannelItem struct {
	ChannelID     int    `json:"channel_id"`
	ChannelName   string `json:"channel_name"`
	ModelName     string `json:"model_name"`
	FailureCount  int64  `json:"failure_count"`
	LastFailureAt int64  `json:"last_failure_at"`
}

// LatencyDistribution holds latency and FTUT distribution data for a time range.
type LatencyDistribution struct {
	TotalRequests int64             `json:"total_requests"`
	AvgMs         int64             `json:"avg_ms"`
	P50Ms         int64             `json:"p50_ms"`
	P95Ms         int64             `json:"p95_ms"`
	P99Ms         int64             `json:"p99_ms"`
	FtutAvgMs     int64             `json:"ftut_avg_ms"`
	FtutP50Ms     int64             `json:"ftut_p50_ms"`
	FtutP95Ms     int64             `json:"ftut_p95_ms"`
	FtutP99Ms     int64             `json:"ftut_p99_ms"`
	Buckets       []HistogramBucket `json:"buckets"`
}

// HistogramBucket is a single bucket in a latency histogram.
type HistogramBucket struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}
