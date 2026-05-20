package op

import (
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/navorder"
	"github.com/lingyuins/octopus/internal/utils/semantic_cache"
)

// Deprecated: Use navorder.NormalizeNavOrder from internal/op/navorder instead.
func NormalizeNavOrder(raw string, defaults []string) []string {
	return navorder.NormalizeNavOrder(raw, defaults)
}

func buildSemanticCacheEvaluationSummary(
	enabled bool,
	runtimeEnabled bool,
	ttlSeconds int,
	threshold int,
	maxEntries int,
	currentEntries int,
	hits int64,
	misses int64,
	stats semantic_cache.RuntimeStats,
) model.SemanticCacheEvaluationSummary {
	return navorder.BuildSemanticCacheEvaluationSummary(enabled, runtimeEnabled, ttlSeconds, threshold, maxEntries, currentEntries, hits, misses, stats)
}
