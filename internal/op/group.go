package op

import (
	"context"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/group"
)

// groupCache is retained for backward compatibility (used by tests).
var groupCache = group.GetCache()

// groupMap is retained for backward compatibility (used by tests).
var groupMap = group.GetNameMap()

// groupRegexMatchersByEndpoint is retained for backward compatibility.
var groupRegexMatchersByEndpoint map[string][]compiledGroupMatcher

// Backward-compatible references for tests
var groupRegexMatchersLock = group.GetRegexMatchersLock()
var groupRegexMatchTimeout = group.GetRegexMatchTimeout()
type compiledGroupMatcher = group.CompiledGroupMatcher

func GroupList(ctx context.Context) ([]model.Group, error) { return group.GroupList(ctx) }

func GroupListModel(ctx context.Context) ([]string, error) { return group.GroupListModel(ctx) }

func GroupListModelByEndpoint(endpointType string, ctx context.Context) ([]string, error) {
	return group.GroupListModelByEndpoint(endpointType, ctx)
}

func GroupListModelCapabilities(ctx context.Context) ([]model.ModelCapability, error) {
	return group.GroupListModelCapabilities(ctx)
}

func GroupGet(id int, ctx context.Context) (*model.Group, error) { return group.GroupGet(id, ctx) }

func GroupGetEnabledMapByEndpoint(endpointType string, name string, ctx context.Context) (model.Group, error) {
	return group.GroupGetEnabledMapByEndpoint(endpointType, name, ctx)
}

func GroupGetEnabledMap(name string, ctx context.Context) (model.Group, error) {
	return group.GroupGetEnabledMap(name, ctx)
}

func GroupCreate(g *model.Group, ctx context.Context) error { return group.GroupCreate(g, ctx) }

func GroupUpdate(req *model.GroupUpdateRequest, ctx context.Context) (*model.Group, error) {
	return group.GroupUpdate(req, ctx)
}

func GroupDel(id int, ctx context.Context) error { return group.GroupDel(id, ctx) }

func GroupDelAll(ctx context.Context) (int64, error) { return group.GroupDelAll(ctx) }

func GroupItemAdd(item *model.GroupItem, ctx context.Context) error { return group.GroupItemAdd(item, ctx) }

func GroupItemBatchAdd(groupID int, items []model.GroupIDAndLLMName, ctx context.Context) error {
	return group.GroupItemBatchAdd(groupID, items, ctx)
}

func GroupItemUpdate(item *model.GroupItem, ctx context.Context) error { return group.GroupItemUpdate(item, ctx) }

func GroupItemDel(id int, ctx context.Context) error { return group.GroupItemDel(id, ctx) }

func GroupItemBatchDelByChannelAndModels(keys []model.GroupIDAndLLMName, ctx context.Context) error {
	return group.GroupItemBatchDelByChannelAndModels(keys, ctx)
}

func GroupItemList(groupID int, ctx context.Context) ([]model.GroupItem, error) {
	return group.GroupItemList(groupID, ctx)
}

// AutoGroupModels and NormalizeModelIdentity are still defined in auto_group.go

// groupRefreshCacheByID is called from within the op package
func groupRefreshCacheByID(id int, ctx context.Context) error { return group.RefreshCacheByID(id, ctx) }

// groupRefreshCacheByIDs is called from within the op package
func groupRefreshCacheByIDs(ids []int, ctx context.Context) error { return group.RefreshCacheByIDs(ids, ctx) }

// Backward-compatible function for tests
func rebuildGroupIndexesFromCache() {
	group.RebuildIndexes()
	groupRegexMatchersByEndpoint = group.GetRegexMatchers()
}

// Backward-compatible function for tests
func normalizeGroupItems(items []model.GroupItem) []model.GroupItem {
	return group.NormalizeItems(items)
}

// groupRefreshCache is called from cache.go (same package)
func groupRefreshCache(ctx context.Context) error { return group.RefreshAllCache(ctx) }

func init() {
	groupRegexMatchersByEndpoint = group.GetRegexMatchers()
}


