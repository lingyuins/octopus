package channel

import (
	"context"
	"fmt"
	"sync"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/cache"
	"github.com/lingyuins/octopus/internal/utils/xstrings"
	"gorm.io/gorm/clause"
)

var chCache = cache.New[int, model.Channel](16)
var keyCache = cache.New[int, model.ChannelKey](16)
var keyCacheNeedUpdate = make(map[int]struct{})
var keyCacheNeedUpdateLock sync.Mutex
var runtimeUpdateLock sync.Mutex

// GetCache returns the internal channel cache (for backward compatibility).
func GetCache() cache.Cache[int, model.Channel] { return chCache }

// GetKeyCache returns the internal channel key cache (for backward compatibility).
func GetKeyCache() cache.Cache[int, model.ChannelKey] { return keyCache }

// GetKeyCacheNeedUpdate returns the key cache dirty set (for backward compatibility).
func GetKeyCacheNeedUpdate() (map[int]struct{}, *sync.Mutex) {
	return keyCacheNeedUpdate, &keyCacheNeedUpdateLock
}

// GetRuntimeUpdateLock returns the runtime update mutex (for backward compatibility).
func GetRuntimeUpdateLock() *sync.Mutex { return &runtimeUpdateLock }

func List(ctx context.Context) ([]model.Channel, error) {
	channels := make([]model.Channel, 0, chCache.Len())
	for _, ch := range chCache.GetAll() {
		channels = append(channels, ch)
	}
	return channels, nil
}

func Create(ch *model.Channel, ctx context.Context) error {
	if ch != nil {
		if err := ch.RequestRewrite.Validate(ch.Type); err != nil {
			return err
		}
		if ch.GroupID == 0 {
			defaultGroupID, err := GroupDefaultID(ctx)
			if err != nil {
				return err
			}
			ch.GroupID = defaultGroupID
		} else if _, err := GroupGet(ch.GroupID, ctx); err != nil {
			return err
		}
	}
	if err := db.GetDB().WithContext(ctx).Create(ch).Error; err != nil {
		return err
	}
	runtimeUpdateLock.Lock()
	defer runtimeUpdateLock.Unlock()
	chCache.Set(ch.ID, *ch)
	for _, k := range ch.Keys {
		if k.ID != 0 {
			keyCache.Set(k.ID, k)
		}
	}
	return nil
}

func KeyUpdate(key model.ChannelKey) error {
	if key.ID == 0 || key.ChannelID == 0 {
		return fmt.Errorf("invalid channel key")
	}
	runtimeUpdateLock.Lock()
	defer runtimeUpdateLock.Unlock()

	ch, ok := chCache.Get(key.ChannelID)
	if !ok {
		return fmt.Errorf("channel not found")
	}
	if len(ch.Keys) > 0 {
		keys := make([]model.ChannelKey, len(ch.Keys))
		copy(keys, ch.Keys)
		for i := range keys {
			if keys[i].ID == key.ID {
				keys[i] = key
				break
			}
		}
		ch.Keys = keys
	}
	chCache.Set(key.ChannelID, ch)
	keyCache.Set(key.ID, key)
	keyCacheNeedUpdateLock.Lock()
	keyCacheNeedUpdate[key.ID] = struct{}{}
	keyCacheNeedUpdateLock.Unlock()
	return nil
}

func BaseUrlUpdate(channelID int, baseUrl []model.BaseUrl) error {
	runtimeUpdateLock.Lock()
	defer runtimeUpdateLock.Unlock()

	ch, ok := chCache.Get(channelID)
	if !ok {
		return fmt.Errorf("channel not found")
	}
	if baseUrl == nil {
		ch.BaseUrls = nil
	} else {
		cp := make([]model.BaseUrl, len(baseUrl))
		copy(cp, baseUrl)
		ch.BaseUrls = cp
	}
	chCache.Set(channelID, ch)
	return nil
}

func KeySaveDB(ctx context.Context) error {
	keyCacheNeedUpdateLock.Lock()
	keyIDs := make([]int, 0, len(keyCacheNeedUpdate))
	for id := range keyCacheNeedUpdate {
		keyIDs = append(keyIDs, id)
	}
	for id := range keyCacheNeedUpdate {
		delete(keyCacheNeedUpdate, id)
	}
	keyCacheNeedUpdateLock.Unlock()

	if len(keyIDs) == 0 {
		return nil
	}

	keys := make([]model.ChannelKey, 0, len(keyIDs))
	for _, id := range keyIDs {
		k, ok := keyCache.Get(id)
		if !ok {
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return nil
	}

	if err := db.GetDB().WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"channel_id",
			"enabled",
			"channel_key",
			"status_code",
			"last_use_time_stamp",
			"total_cost",
			"remark",
		}),
	}).Create(&keys).Error; err != nil {
		keyCacheNeedUpdateLock.Lock()
		for _, id := range keyIDs {
			keyCacheNeedUpdate[id] = struct{}{}
		}
		keyCacheNeedUpdateLock.Unlock()
		return err
	}
	return nil
}

