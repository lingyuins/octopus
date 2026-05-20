package op

import (
	"context"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/ops"
)

// Deprecated: Use ops.OpsCacheStatusGet from internal/op/ops instead.
func OpsCacheStatusGet(ctx context.Context) (*model.OpsCacheStatus, error) {
	return ops.OpsCacheStatusGet(ctx)
}

// Deprecated: Use ops.RefreshSemanticCacheRuntime from internal/op/ops instead.
func RefreshSemanticCacheRuntime() error {
	return ops.RefreshSemanticCacheRuntime()
}

// Deprecated: Use ops.OpsQuotaSummaryGet from internal/op/ops instead.
func OpsQuotaSummaryGet(ctx context.Context) (*model.OpsQuotaSummary, error) {
	return ops.OpsQuotaSummaryGet(ctx)
}

// Deprecated: Use ops.OpsHealthStatusGet from internal/op/ops instead.
func OpsHealthStatusGet(ctx context.Context) (*model.OpsHealthStatus, error) {
	return ops.OpsHealthStatusGet(ctx)
}

// Deprecated: Use ops.OpsSystemSummaryGet from internal/op/ops instead.
func OpsSystemSummaryGet(ctx context.Context) (*model.OpsSystemSummary, error) {
	return ops.OpsSystemSummaryGet(ctx)
}

// Deprecated: Use ops.TelemetrySummaryGet from internal/op/ops instead.
func TelemetrySummaryGet(ctx context.Context) (*model.OpsTelemetrySummary, error) {
	return ops.TelemetrySummaryGet(ctx)
}
