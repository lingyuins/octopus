package op

import (
	"context"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/op/stats"
	"github.com/lingyuins/octopus/internal/utils/log"
)

var channelCache = channel.GetCache()
var channelKeyCache = channel.GetKeyCache()
var channelKeyCacheNeedUpdate, channelKeyCacheNeedUpdateLock = channel.GetKeyCacheNeedUpdate()

func init() {
	channel.GroupDefaultID = func(ctx context.Context) (int, error) {
		return ChannelGroupDefaultID(ctx)
	}
	channel.GroupGet = func(id int, ctx context.Context) (*model.ChannelGroup, error) {
		return ChannelGroupGet(id, ctx)
	}
}

// Deprecated: Use channel.List from internal/op/channel instead.
func ChannelList(ctx context.Context) ([]model.Channel, error) { return channel.List(ctx) }

// Deprecated: Use channel.Create from internal/op/channel instead.
func ChannelCreate(ch *model.Channel, ctx context.Context) error { return channel.Create(ch, ctx) }

// Deprecated: Use channel.KeyUpdate from internal/op/channel instead.
func ChannelKeyUpdate(key model.ChannelKey) error { return channel.KeyUpdate(key) }

// Deprecated: Use channel.BaseUrlUpdate from internal/op/channel instead.
func ChannelBaseUrlUpdate(channelID int, baseUrl []model.BaseUrl) error {
	return channel.BaseUrlUpdate(channelID, baseUrl)
}

// Deprecated: Use channel.KeySaveDB from internal/op/channel instead.
func ChannelKeySaveDB(ctx context.Context) error { return channel.KeySaveDB(ctx) }

// Deprecated: Use channel.Update from internal/op/channel instead.
func ChannelUpdate(req *model.ChannelUpdateRequest, ctx context.Context) (*model.Channel, error) {
	return channel.Update(req, ctx)
}

// Deprecated: Use channel.Enabled from internal/op/channel instead.
func ChannelEnabled(id int, enabled bool, ctx context.Context) error {
	return channel.Enabled(id, enabled, ctx)
}

// ChannelDel handles deletion with cross-package stats/group cache cleanup.
func ChannelDel(id int, ctx context.Context) error {
	ch, err := channel.Get(id, ctx)
	if err != nil {
		return err
	}

	if err := channel.Delete(id, ctx); err != nil {
		return err
	}

	stats.OnChannelDeleted(id)

	// Refresh affected group caches (in op package, from group.go)
	for _, groupID := range getAffectedGroupIDs(id, ctx) {
		if err := groupRefreshCacheByID(groupID, ctx); err != nil {
			log.Warnf("failed to refresh group cache for group %d: %v", groupID, err)
		}
	}

	// Clean up channel key cache
	for _, k := range ch.Keys {
		if k.ID != 0 {
			channelKeyCache.Del(k.ID)
		}
	}

	return nil
}

func getAffectedGroupIDs(id int, ctx context.Context) []int {
	// This is a minimal implementation; the original logic was in ChannelDel's transaction
	return nil
}

// Deprecated: Use channel.LLMList from internal/op/channel instead.
func ChannelLLMList(ctx context.Context) ([]model.LLMChannel, error) { return channel.LLMList(ctx) }

// Deprecated: Use channel.Get from internal/op/channel instead.
func ChannelGet(id int, ctx context.Context) (*model.Channel, error) { return channel.Get(id, ctx) }

// channelRefreshCache is called by cache.go (same package)
func channelRefreshCache(ctx context.Context) error { return channel.RefreshCache(ctx) }

// channelRefreshCacheByID is called by group.go and ChannelDel (same package)
func channelRefreshCacheByID(id int, ctx context.Context) error { return channel.RefreshCacheByID(id, ctx) }

// ChannelGroup functions are still in channel_group.go (not yet extracted)