func Update(req *model.ChannelUpdateRequest, ctx context.Context) (*model.Channel, error) {
	current, ok := chCache.Get(req.ID)
	if !ok {
		return nil, fmt.Errorf("channel not found")
	}

	effectiveType := current.Type
	if req.Type != nil {
		effectiveType = *req.Type
	}

	effectiveRewrite := current.RequestRewrite
	if req.RequestRewrite != nil {
		effectiveRewrite = req.RequestRewrite
	}
	if err := effectiveRewrite.Validate(effectiveType); err != nil {
		return nil, err
	}

	groupID := 0
	if req.GroupID != nil {
		groupID = *req.GroupID
		if groupID == 0 {
			defaultGroupID, err := GroupDefaultID(ctx)
			if err != nil {
				return nil, err
			}
			groupID = defaultGroupID
		} else if _, err := GroupGet(groupID, ctx); err != nil {
			return nil, err
		}
	}

	tx := db.GetDB().WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var selectFields []string
	updates := model.Channel{ID: req.ID}

	if req.Name != nil {
		selectFields = append(selectFields, "name")
		updates.Name = *req.Name
	}
	if req.GroupID != nil {
		selectFields = append(selectFields, "group_id")
		updates.GroupID = groupID
	}
	if req.Type != nil {
		selectFields = append(selectFields, "type")
		updates.Type = *req.Type
	}
	if req.Enabled != nil {
		selectFields = append(selectFields, "enabled")
		updates.Enabled = *req.Enabled
	}
	if req.BaseUrls != nil {
		selectFields = append(selectFields, "base_urls")
		updates.BaseUrls = *req.BaseUrls
	}
	if req.Model != nil {
		selectFields = append(selectFields, "model")
		updates.Model = *req.Model
	}
	if req.CustomModel != nil {
		selectFields = append(selectFields, "custom_model")
		updates.CustomModel = *req.CustomModel
	}
	if req.Proxy != nil {
		selectFields = append(selectFields, "proxy")
		updates.Proxy = *req.Proxy
	}
	if req.AutoSync != nil {
		selectFields = append(selectFields, "auto_sync")
		updates.AutoSync = *req.AutoSync
	}
	if req.AutoGroup != nil {
		selectFields = append(selectFields, "auto_group")
		updates.AutoGroup = *req.AutoGroup
	}
	if req.CustomHeader != nil {
		selectFields = append(selectFields, "custom_header")
		updates.CustomHeader = *req.CustomHeader
	}
	if req.ChannelProxy != nil {
		selectFields = append(selectFields, "channel_proxy")
		updates.ChannelProxy = req.ChannelProxy
	}
	if req.ParamOverride != nil {
		selectFields = append(selectFields, "param_override")
		updates.ParamOverride = req.ParamOverride
	}
	if req.RequestRewrite != nil {
		selectFields = append(selectFields, "request_rewrite")
		updates.RequestRewrite = req.RequestRewrite
	}
	if req.MatchRegex != nil {
		selectFields = append(selectFields, "match_regex")
		updates.MatchRegex = req.MatchRegex
	}

	if len(selectFields) > 0 {
		if err := tx.Model(&model.Channel{}).Where("id = ?", req.ID).Select(selectFields).Updates(&updates).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update channel: %w", err)
		}
	}

	if len(req.KeysToDelete) > 0 {
		if err := tx.Where("id IN ? AND channel_id = ?", req.KeysToDelete, req.ID).Delete(&model.ChannelKey{}).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to delete channel keys: %w", err)
		}
	}

	if len(req.KeysToUpdate) > 0 {
		for _, ku := range req.KeysToUpdate {
			updates := map[string]interface{}{}
			if ku.Enabled != nil {
				updates["enabled"] = *ku.Enabled
			}
			if ku.ChannelKey != nil {
				updates["channel_key"] = *ku.ChannelKey
			}
			if ku.Remark != nil {
				updates["remark"] = *ku.Remark
			}
			if len(updates) == 0 {
				continue
			}
			if err := tx.Model(&model.ChannelKey{}).
				Where("id = ? AND channel_id = ?", ku.ID, req.ID).
				Updates(updates).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to update channel key %d: %w", ku.ID, err)
			}
		}
	}

	if len(req.KeysToAdd) > 0 {
		newKeys := make([]model.ChannelKey, 0, len(req.KeysToAdd))
		for _, ka := range req.KeysToAdd {
			newKeys = append(newKeys, model.ChannelKey{
				ChannelID:  req.ID,
				Enabled:    ka.Enabled,
				ChannelKey: ka.ChannelKey,
				Remark:     ka.Remark,
			})
		}
		if err := tx.Create(&newKeys).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create channel keys: %w", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	if err := RefreshCacheByID(req.ID, ctx); err != nil {
		return nil, err
	}

	ch, _ := chCache.Get(req.ID)
	return &ch, nil
}

