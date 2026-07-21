package balancer

import (
	"context"
	"fmt"
	"github.com/lingyuins/octopus/internal/utils/json"
	"strings"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/store"
)

// keyCooldownEntry 记录某个 (channelID, keyID, modelName) 维度的冷却状态。
//
// 背景：原 ChannelKey.StatusCode / LastUseTimeStamp 是整 key 共享字段，某 key 对
// model A 触发 429 后会连带冷却 model B/C——即使其他模型在该 key 上完全正常
// （见 issue #94 公益站部分模型出问题、其他模型没问题的场景）。
// 这里改成与熔断器、失败提示缓存同维度的内存 map，按模型独立冷却。
type keyCooldownEntry struct {
	statusCode int       // 触发冷却的错误码（>= 400）
	expiresAt  time.Time // 冷却到期时间 = 记录时刻 + cooldown
}

// globalKeyCooldown 全局 key 冷却存储，key: "channelID:keyID:modelName"。
// 与 globalBreaker / globalFailureHintCache 同维度，便于复用清理与日志。
// Redis 启用时冷却记录写入 store.KVStore（TTL 原生过期），此 map 仅作内存回退。
var globalKeyCooldown sync.Map // key: string -> *keyCooldownEntry

func cooldownKey(channelID, keyID int, modelName string) string {
	return buildKey3(channelID, keyID, strings.TrimSpace(modelName))
}

// cooldownKVKeyPrefix 是 key 冷却在 KVStore 中的子系统前缀。
// 完整 Redis key 形如 octopus:cooldown:{channelID}:{keyID}:{modelName}。
const cooldownKVKeyPrefix = "cooldown:"

func cooldownKVKey(channelID, keyID int, modelName string) string {
	return cooldownKVKeyPrefix + cooldownKey(channelID, keyID, modelName)
}

// getRatelimitCooldown 读取 key 错误冷却配置（秒），与 relay.getRatelimitCooldown 一致。
// 0 表示关闭冷却。默认 300s（见 model.SettingKeyRatelimitCooldown）。
func getRatelimitCooldown() time.Duration {
	v, err := setting.GetInt(model.SettingKeyRatelimitCooldown)
	if err != nil || v < 0 {
		return 300 * time.Second
	}
	return time.Duration(v) * time.Second
}

// IsKeyOnCooldown 检查某个 (channelID, keyID, modelName) 是否处于冷却中。
// 冷却时长来自 SettingKeyRatelimitCooldown（默认 300s）。
// modelName 为空时直接放行——后台任务（拉模型/探测）不携带模型语义，不应被冷却。
func IsKeyOnCooldown(channelID, keyID int, modelName string) bool {
	if strings.TrimSpace(modelName) == "" {
		return false
	}
	cooldown := getRatelimitCooldown()
	if cooldown <= 0 {
		return false
	}
	// Redis 后端：EXISTS 语义，TTL 过期由 Redis 保证。降级（err/未找到）回退内存。
	if store.Enabled() {
		kv := store.GetKV()
		_, found, err := kv.Get(context.Background(), cooldownKVKey(channelID, keyID, modelName))
		if err == nil {
			return found
		}
	}
	key := cooldownKey(channelID, keyID, modelName)
	v, ok := globalKeyCooldown.Load(key)
	if !ok {
		return false
	}
	entry, ok := v.(*keyCooldownEntry)
	if !ok {
		globalKeyCooldown.Delete(key)
		return false
	}
	if time.Now().Before(entry.expiresAt) {
		return true
	}
	// 已过期，惰性清理
	globalKeyCooldown.Delete(key)
	return false
}

