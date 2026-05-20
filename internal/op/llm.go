package op

import (
	"context"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/llm"
	"github.com/lingyuins/octopus/internal/op/stats"
)

// llmModelCache is retained for backward compatibility (used by llm_test.go).
var llmModelCache = llm.GetCache()

func LLMList(ctx context.Context) ([]model.LLMInfo, error) {
	statsByName := stats.ModelMetricsByName()
	return llm.ListWithStats(ctx, statsByName)
}

// Deprecated: Use llm.Update from internal/op/llm instead.
func LLMUpdate(m model.LLMInfo, ctx context.Context) error { return llm.Update(m, ctx) }

// Deprecated: Use llm.Delete from internal/op/llm instead.
func LLMDelete(name string, ctx context.Context) error { return llm.Delete(name, ctx) }

// Deprecated: Use llm.BatchDelete from internal/op/llm instead.
func LLMBatchDelete(names []string, ctx context.Context) error { return llm.BatchDelete(names, ctx) }

// Deprecated: Use llm.Create from internal/op/llm instead.
func LLMCreate(m model.LLMInfo, ctx context.Context) error { return llm.Create(m, ctx) }

// Deprecated: Use llm.BatchCreate from internal/op/llm instead.
func LLMBatchCreate(infos []model.LLMInfo, ctx context.Context) error { return llm.BatchCreate(infos, ctx) }

// Deprecated: Use llm.BatchUpdate from internal/op/llm instead.
func LLMBatchUpdate(infos []model.LLMInfo, ctx context.Context) error { return llm.BatchUpdate(infos, ctx) }

// Deprecated: Use llm.Get from internal/op/llm instead.
func LLMGet(name string) (model.LLMPrice, error) { return llm.Get(name) }

// llmRefreshCache is called by cache.go (same package)
func llmRefreshCache(ctx context.Context) error { return llm.RefreshCache(ctx) }