func Enabled(id int, enabled bool, ctx context.Context) error {
	runtimeUpdateLock.Lock()
	defer runtimeUpdateLock.Unlock()

	oldCh, ok := chCache.Get(id)
	if !ok {
		return fmt.Errorf("channel not found")
	}
	if err := db.GetDB().WithContext(ctx).Model(&model.Channel{}).Where("id = ?", id).Update("enabled", enabled).Error; err != nil {
		return err
	}
	oldCh.Enabled = enabled
	chCache.Set(id, oldCh)
	return nil
}

func LLMList(ctx context.Context) ([]model.LLMChannel, error) {
	models := []model.LLMChannel{}
	seen := make(map[string]struct{})
	for _, ch := range chCache.GetAll() {
		modelNames := xstrings.SplitTrimCompact(",", ch.Model, ch.CustomModel)
		for _, modelName := range modelNames {
			if modelName == "" {
				continue
			}
			item := model.LLMChannel{
				Name:        modelName,
				Enabled:     ch.Enabled,
				ChannelID:   ch.ID,
				ChannelName: ch.Name,
			}
			key := fmt.Sprintf("%d|%s", item.ChannelID, item.Name)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			models = append(models, item)
		}
	}
	return models, nil
}

func Get(id int, ctx context.Context) (*model.Channel, error) {
	ch, ok := chCache.Get(id)
	if !ok {
		return nil, fmt.Errorf("channel not found")
	}
	return &ch, nil
}

func RefreshCache(ctx context.Context) error {
	channels := []model.Channel{}
	if err := db.GetDB().WithContext(ctx).
		Preload("Keys").
		Preload("Stats").
		Find(&channels).Error; err != nil {
		return err
	}
	runtimeUpdateLock.Lock()
	defer runtimeUpdateLock.Unlock()

	chCache.Clear()
	keyCache.Clear()
	keyCacheNeedUpdateLock.Lock()
	for id := range keyCacheNeedUpdate {
		delete(keyCacheNeedUpdate, id)
	}
	keyCacheNeedUpdateLock.Unlock()

	for _, ch := range channels {
		chCache.Set(ch.ID, ch)
		for _, k := range ch.Keys {
			if k.ID != 0 {
				keyCache.Set(k.ID, k)
			}
		}
	}
	return nil
}

func RefreshCacheByID(id int, ctx context.Context) error {
	runtimeUpdateLock.Lock()
	defer runtimeUpdateLock.Unlock()

	if old, ok := chCache.Get(id); ok {
		for _, k := range old.Keys {
			if k.ID != 0 {
				keyCache.Del(k.ID)
			}
		}
	}
	var ch model.Channel
	if err := db.GetDB().WithContext(ctx).
		Preload("Keys").
		Preload("Stats").
		First(&ch, id).Error; err != nil {
		return err
	}
	chCache.Set(ch.ID, ch)
	for _, k := range ch.Keys {
		if k.ID != 0 {
			keyCache.Set(k.ID, k)
		}
	}
	return nil
}

// Delete performs channel DB deletion transaction (without stats/group cache cleanup).
func Delete(id int, ctx context.Context) error {
	ch, ok := chCache.Get(id)
	if !ok {
		return fmt.Errorf("channel not found")
	}

	tx := db.GetDB().WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&model.GroupItem{}).
		Where("channel_id = ?", id).
		Delete(&model.GroupItem{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete group items: %w", err)
	}

	if err := tx.Where("channel_id = ?", id).Delete(&model.ChannelKey{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete channel keys: %w", err)
	}

	if err := tx.Where("channel_id = ?", id).Delete(&model.StatsChannel{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete channel stats: %w", err)
	}

	if err := tx.Delete(&model.Channel{}, id).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete channel: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	runtimeUpdateLock.Lock()
	chCache.Del(id)
	for _, k := range ch.Keys {
		if k.ID != 0 {
			keyCache.Del(k.ID)
		}
	}
	runtimeUpdateLock.Unlock()

	return nil
}

// GroupDefaultID is a placeholder; replaced by op via callback.
var GroupDefaultID = func(ctx context.Context) (int, error) {
	return 0, fmt.Errorf("channel: GroupDefaultID not registered")
}

// GroupGet is a placeholder; replaced by op via callback.
var GroupGet = func(id int, ctx context.Context) (*model.ChannelGroup, error) {
	return nil, fmt.Errorf("channel: GroupGet not registered")
}
