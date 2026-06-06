package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	appmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/transformer/inbound"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/utils/semantic_cache"
)

func TestBuildSemanticCacheText_ChatMessagesOnlyUsesText(t *testing.T) {
	userText := "hello"
	req := &transmodel.InternalLLMRequest{
		Model: "gpt-4.1",
		Messages: []transmodel.Message{
			{
				Role: "user",
				Content: transmodel.MessageContent{
					Content: &userText,
				},
			},
		},
	}

	namespace, text, ok := buildSemanticCacheLookupInput(7, "chat", req)
	if !ok {
		t.Fatal("expected cacheable request")
	}
	if namespace != "7:chat:gpt-4.1" {
		t.Fatalf("namespace = %q", namespace)
	}
	if text != "user: hello" {
		t.Fatalf("text = %q", text)
	}
}

func TestBuildSemanticCacheLookupInput_AllowsStreamRequests(t *testing.T) {
	stream := true
	userText := "hello"
	req := &transmodel.InternalLLMRequest{
		Model:  "gpt-4.1",
		Stream: &stream,
		Messages: []transmodel.Message{{
			Role: "user",
			Content: transmodel.MessageContent{
				Content: &userText,
			},
		}},
	}

	namespace, text, ok := buildSemanticCacheLookupInput(1, "chat", req)
	if !ok {
		t.Fatal("expected stream request with stable text to use semantic cache")
	}
	if namespace != "1:chat:gpt-4.1" {
		t.Fatalf("namespace = %q", namespace)
	}
	if text != "user: hello" {
		t.Fatalf("text = %q", text)
	}
}

func TestSemanticCacheEndpointFamily_UsesHandlerInputs(t *testing.T) {
	if got := semanticCacheEndpointFamily(appmodel.EndpointTypeChat, inbound.InboundTypeOpenAIChat); got != "chat" {
		t.Fatalf("chat family = %q", got)
	}
	if got := semanticCacheEndpointFamily(appmodel.EndpointTypeResponses, inbound.InboundTypeOpenAIResponse); got != "responses" {
		t.Fatalf("responses family = %q", got)
	}
	if got := semanticCacheEndpointFamily(appmodel.EndpointTypeMessages, inbound.InboundTypeAnthropic); got != "" {
		t.Fatalf("anthropic family = %q, want empty", got)
	}
	if got := semanticCacheEndpointFamily(appmodel.EndpointTypeEmbeddings, inbound.InboundTypeOpenAIEmbedding); got != "" {
		t.Fatalf("embedding family = %q, want empty", got)
	}
}

func TestBuildSemanticCacheLookupInput_IgnoresNonTextPartsAndNormalizesWhitespace(t *testing.T) {
	text := "  hello\n\nworld\t "
	req := &transmodel.InternalLLMRequest{
		Model: "gpt-4.1",
		Messages: []transmodel.Message{
			{
				Role: "user",
				Content: transmodel.MessageContent{
					MultipleContent: []transmodel.MessageContentPart{
						{Type: "text", Text: &text},
						{Type: "image_url", ImageURL: &transmodel.ImageURL{URL: "https://example.com/image.png"}},
						{Type: "input_audio", Audio: &transmodel.Audio{Format: "mp3", Data: "abc"}},
					},
				},
			},
		},
	}

	namespace, normalized, ok := buildSemanticCacheLookupInput(9, "responses", req)
	if !ok {
		t.Fatal("expected cacheable request")
	}
	if namespace != "9:responses:gpt-4.1" {
		t.Fatalf("namespace = %q", namespace)
	}
	if normalized != "user: hello world" {
		t.Fatalf("normalized = %q", normalized)
	}
}

func TestBuildSemanticCacheLookupInput_BypassesRequestsWithoutStableText(t *testing.T) {
	req := &transmodel.InternalLLMRequest{
		Model: "gpt-4.1",
		Messages: []transmodel.Message{
			{
				Role: "user",
				Content: transmodel.MessageContent{
					MultipleContent: []transmodel.MessageContentPart{
						{Type: "image_url", ImageURL: &transmodel.ImageURL{URL: "https://example.com/image.png"}},
					},
				},
			},
		},
	}

	if _, _, ok := buildSemanticCacheLookupInput(1, "chat", req); ok {
		t.Fatal("expected non-text-only request to bypass semantic cache")
	}
}

