package relay

import (
	"errors"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

func TestOutboundAttemptTypesChatOnChatChannelAutoPrefersChat(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIChatCompletion}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "")
	want := []outbound.OutboundType{outbound.OutboundTypeOpenAIChat, outbound.OutboundTypeOpenAIResponse}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesChatOnResponseChannelAutoPrefersChat(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIChatCompletion}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIResponse, req, "")
	want := []outbound.OutboundType{outbound.OutboundTypeOpenAIChat, outbound.OutboundTypeOpenAIResponse}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesResponsesOnChatChannelAutoPrefersChat(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIResponse}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "")
	want := []outbound.OutboundType{outbound.OutboundTypeOpenAIChat, outbound.OutboundTypeOpenAIResponse}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesResponsesOnResponseChannelAutoPrefersChat(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIResponse}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIResponse, req, "")
	want := []outbound.OutboundType{outbound.OutboundTypeOpenAIChat, outbound.OutboundTypeOpenAIResponse}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesEmbeddingNoFallback(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIEmbedding}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "")
	if len(got) != 1 || got[0] != outbound.OutboundTypeOpenAIChat {
		t.Fatalf("attempt types = %#v, want single channel type", got)
	}
}

func TestOutboundAttemptTypesNilRequest(t *testing.T) {
	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, nil, "")
	if len(got) != 1 || got[0] != outbound.OutboundTypeOpenAIChat {
		t.Fatalf("attempt types = %#v, want single channel type", got)
	}
}

func TestOutboundAttemptTypesChatFormatPrefersChatFirst(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIChatCompletion}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "chat")
	want := []outbound.OutboundType{outbound.OutboundTypeOpenAIChat, outbound.OutboundTypeOpenAIResponse}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesResponsesFormatPrefersResponseFirst(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIChatCompletion}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "responses")
	want := []outbound.OutboundType{outbound.OutboundTypeOpenAIResponse, outbound.OutboundTypeOpenAIChat}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesChatOnlyDisablesFallback(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIChatCompletion}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "chat_only")
	want := []outbound.OutboundType{outbound.OutboundTypeOpenAIChat}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesResponsesOnlyDisablesFallback(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIChatCompletion}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "responses_only")
	want := []outbound.OutboundType{outbound.OutboundTypeOpenAIResponse}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesMessagesFormatPrefersAnthropicFirst(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIChatCompletion}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "messages")
	want := []outbound.OutboundType{outbound.OutboundTypeAnthropic, outbound.OutboundTypeOpenAIChat, outbound.OutboundTypeOpenAIResponse}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesMessagesOnlyDisablesFallback(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIChatCompletion}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "messages_only")
	want := []outbound.OutboundType{outbound.OutboundTypeAnthropic}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesPassthroughDisablesFallback(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIChatCompletion}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "passthrough")
	want := []outbound.OutboundType{outbound.OutboundTypePassthrough}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesAnthropicMessagesOnlyUsesAnthropic(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatAnthropicMessage}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "messages_only")
	want := []outbound.OutboundType{outbound.OutboundTypeAnthropic}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesAnthropicMessagesPrefersAnthropicFirst(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatAnthropicMessage}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "messages")
	want := []outbound.OutboundType{outbound.OutboundTypeAnthropic, outbound.OutboundTypeOpenAIChat, outbound.OutboundTypeOpenAIResponse}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesAnthropicPassthroughDisablesFallback(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatAnthropicMessage}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "passthrough")
	want := []outbound.OutboundType{outbound.OutboundTypePassthrough}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesRawPassthroughUsesRawAdapter(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIChatCompletion}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "raw")
	want := []outbound.OutboundType{outbound.OutboundTypeRaw}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesAnthropicRawPassthroughUsesRawAdapter(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatAnthropicMessage}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "raw")
	want := []outbound.OutboundType{outbound.OutboundTypeRaw}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestOutboundAttemptTypesAnthropicAutoStillPrefersChatFallbackChain(t *testing.T) {
	// Anthropic inbound + auto format on an OpenAI-compatible channel should still
	// enter the LLM adapter-selection path instead of hard-coding channel type.
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatAnthropicMessage}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "")
	want := []outbound.OutboundType{outbound.OutboundTypeOpenAIChat, outbound.OutboundTypeOpenAIResponse}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

