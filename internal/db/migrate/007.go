package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 7,
		Up:      ensureDefaultChannelGroup,
	})
}

func ensureDefaultChannelGroup(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.ChannelGroup{}) || !db.Migrator().HasTable(&model.Channel{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&model.Channel{}, "GroupID") {
		if err := db.Migrator().AddColumn(&model.Channel{}, "GroupID"); err != nil {
			return fmt.Errorf("failed to add channels.group_id: %w", err)
		}
	}

	var defaultGroup model.ChannelGroup
	err := db.Where("is_default = ?", true).First(&defaultGroup).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return fmt.Errorf("failed to query default channel group: %w", err)
		}
		defaultGroup = model.ChannelGroup{
			Name:      model.DefaultChannelGroupName,
			IsDefault: true,
		}
		if createErr := db.Create(&defaultGroup).Error; createErr != nil {
			return fmt.Errorf("failed to create default channel group: %w", createErr)
		}
	}

	if err := db.Model(&model.ChannelGroup{}).
		Where("id <> ? AND is_default = ?", defaultGroup.ID, true).
		Update("is_default", false).Error; err != nil {
		return fmt.Errorf("failed to normalize default channel group flag: %w", err)
	}

	if err := db.Model(&model.Channel{}).
		Where("group_id IS NULL OR group_id = 0").
		Update("group_id", defaultGroup.ID).Error; err != nil {
		return fmt.Errorf("failed to backfill channels.group_id: %w", err)
	}

	return nil
}
