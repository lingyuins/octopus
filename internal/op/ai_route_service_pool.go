package op

import (
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/airoute"
)

// normalizeAIRouteServiceName is retained for backward compatibility (used by ops.go and airoute tests).
func normalizeAIRouteServiceName(cfg model.AIRouteServiceConfig, ordinal int) string {
	return airoute.NormalizeAIRouteServiceName(cfg, ordinal)
}
