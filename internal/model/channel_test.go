package model

import (
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

func TestRequestRewriteConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *RequestRewriteConfig
		channelType outbound.OutboundType
		wantErr     string
	}{
		{
			name:        "nil config is valid",
			cfg:         nil,
			channelType: outbound.OutboundTypeOpenAIChat,
		},
		{
			name: "disabled config is valid",
			cfg: &RequestRewriteConfig{
				Enabled: false,
			},
			channelType: outbound.OutboundTypeOpenAIChat,
		},
		{
			name: "missing profile when enabled",
			cfg: &RequestRewriteConfig{
				Enabled: true,
			},
			channelType: outbound.OutboundTypeOpenAIChat,
			wantErr:     "request rewrite profile is required when enabled",
		},
		{
			name: "unsupported profile",
			cfg: &RequestRewriteConfig{
				Enabled: true,
				Profile: "unknown",
			},
			channelType: outbound.OutboundTypeOpenAIChat,
			wantErr:     "unsupported request rewrite profile: unknown",
		},
		{
			name: "unsupported channel type",
			cfg: &RequestRewriteConfig{
				Enabled: true,
				Profile: RequestRewriteProfileOpenAIChatCompat,
			},
			channelType: outbound.OutboundTypeOpenAIResponse,
			wantErr:     "request rewrite profile openai_chat_compat is not supported for channel type 1",
		},
		{
			name: "unsupported tool role strategy",
			cfg: &RequestRewriteConfig{
				Enabled:          true,
				Profile:          RequestRewriteProfileOpenAIChatCompat,
				ToolRoleStrategy: "broken",
			},
			channelType: outbound.OutboundTypeOpenAIChat,
			wantErr:     "unsupported tool role strategy: broken",
		},
		{
			name: "unsupported system message strategy",
			cfg: &RequestRewriteConfig{
				Enabled:               true,
				Profile:               RequestRewriteProfileOpenAIChatCompat,
				SystemMessageStrategy: "broken",
			},
			channelType: outbound.OutboundTypeOpenAIChat,
			wantErr:     "unsupported system message strategy: broken",
		},
		{
			name: "valid openai chat compat config",
			cfg: &RequestRewriteConfig{
				Enabled:               true,
				Profile:               RequestRewriteProfileOpenAIChatCompat,
				ToolRoleStrategy:      ToolRoleStrategyStringifyToUser,
				SystemMessageStrategy: SystemMessageStrategyMerge,
			},
			channelType: outbound.OutboundTypeOpenAIChat,
		},
		{
			name: "valid mimo chat compat config",
			cfg: &RequestRewriteConfig{
				Enabled:               true,
				Profile:               RequestRewriteProfileOpenAIChatCompat,
				ToolRoleStrategy:      ToolRoleStrategyKeep,
				SystemMessageStrategy: SystemMessageStrategyKeep,
			},
			channelType: outbound.OutboundTypeMimo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate(tt.channelType)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want %q", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("Validate() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestChannelGetNormalizedBaseUrlAddsProviderVersionPath(t *testing.T) {
	tests := []struct {
		name        string
		channelType outbound.OutboundType
		baseURL     string
		suffixMode  string
		want        string
	}{
		{
			name:        "openai root adds v1",
			channelType: outbound.OutboundTypeOpenAIChat,
			baseURL:     "https://api.openai.com",
			want:        "https://api.openai.com/v1",
		},
		{
			name:        "openai v1 is not duplicated",
			channelType: outbound.OutboundTypeOpenAIResponse,
			baseURL:     "https://api.openai.com/v1/",
			want:        "https://api.openai.com/v1",
		},
		{
			name:        "openai compatible api path adds v1",
			channelType: outbound.OutboundTypeOpenAIChat,
			baseURL:     "https://openrouter.ai/api",
			want:        "https://openrouter.ai/api/v1",
		},
		{
			name:        "anthropic root adds v1",
			channelType: outbound.OutboundTypeAnthropic,
			baseURL:     "https://api.anthropic.com",
			want:        "https://api.anthropic.com/v1",
		},
		{
			name:        "gemini root adds v1beta",
			channelType: outbound.OutboundTypeGemini,
			baseURL:     "https://generativelanguage.googleapis.com",
			want:        "https://generativelanguage.googleapis.com/v1beta",
		},
		{
			name:        "gemini v1beta is not duplicated",
			channelType: outbound.OutboundTypeGemini,
			baseURL:     "https://generativelanguage.googleapis.com/v1beta",
			want:        "https://generativelanguage.googleapis.com/v1beta",
		},
		{
			name:        "volcengine root adds api v3",
			channelType: outbound.OutboundTypeVolcengine,
			baseURL:     "https://ark.cn-beijing.volces.com",
			want:        "https://ark.cn-beijing.volces.com/api/v3",
		},
		{
			name:        "mimo root adds v1",
			channelType: outbound.OutboundTypeMimo,
			baseURL:     "https://api.xiaomimimo.com",
			want:        "https://api.xiaomimimo.com/v1",
		},
		{
			name:        "custom suffix mode keeps raw url",
			channelType: outbound.OutboundTypeGemini,
			baseURL:     "https://proxy.example.com/custom",
			suffixMode:  "custom",
			want:        "https://proxy.example.com/custom",
		},
		{
			name:        "openai compatible suffix mode can override channel type",
			channelType: outbound.OutboundTypeGemini,
			baseURL:     "https://proxy.example.com/api",
			suffixMode:  "openai_compat",
			want:        "https://proxy.example.com/api/v1",
		},
		{
			name:        "gemini suffix mode can override channel type",
			channelType: outbound.OutboundTypeOpenAIChat,
			baseURL:     "https://generativelanguage.googleapis.com",
			suffixMode:  "gemini",
			want:        "https://generativelanguage.googleapis.com/v1beta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := Channel{
				Type: tt.channelType,
				BaseUrls: []BaseUrl{
					{URL: tt.baseURL, SuffixMode: tt.suffixMode},
				},
			}

			if got := channel.GetNormalizedBaseUrl(); got != tt.want {
				t.Fatalf("GetNormalizedBaseUrl() = %q, want %q", got, tt.want)
			}
		})
	}
}
