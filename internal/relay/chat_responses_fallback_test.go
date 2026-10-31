package relay

import (
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

func TestOutboundAttemptTypesAnthropicAutoPrefersAnthropicFirst(t *testing.T) {
	// Anthropic inbound + auto format on an OpenAI-compatible channel should try the
	// zero-conversion Anthropic adapter first, then fall back to Chat → Responses.
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatAnthropicMessage}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req, "")
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

func TestOutboundAttemptTypesAnthropicChatFormatStillPrefersChat(t *testing.T) {
	// The explicit "chat" option must NOT sniff the inbound format: it always means
	// Chat Completions first, Responses second, even for Anthropic inbound requests.
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatAnthropicMessage}

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
	}

	if shouldTryAdapterFallback(result, 0, 2) {
		t.Fatal("expected client error ScopeNone to skip adapter fallback so upstream body can be returned immediately")
	}
}

func TestShouldTryAdapterFallbackSkipsWrittenResponse(t *testing.T) {
	result := attemptResult{
		Success:  false,
		Written:  true,
		Decision: RetryDecision{Scope: ScopeNextChannel, Reason: "stream failed", Code: 503, IsError: true},
	}

	if shouldTryAdapterFallback(result, 0, 2) {
		t.Fatal("expected a written response to stop adapter fallback")
	}
}

func TestShouldTryAdapterFallbackSkipsLastAdapter(t *testing.T) {
	result := attemptResult{
		Success:  false,
		Decision: RetryDecision{Scope: ScopeNextChannel, Reason: "gateway error", Code: 503, IsError: true},
	}

	if shouldTryAdapterFallback(result, 1, 2) {
		t.Fatal("expected the last adapter to stop fallback")
	}
}

func TestShouldDeferFailureAccounting(t *testing.T) {
	tests := []struct {
		name       string
		configured bool
		decision   RetryDecision
		written    bool
		want       bool
	}{
		{
			name:       "intermediate route failure",
			configured: true,
			decision:   RetryDecision{Scope: ScopeNextChannel, Code: 404, IsError: true},
			want:       true,
		},
		{
			name:       "final route failure",
			configured: false,
			decision:   RetryDecision{Scope: ScopeNextChannel, Code: 404, IsError: true},
		},
		{
			name:       "same channel failure",
			configured: true,
			decision:   RetryDecision{Scope: ScopeSameChannel, Code: 429, IsError: true},
		},
		{
			name:       "written response",
			configured: true,
			decision:   RetryDecision{Scope: ScopeNextChannel, Code: 503, IsError: true},
			written:    true,
		},
		{
			name:       "accounting skipped",
			configured: true,
			decision:   RetryDecision{Scope: ScopeNextChannel, Code: 503, IsError: true, SkipFailureAccounting: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldDeferFailureAccounting(tt.configured, tt.decision, tt.written); got != tt.want {
				t.Fatalf("shouldDeferFailureAccounting() = %v, want %v", got, tt.want)
			}
		})
	}
}
