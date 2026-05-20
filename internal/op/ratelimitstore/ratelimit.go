package ratelimitstore

import (
	"fmt"
	"sync"

	"github.com/lingyuins/octopus/internal/utils/ratelimit"
)

var (
	requestBuckets sync.Map // "apiKeyID:modelName" -> *ratelimit.TokenBucket
	tokenBuckets   sync.Map // "apiKeyID:modelName" -> *ratelimit.TokenBucket
)

func rateLimitKey(apiKeyID int, modelName string) string {
	return fmt.Sprintf("%d:%s", apiKeyID, modelName)
}

// CheckRateLimit checks if the request is within the rate limits.
// Returns: allowed, remaining requests, retry-after seconds.
func CheckRateLimit(apiKeyID int, modelName string, rpm int, tpm int, tokenCount int) (allowed bool, remaining int, retryAfter int) {
	key := rateLimitKey(apiKeyID, modelName)

	if rpm > 0 {
		reqBucket := getOrCreateBucket(&requestBuckets, key, rpm, rpm)
		if !reqBucket.Allow() {
			return false, 0, int(reqBucket.ResetAt().Unix())
		}
	}

	if tpm > 0 {
		tokenBucket := getOrCreateBucket(&tokenBuckets, key, tpm, tpm)
		if tokenCount <= 0 {
			tokenCount = 1
		}
		if !tokenBucket.AllowN(tokenCount) {
			return false, 0, int(tokenBucket.ResetAt().Unix())
		}
	}

	if rpm > 0 {
		reqBucket := getOrCreateBucket(&requestBuckets, key, rpm, rpm)
		remaining = reqBucket.TokensRemaining()
	}
	return true, remaining, 0
}

// ConsumeTokens deducts the actual token count from the rate limit bucket after a successful request.
func ConsumeTokens(apiKeyID int, modelName string, tpm int, tokenCount int) {
	if tpm <= 0 || tokenCount <= 0 {
		return
	}
	key := rateLimitKey(apiKeyID, modelName)
	tokenBucket := getOrCreateBucket(&tokenBuckets, key, tpm, tpm)
	tokenBucket.AllowN(tokenCount)
}

func getOrCreateBucket(m *sync.Map, key string, ratePerMinute int, burst int) *ratelimit.TokenBucket {
	if v, ok := m.Load(key); ok {
		if b, ok := v.(*ratelimit.TokenBucket); ok {
			return b
		}
	}
	bucket := ratelimit.NewTokenBucket(ratePerMinute, burst)
	actual, _ := m.LoadOrStore(key, bucket)
	if b, ok := actual.(*ratelimit.TokenBucket); ok {
		return b
	}
	return bucket
}
