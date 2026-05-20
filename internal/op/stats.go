package op

import (
	"context"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/stats"
)

// Deprecated: Use stats.SaveDBTask from internal/op/stats instead.
func StatsSaveDBTask() { stats.SaveDBTask() }

// Deprecated: Use stats.SaveDB from internal/op/stats instead.
func StatsSaveDB(ctx context.Context) error { return stats.SaveDB(ctx) }

// Deprecated: Use stats.DailyUpdate from internal/op/stats instead.
func StatsDailyUpdate(ctx context.Context, metrics model.StatsMetrics) error {
	return stats.DailyUpdate(ctx, metrics)
}

// Deprecated: Use stats.TotalUpdate from internal/op/stats instead.
func StatsTotalUpdate(metrics model.StatsMetrics) error { return stats.TotalUpdate(metrics) }

// Deprecated: Use stats.ChannelUpdate from internal/op/stats instead.
func StatsChannelUpdate(channelID int, metrics model.StatsMetrics) error {
	return stats.ChannelUpdate(channelID, metrics)
}

// Deprecated: Use stats.HourlyUpdate from internal/op/stats instead.
func StatsHourlyUpdate(metrics model.StatsMetrics) error { return stats.HourlyUpdate(metrics) }

// Deprecated: Use stats.ModelUpdate from internal/op/stats instead.
func StatsModelUpdate(s model.StatsModel) error { return stats.ModelUpdate(s) }

// Deprecated: Use stats.ModelList from internal/op/stats instead.
func StatsModelList() []model.StatsModel { return stats.ModelList() }

// Deprecated: Use stats.ModelRecord from internal/op/stats instead.
func StatsModelRecord(channelID int, modelName string, metrics model.StatsMetrics) error {
	return stats.ModelRecord(channelID, modelName, metrics)
}

// Deprecated: Use stats.APIKeyUpdate from internal/op/stats instead.
func StatsAPIKeyUpdate(apiKeyID int, metrics model.StatsMetrics) error {
	return stats.APIKeyUpdate(apiKeyID, metrics)
}

// Deprecated: Use stats.ChannelDel from internal/op/stats instead.
func StatsChannelDel(id int) error { return stats.ChannelDel(id) }

// Deprecated: Use stats.APIKeyDel from internal/op/stats instead.
func StatsAPIKeyDel(id int) error { return stats.APIKeyDel(id) }

// Deprecated: Use stats.TotalGet from internal/op/stats instead.
func StatsTotalGet() model.StatsTotal { return stats.TotalGet() }

// Deprecated: Use stats.TodayGet from internal/op/stats instead.
func StatsTodayGet() model.StatsDaily { return stats.TodayGet() }

// Deprecated: Use stats.ChannelGet from internal/op/stats instead.
func StatsChannelGet(id int) model.StatsChannel { return stats.ChannelGet(id) }

// Deprecated: Use stats.APIKeyGet from internal/op/stats instead.
func StatsAPIKeyGet(id int) model.StatsAPIKey { return stats.APIKeyGet(id) }

// Deprecated: Use stats.APIKeyList from internal/op/stats instead.
func StatsAPIKeyList() []model.StatsAPIKey { return stats.APIKeyList() }

// Deprecated: Use stats.ChannelList from internal/op/stats instead.
func StatsChannelList() []model.StatsChannel { return stats.ChannelList() }

// Deprecated: Use stats.HourlyGet from internal/op/stats instead.
func StatsHourlyGet() []model.StatsHourly { return stats.HourlyGet() }

// Deprecated: Use stats.GetDaily from internal/op/stats instead.
func StatsGetDaily(ctx context.Context) ([]model.StatsDaily, error) { return stats.GetDaily(ctx) }

// statsRefreshCache is called by cache.go (same package)
func statsRefreshCache(ctx context.Context) error { return stats.RefreshCache(ctx) }