func TestBuildSemanticCacheLookupInput_RecordsBypassStats(t *testing.T) {
	semantic_cache.ResetRuntimeStats()

	req := &transmodel.InternalLLMRequest{Model: "gpt-4.1"}
	if _, _, ok := buildSemanticCacheLookupInput(1, "chat", req); ok {
		t.Fatal("expected request without stable text to bypass semantic cache")
	}

	stats := semantic_cache.GetRuntimeStats()
	if stats.BypassedRequests != 1 {
		t.Fatalf("bypassed_requests = %d, want 1", stats.BypassedRequests)
	}
}

func TestSemanticCacheRuntimeStatsCounters(t *testing.T) {
	semantic_cache.ResetRuntimeStats()

	semantic_cache.RecordEvaluated()
	semantic_cache.RecordHit()
	semantic_cache.RecordMiss()
	semantic_cache.RecordBypass()
	semantic_cache.RecordStored()

	stats := semantic_cache.GetRuntimeStats()
	if stats.EvaluatedRequests != 1 {
		t.Fatalf("evaluated_requests = %d, want 1", stats.EvaluatedRequests)
	}
	if stats.CacheHitResponses != 1 {
		t.Fatalf("cache_hit_responses = %d, want 1", stats.CacheHitResponses)
	}
	if stats.CacheMissRequests != 1 {
		t.Fatalf("cache_miss_requests = %d, want 1", stats.CacheMissRequests)
	}
	if stats.BypassedRequests != 1 {
		t.Fatalf("bypassed_requests = %d, want 1", stats.BypassedRequests)
	}
	if stats.StoredResponses != 1 {
		t.Fatalf("stored_responses = %d, want 1", stats.StoredResponses)
	}
}

func TestServeStreamingCacheHitEmitsSSEChunks(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	payload := []byte(`{"id":"chatcmpl-cache","created":123,"model":"gpt-4.1","choices":[{"index":0,"message":{"role":"assistant","content":"hello cached stream"}}]}`)
	if err := serveStreamingCacheHit(c, payload, "gpt-4.1"); err != nil {
		t.Fatalf("serveStreamingCacheHit() error = %v", err)
	}

	body := recorder.Body.String()
	if recorder.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", recorder.Header().Get("Content-Type"))
	}
	if !strings.Contains(body, "data: ") || !strings.Contains(body, "hello cached stream") {
		t.Fatalf("body does not contain cached SSE chunk: %s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("body does not contain done marker: %s", body)
	}
}

func TestSemanticCacheHitPayload_AddsSemanticHitMarkerAndRemovesProviderCachedTokens(t *testing.T) {
	payload := []byte(`{"usage":{"input_tokens":100,"cached_tokens":40,"input_token_details":{"cached_tokens":40},"prompt_tokens_details":{"cached_tokens":40}}}`)
	normalized := semanticCacheHitPayload(payload, &transmodel.InternalLLMRequest{})

	var parsed struct {
		Octopus struct {
			SemanticCache struct {
				Hit bool `json:"hit"`
			} `json:"semantic_cache"`
		} `json:"octopus"`
		Usage map[string]any `json:"usage"`
	}
	if err := json.Unmarshal(normalized, &parsed); err != nil {
		t.Fatalf("unmarshal normalized payload: %v", err)
	}
	if !parsed.Octopus.SemanticCache.Hit {
		t.Fatal("expected semantic cache hit marker")
	}
	if _, ok := parsed.Usage["cached_tokens"]; ok {
		t.Fatal("expected top-level cached_tokens to be removed")
	}
	if inputTokenDetails, ok := parsed.Usage["input_token_details"].(map[string]any); ok {
		if _, exists := inputTokenDetails["cached_tokens"]; exists {
			t.Fatal("expected input_token_details.cached_tokens to be removed")
		}
	}
	if promptTokenDetails, ok := parsed.Usage["prompt_tokens_details"].(map[string]any); ok {
		if _, exists := promptTokenDetails["cached_tokens"]; exists {
			t.Fatal("expected prompt_tokens_details.cached_tokens to be removed")
		}
	}
}
