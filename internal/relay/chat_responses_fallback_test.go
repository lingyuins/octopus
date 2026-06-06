package relay

import (
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

func TestOutboundAttemptTypesForChatPrefersResponsesThenChat(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIChatCompletion}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req)
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

func TestOutboundAttemptTypesLeavesResponsesRequestsOnResponses(t *testing.T) {
	req := &model.InternalLLMRequest{RawAPIFormat: model.APIFormatOpenAIResponse}

	got := outboundAttemptTypes(outbound.OutboundTypeOpenAIChat, req)
	want := []outbound.OutboundType{outbound.OutboundTypeOpenAIChat}

	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("attempt types = %#v, want %#v", got, want)
	}
}
