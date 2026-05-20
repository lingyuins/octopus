package apikey

import (
	"context"
	"fmt"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/cache"
)

var keyCache = cache.New[int, model.APIKey](16)
var keyIDMap = cache.New[string, int](16)

// GetCache returns the internal API key cache (for backward compatibility).
func GetCache() cache.Cache[int, model.APIKey] { return keyCache }

// GetIDMap returns the internal key ID map (for backward compatibility).
func GetIDMap() cache.Cache[string, int] { return keyIDMap }

func Create(key *model.APIKey, ctx context.Context) error {
	if err := db.GetDB().WithContext(ctx).Create(key).Error; err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}
	keyCache.Set(key.ID, *key)
	keyIDMap.Set(key.APIKey, key.ID)
	return nil
}

func Update(key *model.APIKey, ctx context.Context) error {
	existing, ok := keyCache.Get(key.ID)
	if !ok {
		return fmt.Errorf("API key not found")
	}
	if err := db.GetDB().WithContext(ctx).Omit("api_key").Save(key).Error; err != nil {
		return fmt.Errorf("failed to update API key: %w", err)
	}
	key.APIKey = existing.APIKey
	keyCache.Set(key.ID, *key)
	return nil
}

func List(ctx context.Context) ([]model.APIKey, error) {
	keys := make([]model.APIKey, 0, keyCache.Len())
	for _, apiKey := range keyCache.GetAll() {
		keys = append(keys, apiKey)
	}
	return keys, nil
}

func Get(id int, ctx context.Context) (model.APIKey, error) {
	apiKey, ok := keyCache.Get(id)
	if !ok {
		return model.APIKey{}, fmt.Errorf("API key not found")
	}
	return apiKey, nil
}

func GetByKey(apiKey string, ctx context.Context) (model.APIKey, error) {
	id, ok := keyIDMap.Get(apiKey)
	if !ok {
		return model.APIKey{}, fmt.Errorf("API key not found")
	}
	return Get(id, ctx)
}

// DeleteStatsFunc is a callback to delete stats associated with an API key.
// Set by the op package to handle cross-package stats cache references.
var DeleteStatsFunc func(id int)

func Delete(id int, ctx context.Context) error {
	apiKey, err := Get(id, ctx)
	if err != nil {
		return err
	}
	tx := db.GetDB().WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	result := tx.Delete(&model.APIKey{ID: id})
	if result.Error != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete API key: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("API key not found")
	}
	if err := tx.Where("api_key_id = ?", id).Delete(&model.StatsAPIKey{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete stats API key: %w", err)
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit API key deletion: %w", err)
	}

	if DeleteStatsFunc != nil {
		DeleteStatsFunc(id)
	}

	keyCache.Del(id)
	keyIDMap.Del(apiKey.APIKey)
	return nil
}

func RefreshCache(ctx context.Context) error {
	apiKeys := []model.APIKey{}
	if err := db.GetDB().WithContext(ctx).Find(&apiKeys).Error; err != nil {
		return err
	}
	keyCache.Clear()
	keyIDMap.Clear()
	for _, apiKey := range apiKeys {
		keyCache.Set(apiKey.ID, apiKey)
		keyIDMap.Set(apiKey.APIKey, apiKey.ID)
	}
	return nil
}
