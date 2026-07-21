package relay

import (
	"context"

	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/store"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/utils/semantic_cache"
	"golang.org/x/sync/singleflight"
)

func getFailureHintTTLUnauthorized() time.Duration {
	if v, err := setting.GetInt(dbmodel.SettingKeyFailureHintTTLUnauthorized); err == nil && v > 0 {
		return time.Duration(v) * time.Second
	}
	return 10 * time.Second
}

func getFailureHintTTLRateLimitCap() time.Duration {
	if v, err := setting.GetInt(dbmodel.SettingKeyFailureHintTTLRateLimit); err == nil && v > 0 {
		return time.Duration(v) * time.Second
	}
	return 5 * time.Second
}

func getFailureHintTTLNetwork() time.Duration {
	if v, err := setting.GetInt(dbmodel.SettingKeyFailureHintTTLNetwork); err == nil && v > 0 {
		return time.Duration(v) * time.Second
	}
	return 2 * time.Second
}

type retryLookupInput struct {
	namespace string
	text      string
	ok        bool
}

type retrySemanticEmbedding struct {
	embedding []float64
	err       error
}

type retryRequestCache struct {
	mu sync.Mutex

	lookupInput       map[string]retryLookupInput
	lookupComputed    map[string]bool
	embeddings        map[string]retrySemanticEmbedding
	embeddingComputed map[string]bool
}

func newRetryRequestCache() *retryRequestCache {
	return &retryRequestCache{
		lookupInput:       make(map[string]retryLookupInput),
		lookupComputed:    make(map[string]bool),
		embeddings:        make(map[string]retrySemanticEmbedding),
		embeddingComputed: make(map[string]bool),
	}
}

func (c *retryRequestCache) getLookupInput(cacheKey string, compute func() (string, string, bool)) (string, string, bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lookupComputed[cacheKey] {
		entry := c.lookupInput[cacheKey]
		return entry.namespace, entry.text, entry.ok, true
	}
	namespace, text, ok := compute()
	c.lookupInput[cacheKey] = retryLookupInput{namespace: namespace, text: text, ok: ok}
	c.lookupComputed[cacheKey] = true
	return namespace, text, ok, false
}

func (c *retryRequestCache) getEmbedding(cacheKey string, compute func() ([]float64, error)) ([]float64, error, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.embeddingComputed[cacheKey] {
		entry := c.embeddings[cacheKey]
		return append([]float64(nil), entry.embedding...), entry.err, true
	}
	embedding, err := compute()
	c.embeddings[cacheKey] = retrySemanticEmbedding{embedding: append([]float64(nil), embedding...), err: err}
	c.embeddingComputed[cacheKey] = true
	return append([]float64(nil), embedding...), err, false
}

type inflightRelayResult struct {
	internalResp *transmodel.InternalLLMResponse
	actualModel  string
	attempts     []dbmodel.ChannelAttempt
	namespace    string
	requestText  string
}

func newInflightRelayResult(resp *transmodel.InternalLLMResponse, actualModel string, attempts []dbmodel.ChannelAttempt, namespace string, requestText string) *inflightRelayResult {
	return &inflightRelayResult{
		internalResp: resp,
		actualModel:  actualModel,
		attempts:     append([]dbmodel.ChannelAttempt(nil), attempts...),
		namespace:    namespace,
		requestText:  requestText,
	}
}

var relayInflightGroup singleflight.Group

type failureHintEntry struct {
	decision   RetryDecision
	reason     string
	expiresAt  time.Time
	statusCode int
}

type failureHintCache struct {
	mu      sync.Mutex
	entries map[string]failureHintEntry
}

var globalFailureHintCache = &failureHintCache{entries: make(map[string]failureHintEntry)}

func failureHintKey(channelID, keyID int, modelName string) string {
	return buildFailureHintKey(channelID, keyID, modelName)
}

func shouldStoreFailureHint(statusCode int, err error) (time.Duration, bool) {
	switch statusCode {
	case http.StatusTooManyRequests:
		return getFailureHintTTLRateLimitCap(), true
	case http.StatusUnauthorized, http.StatusForbidden:
		return getFailureHintTTLUnauthorized(), true
	case http.StatusBadRequest:
		return 0, false
	}
	if err == nil {
		return 0, false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return getFailureHintTTLNetwork(), true
	}
	return 0, false
}

func minFailureHintTTL(ttl time.Duration, ratelimitCooldown int) time.Duration {
	if ttl <= 0 {
		return ttl
	}
	if ratelimitCooldown <= 0 {
		return ttl
	}
	cooldown := time.Duration(ratelimitCooldown) * time.Second
	if cooldown < ttl {
		return cooldown
	}
	return ttl
}

func recordFailureHint(channelID, keyID int, modelName string, decision RetryDecision, err error, ratelimitCooldown int) {
	ttl, ok := shouldStoreFailureHint(decision.Code, err)
	if !ok {
		return
	}
	if decision.Code == http.StatusTooManyRequests {
		ttl = minFailureHintTTL(ttl, ratelimitCooldown)
	}
	if ttl <= 0 {
		return
	}
	globalFailureHintCache.set(channelID, keyID, modelName, failureHintEntry{
		decision:   decision,
		reason:     decision.Reason,
		expiresAt:  time.Now().Add(ttl),
		statusCode: decision.Code,
	})
}