// Unknown format values fall back to the default auto behavior so a stale or
// mistyped setting never disables routing entirely.
func TestOutboundAttemptTypesUnknownFormatFallsBackToAuto(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIChatCompletion}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "bogus")
	want := []outbound.OutboundType{outbound.OutboundTypeOpenAIChat, outbound.OutboundTypeOpenAIResponse}

	if len(got) != len(want) {
		t.Fatalf("attempt types len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt types = %#v, want %#v", got, want)
		}
	}
}

func TestShouldTryAdapterFallbackSkipsSameChannelFailures(t *testing.T) {
	result := attemptResult{
		Success:  false,
		Written:  false,
		Decision: RetryDecision{Scope: ScopeSameChannel, Reason: "unauthorized", Code: 401, IsError: true},
	}

	if shouldTryAdapterFallback(result, 0, 2) {
		t.Fatal("expected key-scoped failure to skip adapter fallback")
	}
}

func TestShouldTryAdapterFallbackAllowsNextChannelFailures(t *testing.T) {
	result := attemptResult{
		Success:  false,
		Written:  false,
		Decision: RetryDecision{Scope: ScopeNextChannel, Reason: "gateway error", Code: 503, IsError: true},
	}

	if !shouldTryAdapterFallback(result, 0, 2) {
		t.Fatal("expected route-scoped failure to allow adapter fallback")
	}
}

func TestShouldTryAdapterFallbackSkipsClientErrorScopeNone(t *testing.T) {
	result := attemptResult{
		Success:  false,
		Written:  false,
		Decision: RetryDecision{Scope: ScopeNone, Reason: "bad request, client error", Code: 400, IsError: true},
		Err:      errors.New(`channel foo adapter=response attempt 1/4: upstream error: 400: {"error":{"message":"输入内容过长","code":"context_length_exceeded"}}`),
	}

	if shouldTryAdapterFallback(result, 0, 2) {
		t.Fatal("expected client error ScopeNone to skip adapter fallback so upstream body can be returned immediately")
	}
}

func TestShouldTryAdapterFallbackAllowsResponsesToolHistoryMismatch(t *testing.T) {
	result := attemptResult{
		Success:  false,
		Written:  false,
		Decision: RetryDecision{Scope: ScopeNone, Reason: "bad request, client error", Code: 400, IsError: true},
		Err: errors.New(`channel 基元律动 adapter=response attempt 1/4: upstream error: 400: {"error":{"message":"No tool output found for tool call call_01_jCi6YrQJgn7qQhsWk5vD3152.","type":"invalid_request_error"}}`),
	}

	if !shouldTryAdapterFallback(result, 0, 2) {
		t.Fatal("expected Responses tool-history 400 to allow adapter fallback to chat")
	}
	if shouldTryAdapterFallback(result, 1, 2) {
		t.Fatal("expected last adapter attempt to stop even on format mismatch")
	}
}

func TestShouldTryAdapterFallbackSkipsGenericInvalidRequest(t *testing.T) {
	result := attemptResult{
		Success:  false,
		Written:  false,
		Decision: RetryDecision{Scope: ScopeNone, Reason: "bad request, client error", Code: 400, IsError: true},
		Err:      errors.New(`upstream error: 400: {"error":{"message":"invalid_request_error: unknown field","type":"invalid_request_error"}}`),
	}

	if shouldTryAdapterFallback(result, 0, 2) {
		t.Fatal("expected generic 400 invalid_request to stay terminal")
	}
}

func TestIsOutboundAdapterFormatMismatch(t *testing.T) {
	if !isOutboundAdapterFormatMismatch(400, errors.New("No tool output found for function call abc")) {
		t.Fatal("expected function_call wording to match")
	}
	if !isOutboundAdapterFormatMismatch(400, errors.New(`Invalid 'input[3].call_id': empty`)) {
		t.Fatal("expected Invalid 'input[ to match")
	}
	if isOutboundAdapterFormatMismatch(400, errors.New("context_length_exceeded")) {
		t.Fatal("context length must not match format mismatch")
	}
	if isOutboundAdapterFormatMismatch(500, errors.New("No tool output found for tool call x")) {
		t.Fatal("non-400 must not match")
	}
}
