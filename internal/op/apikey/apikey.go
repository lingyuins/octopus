package apikey

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/cache"
)

var keyCache = cache.New[int, model.APIKey](16)

// keyIDMap 以**明文** API Key 为键映射到 ID（仅内存），用于热路径查找。
// 数据库只存 SHA-256 哈希（见 hashAPIKey），重启后该 map 为空，
// 由 GetByKey 按哈希回查 DB 惰性重建。
var keyIDMap = cache.New[string, int](16)

// GetCache returns the internal API key cache (for backward compatibility).
func GetCache() cache.Cache[int, model.APIKey] { return keyCache }

// GetIDMap returns the internal key ID map (for backward compatibility).
func GetIDMap() cache.Cache[string, int] { return keyIDMap }

// hashAPIKey 计算 API Key 的 SHA-256 哈希（hex）。数据库只存哈希，
// 泄露数据库文件时无法还原出可用的网关密钥。
func hashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// isLegacyPlaintextAPIKey 判断列值是否为升级前的明文 Key（以 sk- 前缀开头）。
// 新写入的哈希值是 64 位 hex，不带前缀。
func isLegacyPlaintextAPIKey(v string) bool {
	return strings.HasPrefix(v, "sk-")
}

func Create(key *model.APIKey, ctx context.Context) error {
	raw := key.APIKey
	// 落库前替换为哈希；成功后恢复明文供响应与内存缓存使用。
	key.APIKey = hashAPIKey(raw)
	if err := db.GetDB().WithContext(ctx).Create(key).Error; err != nil {
		key.APIKey = raw
		return fmt.Errorf("failed to create API key: %w", err)
	}
	key.APIKey = raw
	keyCache.Set(key.ID, *key)
	keyIDMap.Set(raw, key.ID)
	return nil
}

func Update(key *model.APIKey, ctx context.Context) error {
	existing, ok := keyCache.Get(key.ID)
	if !ok {
		return fmt.Errorf("API key not found")
	}

	// Determine whether the key value itself is being changed.
	newKeyValue := strings.TrimSpace(key.APIKey)
	keyValueChanged := newKeyValue != "" && newKeyValue != existing.APIKey

	if keyValueChanged {
		// 哈希后落库；恢复明文供缓存使用。
		key.APIKey = hashAPIKey(newKeyValue)
		if err := db.GetDB().WithContext(ctx).Save(key).Error; err != nil {
			key.APIKey = newKeyValue
			return fmt.Errorf("failed to update API key: %w", err)
		}
		key.APIKey = newKeyValue
		// 清理该 ID 的旧映射（重启后明文键未知，扫描 keyIDMap 按 ID 删除）。
		for k, v := range keyIDMap.GetAll() {
			if v == key.ID {
				keyIDMap.Del(k)
			}
		}
		keyIDMap.Set(newKeyValue, key.ID)
		keyCache.Set(key.ID, *key)
	} else {
		// Key value unchanged; omit it from the save to avoid accidental overwrite.
		if err := db.GetDB().WithContext(ctx).Omit("api_key").Save(key).Error; err != nil {
			return fmt.Errorf("failed to update API key: %w", err)
		}
		key.APIKey = existing.APIKey
		keyCache.Set(key.ID, *key)
	}
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
		// 明文映射未命中（如重启后）：按哈希回查 DB 并惰性重建映射。
		var key model.APIKey
		if err := db.GetDB().WithContext(ctx).Where("api_key = ?", hashAPIKey(apiKey)).First(&key).Error; err != nil {
			return model.APIKey{}, fmt.Errorf("API key not found")
		}
		keyCache.Set(key.ID, key)
		keyIDMap.Set(apiKey, key.ID)
		return key, nil
	}
	return Get(id, ctx)
}

// DeleteStatsFunc is a callback to delete stats associated with an API key.
// Set by the op package to handle cross-package stats cache references.
var DeleteStatsFunc func(id int)

// DeleteSessionFunc is a callback to delete sticky session entries for an API key.
// Set by the relay package to handle cross-package balancer session cleanup.
var DeleteSessionFunc func(id int)

func Delete(id int, ctx context.Context) error {
	if _, err := Get(id, ctx); err != nil {
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

	if DeleteSessionFunc != nil {
		DeleteSessionFunc(id)
	}

	keyCache.Del(id)
	// keyIDMap 以明文为键（重启后明文未知），按 ID 扫描清理。
	for k, v := range keyIDMap.GetAll() {
		if v == id {
			keyIDMap.Del(k)
		}
	}
	return nil
}

func RefreshCache(ctx context.Context) error {
	apiKeys := []model.APIKey{}
	if err := db.GetDB().WithContext(ctx).Find(&apiKeys).Error; err != nil {
		return err
	}
	keyCache.Clear()
	keyIDMap.Clear()
	for i := range apiKeys {
		// 升级迁移：存量明文 Key（sk- 前缀）一次性哈希化。
		if isLegacyPlaintextAPIKey(apiKeys[i].APIKey) {
			hashed := hashAPIKey(apiKeys[i].APIKey)
			if err := db.GetDB().WithContext(ctx).Model(&model.APIKey{}).
				Where("id = ?", apiKeys[i].ID).
				Update("api_key", hashed).Error; err == nil {
				apiKeys[i].APIKey = hashed
			}
		}
		keyCache.Set(apiKeys[i].ID, apiKeys[i])
	}
	return nil
}
