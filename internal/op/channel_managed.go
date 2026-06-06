package op

import (
	"context"
	"fmt"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/channel"
)

// ChannelEnabledManaged enables/disables a channel bypassing the managed-check guard.
func ChannelEnabledManaged(id int, enabled bool, ctx context.Context) error {
	result := db.GetDB().WithContext(ctx).Model(&model.Channel{}).Where("id = ?", id).Update("enabled", enabled)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("channel not found")
	}
	return nil
}

// ChannelDelManaged deletes a channel bypassing the managed-check guard.
func ChannelDelManaged(id int, ctx context.Context) error {
	if _, err := channel.Get(id, ctx); err != nil {
		return fmt.Errorf("channel not found")
	}
	return ChannelDel(id, ctx)
}

// ChannelGetByName finds a channel by name.
func ChannelGetByName(name string, ctx context.Context) (*model.Channel, error) {
	var channel model.Channel
	if err := db.GetDB().WithContext(ctx).Where("name = ?", name).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// GroupRefreshCacheByIDs refreshes the group cache for the given IDs.
func GroupRefreshCacheByIDs(ids []int, ctx context.Context) error {
	return groupRefreshCacheByIDs(ids, ctx)
}
