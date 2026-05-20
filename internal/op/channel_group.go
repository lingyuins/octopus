package op

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/cache"
)

var channelGroupCache = cache.New[int, model.ChannelGroup](16)
var channelGroupMutationLock sync.Mutex

func ChannelGroupList(ctx context.Context) ([]model.ChannelGroup, error) {
	if channelGroupCache.Len() == 0 {
		if err := channelGroupRefreshCache(ctx); err != nil {
			return nil, err
		}
	}
	groups := make([]model.ChannelGroup, 0, channelGroupCache.Len())
	for _, group := range channelGroupCache.GetAll() {
		groups = append(groups, group)
	}
	sortChannelGroups(groups)
	return groups, nil
}

func ChannelGroupGet(id int, ctx context.Context) (*model.ChannelGroup, error) {
	if id <= 0 {
		return nil, fmt.Errorf("channel group not found")
	}
	if group, ok := channelGroupCache.Get(id); ok {
		return &group, nil
	}

	var group model.ChannelGroup
	if err := db.GetDB().WithContext(ctx).First(&group, id).Error; err != nil {
		return nil, fmt.Errorf("channel group not found")
	}
	channelGroupMutationLock.Lock()
	channelGroupCache.Set(group.ID, group)
	channelGroupMutationLock.Unlock()
	return &group, nil
}

func ChannelGroupCreate(name string, ctx context.Context) (*model.ChannelGroup, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, fmt.Errorf("channel group name is required")
	}

	group := &model.ChannelGroup{Name: trimmedName}
	if err := db.GetDB().WithContext(ctx).Create(group).Error; err != nil {
		return nil, err
	}

	channelGroupMutationLock.Lock()
	channelGroupCache.Set(group.ID, *group)
	channelGroupMutationLock.Unlock()
	return group, nil
}

func ChannelGroupUpdate(id int, name string, ctx context.Context) (*model.ChannelGroup, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, fmt.Errorf("channel group name is required")
	}

	group, err := ChannelGroupGet(id, ctx)
	if err != nil {
		return nil, err
	}

	if err := db.GetDB().WithContext(ctx).
		Model(&model.ChannelGroup{}).
		Where("id = ?", id).
		Update("name", trimmedName).Error; err != nil {
		return nil, err
	}

	group.Name = trimmedName
	if err := channelGroupRefreshCacheByID(group.ID, ctx); err != nil {
		return nil, err
	}
	updated, _ := channelGroupCache.Get(group.ID)
	return &updated, nil
}

func ChannelGroupDelete(id int, ctx context.Context) error {
	group, err := ChannelGroupGet(id, ctx)
	if err != nil {
		return err
	}
	if group.IsDefault {
		return fmt.Errorf("default channel group cannot be deleted")
	}

	var count int64
	if err := db.GetDB().WithContext(ctx).Model(&model.Channel{}).Where("group_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("channel group is not empty")
	}

	if err := db.GetDB().WithContext(ctx).Delete(&model.ChannelGroup{}, id).Error; err != nil {
		return err
	}

	channelGroupMutationLock.Lock()
	channelGroupCache.Del(id)
	channelGroupMutationLock.Unlock()
	return nil
}

func ChannelGroupDefaultID(ctx context.Context) (int, error) {
	for _, group := range channelGroupCache.GetAll() {
		if group.IsDefault {
			return group.ID, nil
		}
	}

	group, err := ensureDefaultChannelGroup(ctx)
	if err != nil {
		return 0, err
	}
	return group.ID, nil
}

func ensureDefaultChannelGroup(ctx context.Context) (model.ChannelGroup, error) {
	var group model.ChannelGroup
	if err := db.GetDB().WithContext(ctx).Where("is_default = ?", true).First(&group).Error; err == nil {
		channelGroupMutationLock.Lock()
		channelGroupCache.Set(group.ID, group)
		channelGroupMutationLock.Unlock()
		return group, nil
	}

	group = model.ChannelGroup{Name: model.DefaultChannelGroupName, IsDefault: true}
	if err := db.GetDB().WithContext(ctx).Create(&group).Error; err != nil {
		if err := db.GetDB().WithContext(ctx).Where("is_default = ?", true).First(&group).Error; err != nil {
			return model.ChannelGroup{}, fmt.Errorf("default channel group not found")
		}
	}

	channelGroupMutationLock.Lock()
	channelGroupCache.Set(group.ID, group)
	channelGroupMutationLock.Unlock()
	return group, nil
}

func channelGroupRefreshCache(ctx context.Context) error {
	var groups []model.ChannelGroup
	if err := db.GetDB().WithContext(ctx).Find(&groups).Error; err != nil {
		return err
	}

	channelGroupMutationLock.Lock()
	channelGroupCache.Clear()
	for _, group := range groups {
		channelGroupCache.Set(group.ID, group)
	}
	channelGroupMutationLock.Unlock()
	return nil
}

func channelGroupRefreshCacheByID(id int, ctx context.Context) error {
	var group model.ChannelGroup
	if err := db.GetDB().WithContext(ctx).First(&group, id).Error; err != nil {
		return err
	}

	channelGroupMutationLock.Lock()
	channelGroupCache.Set(group.ID, group)
	channelGroupMutationLock.Unlock()
	return nil
}

func sortChannelGroups(groups []model.ChannelGroup) {
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].IsDefault != groups[j].IsDefault {
			return groups[i].IsDefault
		}
		if groups[i].CreatedAt != groups[j].CreatedAt {
			return groups[i].CreatedAt < groups[j].CreatedAt
		}
		return groups[i].ID < groups[j].ID
	})
}