// RecordKeyCooldown 记录某 (channelID, keyID, modelName) 的冷却。
// 仅 statusCode >= 400 才记录（与原 ChannelKey.StatusCode 判断条件一致）。
// 冷却时长来自 SettingKeyRatelimitCooldown（默认 300s）。
func RecordKeyCooldown(channelID, keyID int, modelName string, statusCode int) {
	if statusCode < 400 || strings.TrimSpace(modelName) == "" || keyID == 0 {
		return
	}
	cooldown := getRatelimitCooldown()
	if cooldown <= 0 {
		return
	}
	// Redis 后端：冷却记录写入 KVStore，TTL 原生过期，无需维护内存 map。
	// statusCode 用 JSON 序列化存入（与 failureHintCache 一致的模式）。
	if store.Enabled() {
		entry := keyCooldownEntry{
			statusCode: statusCode,
			expiresAt:  time.Now().Add(cooldown),
		}
		if data, err := json.Marshal(entry); err == nil {
			if err := store.GetKV().Set(context.Background(),
				cooldownKVKey(channelID, keyID, modelName), data, cooldown); err == nil {
				return
			}
		}
		// 序列化/写入失败回退内存路径，保证冷却不丢（容错）。
	}
	key := cooldownKey(channelID, keyID, modelName)
	globalKeyCooldown.Store(key, &keyCooldownEntry{
		statusCode: statusCode,
		expiresAt:  time.Now().Add(cooldown),
	})
}

// ClearKeyCooldown 清除指定 (channelID, keyID, modelName) 的冷却。
// 用于「429 渠道内延时重试」：等待间隔到期后允许再次选中同一 Key。
func ClearKeyCooldown(channelID, keyID int, modelName string) {
	if keyID == 0 || strings.TrimSpace(modelName) == "" {
		return
	}
	if store.Enabled() {
		_ = store.GetKV().Del(context.Background(), cooldownKVKey(channelID, keyID, modelName))
	}
	globalKeyCooldown.Delete(cooldownKey(channelID, keyID, modelName))
}

// PurgeExpiredKeyCooldowns 清理所有已过期的冷却条目，防止 map 无界增长。
// 由 relay log flush 定时任务周期性调用，与 PurgeFailureHintCache 同点清理。
// Redis 模式下 TTL 自动过期，此处为 no-op。
func PurgeExpiredKeyCooldowns() int {
	if store.Enabled() {
		return 0
	}
	now := time.Now()
	removed := 0
	globalKeyCooldown.Range(func(key, value any) bool {
		entry, ok := value.(*keyCooldownEntry)
		if !ok {
			globalKeyCooldown.Delete(key)
			removed++
			return true
		}
		if now.After(entry.expiresAt) {
			globalKeyCooldown.Delete(key)
			removed++
		}
		return true
	})
	return removed
}

// RemoveChannelKeyCooldowns 删除指定渠道的所有冷却条目。
// 在渠道被删除时调用，注册于 OnChannelDeletedHooks，防止 globalKeyCooldown 无限增长。
// Redis 模式下按前缀删除（octopus:cooldown:{channelID}:*）。
func RemoveChannelKeyCooldowns(channelID int) {
	if store.Enabled() {
		// 按前缀删除：octopus:cooldown:{channelID}:*
		_ = store.GetKV().DelByPrefix(context.Background(),
			fmt.Sprintf("%s%d:", cooldownKVKeyPrefix, channelID))
		return
	}
	prefix := buildKeyPrefix(channelID)
	globalKeyCooldown.Range(func(key, _ any) bool {
		if k, ok := key.(string); ok && strings.HasPrefix(k, prefix) {
			globalKeyCooldown.Delete(key)
		}
		return true
	})
}

// RemoveKeyCooldowns 删除指定 key 的所有冷却条目（跨模型）。
// 供 key 禁用/删除时可选调用。key 形如 "channelID:keyID:modelName"，
// 按 ":keyID:" 子串定位（channelID 段不含冒号，故该子串唯一匹配该 keyID）。
// Redis 模式下用 SCAN + 子串匹配删除（前缀无法表达 ":keyID:"）。
func RemoveKeyCooldowns(keyID int) {
	if keyID == 0 {
		return
	}
	needle := buildKeyNeedle(keyID)
	if store.Enabled() {
		// 在 cooldown 命名空间下按子串删除（KVStore.DelBySubstring 内部 SCAN + 客户端过滤）。
		_ = store.GetKV().DelBySubstring(context.Background(), cooldownKVKeyPrefix, needle)
		return
	}
	globalKeyCooldown.Range(func(key, _ any) bool {
		k, ok := key.(string)
		if !ok {
			return true
		}
		if strings.Contains(k, needle) {
			globalKeyCooldown.Delete(key)
		}
		return true
	})
}
