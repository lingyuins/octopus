package relay

import (
	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/stats"
)

func updateChannelSuccessStats(channelID int, waitTimeMs int64, metrics dbmodel.StatsMetrics) {
	stats.ChannelUpdate(channelID, dbmodel.StatsMetrics{
		WaitTime:       waitTimeMs,
		InputToken:     metrics.InputToken,
		OutputToken:    metrics.OutputToken,
		InputCost:      metrics.InputCost,
		OutputCost:     metrics.OutputCost,
		RequestSuccess: 1,
	})
}
