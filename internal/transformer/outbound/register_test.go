package outbound

import (
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

func TestGet_MimoUsesOpenAIChatOutbound(t *testing.T) {
	got := Get(OutboundTypeMimo)
	if got == nil {
		t.Fatal("Get(OutboundTypeMimo) = nil, want outbound instance")
	}
}

func TestIsChatChannelType_Mimo(t *testing.T) {
	if !IsChatChannelType(OutboundTypeMimo) {
		t.Fatal("IsChatChannelType(OutboundTypeMimo) = false, want true")
	}
}

func TestResolveAttemptTypes(t *testing.T) {
	tests := []struct {
		name         string
		channelType  OutboundType
		rawAPIFormat model.APIFormat
		format       string
		want         []OutboundType
	}{
		{
			name:         "auto anthropic inbound on chat channel",
			channelType:  OutboundTypeOpenAIChat,
			rawAPIFormat: model.APIFormatAnthropicMessage,
			want:         []OutboundType{OutboundTypeAnthropic, OutboundTypeOpenAIChat, OutboundTypeOpenAIResponse},
		},
		{
			name:         "auto anthropic inbound on response channel",
			channelType:  OutboundTypeOpenAIResponse,
			rawAPIFormat: model.APIFormatAnthropicMessage,
			want:         []OutboundType{OutboundTypeAnthropic, OutboundTypeOpenAIChat, OutboundTypeOpenAIResponse},
		},
		{
			name:         "explicit chat ignores inbound format",
			channelType:  OutboundTypeOpenAIChat,
			rawAPIFormat: model.APIFormatAnthropicMessage,
			format:       "chat",
			want:         []OutboundType{OutboundTypeOpenAIChat, OutboundTypeOpenAIResponse},
		},
		{
			name:         "explicit responses keeps response first",
			channelType:  OutboundTypeOpenAIChat,
			rawAPIFormat: model.APIFormatAnthropicMessage,
			format:       "responses",
			want:         []OutboundType{OutboundTypeOpenAIResponse, OutboundTypeOpenAIChat},
		},
		{
			name:         "auto openai response inbound remains chat first",
			channelType:  OutboundTypeOpenAIResponse,
			rawAPIFormat: model.APIFormatOpenAIResponse,
			want:         []OutboundType{OutboundTypeOpenAIChat, OutboundTypeOpenAIResponse},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &model.InternalLLMRequest{RawAPIFormat: tt.rawAPIFormat}
			got := ResolveAttemptTypes(tt.channelType, request, tt.format)
			if len(got) != len(tt.want) {
				t.Fatalf("ResolveAttemptTypes() len = %d, want %d: %#v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("ResolveAttemptTypes() = %#v, want %#v", got, tt.want)
				}
			}
		})
	}
}