// failureHintKeyPrefix 是失败提示缓存在 KVStore 中的子系统前缀（追加到 store 统一前缀后）。
// 完整 Redis key 形如 octopus:fhint:{channelID}:{keyID}:{modelName}。
const failureHintKeyPrefix = "fhint:"

func failureHintKVKey(channelID, keyID int, modelName string) string {
	return failureHintKeyPrefix + failureHintKey(channelID, keyID, modelName)
}

func (c *failureHintCache) set(channelID, keyID int, modelName string, entry failureHintEntry) {
	// Redis 后端：序列化整条 entry 写入 KVStore，TTL 由 Redis 原生过期保证，
	// 无需维护 expiresAt 与主动 purge。降级（Redis err）时回退内存。
	if store.Enabled() {
		data, err := jsonAPI.Marshal(entry)
		if err == nil {
			ttl := time.Until(entry.expiresAt)
			if ttl > 0 {
				kv := store.GetKV()
				if err := kv.Set(context.Background(), failureHintKVKey(channelID, keyID, modelName), data, ttl); err == nil {
					return
				}
			}
		}
		// 序列化/写入失败时回退内存路径，保证失败提示不丢（容错）。
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[failureHintKey(channelID, keyID, modelName)] = entry
}

func (c *failureHintCache) get(channelID, keyID int, modelName string) (failureHintEntry, bool) {
	// Redis 后端：直接读 KVStore，TTL 过期由 Redis 保证。
	if store.Enabled() {
		kv := store.GetKV()
		data, found, err := kv.Get(context.Background(), failureHintKVKey(channelID, keyID, modelName))
		if err == nil && found {
			var entry failureHintEntry
			if jsonAPI.Unmarshal(data, &entry) == nil {
				return entry, true
			}
		}
		// 未找到或出错（Redis 宕机）时回退内存读取，保证降级可用。
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	key := failureHintKey(channelID, keyID, modelName)
	entry, ok := c.entries[key]
	if !ok {
		return failureHintEntry{}, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		return failureHintEntry{}, false
	}
	return entry, true
}

// delete 移除指定 (channel,key,model) 的失败提示，供 429 hold 再试前清理。
func (c *failureHintCache) delete(channelID, keyID int, modelName string) {
	if store.Enabled() {
		_ = store.GetKV().Del(context.Background(), failureHintKVKey(channelID, keyID, modelName))
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, failureHintKey(channelID, keyID, modelName))
}

// purgeExpired 主动清理所有已过期的条目，防止 map 无限增长。
// 由定时任务周期性调用。Redis 模式下 TTL 自动过期，此处为 no-op。
func (c *failureHintCache) purgeExpired() {
	if store.Enabled() {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}

// PurgeFailureHintCache 导出接口供定时任务调用，清理过期的失败提示缓存条目。
func PurgeFailureHintCache() {
	globalFailureHintCache.purgeExpired()
}

func resetFailureHintCache() {
	globalFailureHintCache.mu.Lock()
	defer globalFailureHintCache.mu.Unlock()
	globalFailureHintCache.entries = make(map[string]failureHintEntry)
}

func failureHintSkipReason(entry failureHintEntry) string {
	remaining := time.Until(entry.expiresAt)
	if remaining < 0 {
		remaining = 0
	}
	codeLabel := entry.statusCode
	if codeLabel > 0 {
		return fmt.Sprintf("recent failure hint: %d cooldown %s remaining", codeLabel, remaining.Round(time.Second))
	}
	return fmt.Sprintf("recent failure hint: %s (%s remaining)", strings.TrimSpace(entry.reason), remaining.Round(time.Second))
}

func cloneInternalResponse(resp *transmodel.InternalLLMResponse) *transmodel.InternalLLMResponse {
	if resp == nil {
		return nil
	}
	payload, err := jsonAPI.Marshal(resp)
	if err != nil {
		return nil
	}
	var cloned transmodel.InternalLLMResponse
	if err := jsonAPI.Unmarshal(payload, &cloned); err != nil {
		return nil
	}
	return &cloned
}

func cloneSemanticEmbedding(src []float64) []float64 {
	return append([]float64(nil), src...)
}

func requestSingleflightKey(apiKeyID int, endpointFamily, requestModel, text string, req *transmodel.InternalLLMRequest) (string, bool) {
	if req == nil || apiKeyID <= 0 {
		return "", false
	}
	if req.Stream != nil && *req.Stream {
		return "", false
	}
	if strings.TrimSpace(endpointFamily) == "" || strings.TrimSpace(requestModel) == "" || strings.TrimSpace(text) == "" {
		return "", false
	}
	if len(req.Tools) > 0 {
		return "", false
	}
	return buildSemanticCacheKey(apiKeyID, endpointFamily, requestModel, text), true
}

func lookupSemanticEmbeddingWithCache(ctx context.Context, req *relayRequest, cfg semantic_cache.RuntimeConfig, cacheKey string, text string) ([]float64, bool, error) {
	if req == nil || req.retryCache == nil {
		embedding, err := semantic_cache.NewEmbeddingClient(cfg).CreateEmbedding(ctx, text)
		return embedding, false, err
	}
	embedding, err, fromCache := req.retryCache.getEmbedding(cacheKey, func() ([]float64, error) {
		return semantic_cache.NewEmbeddingClient(cfg).CreateEmbedding(ctx, text)
	})
	return embedding, fromCache, err
}
