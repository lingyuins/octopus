package relay

import (
	"errors"
	"net/http"
	"testing"
	"time"

	dbmodel "github.com/lingyuins/octopus/internal/model"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
)

func TestRetryRequestCache_ReusesLookupInput(t *testing.T) {
	cache := newRetryRequestCache()
	calls := 0
	compute := func() (string, string, bool) {
		calls++
		return "ns", "text", true
	}

	_, _, _, fromCache := cache.getLookupInput("k", compute)
	if fromCache {
		t.Fatal("first lookup should not come from cache")
	}
	_, _, _, fromCache = cache.getLookupInput("k", compute)
	if !fromCache {
		t.Fatal("second lookup should come from cache")
	}
	if calls != 1 {
		t.Fatalf("compute calls = %d, want 1", calls)
	}
}

func TestRetryRequestCache_ReusesEmbedding(t *testing.T) {
	cache := newRetryRequestCache()
	calls := 0
	compute := func() ([]float64, error) {
		calls++
		return []float64{1, 2, 3}, nil
	}

	_, _, fromCache := cache.getEmbedding("k", compute)
	if fromCache {
		t.Fatal("first embedding should not come from cache")
	}
	got, _, fromCache := cache.getEmbedding("k", compute)
	if !fromCache {
		t.Fatal("second embedding should come from cache")
	}
	if calls != 1 {
		t.Fatalf("compute calls = %d, want 1", calls)
	}
	if len(got) != 3 || got[0] != 1 {
		t.Fatalf("unexpected embedding: %#v", got)
	}
}

func TestFailureHintCacheStoresRetryableStatuses(t *testing.T) {
	resetFailureHintCache()
	recordFailureHint(1, 2, "gpt-4.1", RetryDecision{Scope: ScopeSameChannel, Reason: "rate limited", Code: http.StatusTooManyRequests, IsError: true}, errors.New("429"), 10)
	if _, ok := globalFailureHintCache.get(1, 2, "gpt-4.1"); !ok {
		t.Fatal("expected failure hint to be stored")
	}
}

func TestFailureHintCacheSkipsBadRequest(t *testing.T) {
	resetFailureHintCache()
	recordFailureHint(1, 2, "gpt-4.1", RetryDecision{Scope: ScopeNone, Reason: "bad request", Code: http.StatusBadRequest, IsError: true}, errors.New("400"), 10)
	if _, ok := globalFailureHintCache.get(1, 2, "gpt-4.1"); ok {
		t.Fatal("did not expect failure hint for bad request")
	}
}

func TestFailureHintCacheExpires(t *testing.T) {
	resetFailureHintCache()
	globalFailureHintCache.set(1, 2, "gpt-4.1", failureHintEntry{statusCode: http.StatusTooManyRequests, expiresAt: time.Now().Add(-time.Second)})
	if _, ok := globalFailureHintCache.get(1, 2, "gpt-4.1"); ok {
		t.Fatal("expected expired hint to be removed")
	}
}

func TestRequestSingleflightKey_BypassesToolsAndStream(t *testing.T) {
	stream := true
	if _, ok := requestSingleflightKey(1, "chat", "gpt-4.1", "hello", &transmodel.InternalLLMRequest{Stream: &stream}); ok {
		t.Fatal("expected stream request to bypass singleflight")
	}
	if _, ok := requestSingleflightKey(1, "chat", "gpt-4.1", "hello", &transmodel.InternalLLMRequest{Tools: []transmodel.Tool{{Type: "function"}}}); ok {
		t.Fatal("expected tool request to bypass singleflight")
	}
}

func TestPrepareCandidateSkipsFailureHint(t *testing.T) {
	resetFailureHintCache()
	globalFailureHintCache.set(1, 2, "gpt-4.1", failureHintEntry{statusCode: http.StatusTooManyRequests, expiresAt: time.Now().Add(time.Second)})
	reason := failureHintSkipReason(failureHintEntry{statusCode: http.StatusTooManyRequests, expiresAt: time.Now().Add(time.Second)})
	if reason == "" {
		t.Fatal("expected skip reason")
	}
	_ = dbmodel.Channel{}
}
