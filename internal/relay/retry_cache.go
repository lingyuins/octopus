package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/utils/semantic_cache"
	"golang.org/x/sync/singleflight"
)

const (
	failureHintTTLUnauthorized = 10 * time.Second
	failureHintTTLRateLimitCap = 5 * time.Second
	failureHintTTLNetwork      = 2 * time.Second
)

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
	namespace    string
	requestText  string
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
	return fmt.Sprintf("%d:%d:%s", channelID, keyID, strings.TrimSpace(modelName))
}

func shouldStoreFailureHint(statusCode int, err error) (time.Duration, bool) {
	switch statusCode {
	case http.StatusTooManyRequests:
		return failureHintTTLRateLimitCap, true
	case http.StatusUnauthorized, http.StatusForbidden:
		return failureHintTTLUnauthorized, true
	case http.StatusBadRequest:
		return 0, false
	}
	if err == nil {
		return 0, false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return failureHintTTLNetwork, true
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

func (c *failureHintCache) set(channelID, keyID int, modelName string, entry failureHintEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[failureHintKey(channelID, keyID, modelName)] = entry
}

func (c *failureHintCache) get(channelID, keyID int, modelName string) (failureHintEntry, bool) {
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
	payload, err := json.Marshal(resp)
	if err != nil {
		return nil
	}
	var cloned transmodel.InternalLLMResponse
	if err := json.Unmarshal(payload, &cloned); err != nil {
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
	return fmt.Sprintf("%d|%s|%s|%s|false", apiKeyID, endpointFamily, requestModel, text), true
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


