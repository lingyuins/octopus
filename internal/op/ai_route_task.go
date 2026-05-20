package op

import (
	"context"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/airoute"
)

// Deprecated: Use airoute.DefaultAIRouteTaskInterruptedMessage instead.
const DefaultAIRouteTaskInterruptedMessage = airoute.DefaultAIRouteTaskInterruptedMessage

// Deprecated: Use airoute.AIRouteTaskCreate from internal/op/airoute instead.
func AIRouteTaskCreate(ctx context.Context, progress model.GenerateAIRouteProgress) error {
	return airoute.AIRouteTaskCreate(ctx, progress)
}

// Deprecated: Use airoute.AIRouteTaskSaveProgress from internal/op/airoute instead.
func AIRouteTaskSaveProgress(ctx context.Context, progress model.GenerateAIRouteProgress) error {
	return airoute.AIRouteTaskSaveProgress(ctx, progress)
}

// Deprecated: Use airoute.AIRouteTaskGet from internal/op/airoute instead.
func AIRouteTaskGet(ctx context.Context, id string) (*model.GenerateAIRouteProgress, error) {
	return airoute.AIRouteTaskGet(ctx, id)
}

// Deprecated: Use airoute.AIRouteTaskFindActive from internal/op/airoute instead.
func AIRouteTaskFindActive(ctx context.Context, scope model.AIRouteScope, groupID int) (*model.GenerateAIRouteProgress, error) {
	return airoute.AIRouteTaskFindActive(ctx, scope, groupID)
}

// Deprecated: Use airoute.AIRouteTaskMarkActiveInterrupted from internal/op/airoute instead.
func AIRouteTaskMarkActiveInterrupted(ctx context.Context, message string) (int64, error) {
	return airoute.AIRouteTaskMarkActiveInterrupted(ctx, message)
}
