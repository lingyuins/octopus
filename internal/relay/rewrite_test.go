package relay

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	appmodel "github.com/lingyuins/octopus/internal/model"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
	openaioutbound "github.com/lingyuins/octopus/internal/transformer/outbound/openai"
)

func TestPrepareInternalRequestForOutbound_IsScopedPerChannelAttempt(t *testing.T) {
	first := "first"
	second := "second"
	baseRequest := &transmodel.InternalLLMRequest{
		Model: "gpt-4o-mini",
		Messages: []transmodel.Message{
			{
				Role: "user",
				Content: transmodel.MessageContent{
					MultipleContent: []transmodel.MessageContentPart{
						{Type: "text", Text: &first},
						{Type: "text", Text: &second},
					},
				},
			},
		},
	}

	rewriteChannel := &appmodel.Channel{
		Type: outbound.OutboundTypeOpenAIChat,
		RequestRewrite: &appmodel.RequestRewriteConfig{
			Enabled: true,
			Profile: appmodel.RequestRewriteProfileOpenAIChatCompat,
		},
	}
	plainChannel := &appmodel.Channel{
		Type: outbound.OutboundTypeOpenAIChat,
	}

	rewritten, _, err := prepareInternalRequestForOutbound(rewriteChannel, baseRequest, &appmodel.Group{EndpointType: appmodel.EndpointTypeDeepSeek})
	if err != nil {
		t.Fatalf("prepareInternalRequestForOutbound() rewrite channel error = %v", err)
	}
	plain, _, err := prepareInternalRequestForOutbound(plainChannel, baseRequest, &appmodel.Group{EndpointType: appmodel.EndpointTypeChat})
	if err != nil {
		t.Fatalf("prepareInternalRequestForOutbound() plain channel error = %v", err)
	}

	if rewritten.Messages[0].Content.Content == nil || *rewritten.Messages[0].Content.Content != "first\nsecond" {
		t.Fatalf("rewritten content = %#v, want flattened string", rewritten.Messages[0].Content)
	}
	if plain.Messages[0].Content.Content != nil {
		t.Fatalf("plain channel content = %#v, want original multipart content", plain.Messages[0].Content)
	}
	if len(plain.Messages[0].Content.MultipleContent) != 2 {
		t.Fatalf("plain channel content parts len = %d, want 2", len(plain.Messages[0].Content.MultipleContent))
	}
	if baseRequest.Messages[0].Content.Content != nil {
		t.Fatal("base request was mutated across channel attempts")
	}
	if rewritten.TransformerMetadata[transmodel.TransformerMetadataGroupEndpointType] != appmodel.EndpointTypeDeepSeek {
		t.Fatalf("rewritten transformer metadata = %#v, want deepseek endpoint type", rewritten.TransformerMetadata)
	}
	if plain.TransformerMetadata[transmodel.TransformerMetadataGroupEndpointType] != appmodel.EndpointTypeChat {
		t.Fatalf("plain transformer metadata = %#v, want chat endpoint type", plain.TransformerMetadata)
	}
}

func TestPrepareInternalRequestForOutbound_ExplicitProviderInAutoModePreservesDeepSeekReasoning(t *testing.T) {
	reasoning := "需要调用天气工具"
	request := &transmodel.InternalLLMRequest{
		Model:        "deepseek-flash",
		RawAPIFormat: transmodel.APIFormatOpenAIChatCompletion,
		Messages: []transmodel.Message{{
			Role:             "assistant",
			ReasoningContent: &reasoning,
			ToolCalls: []transmodel.ToolCall{{
				ID: "call_1", Type: "function",
				Function: transmodel.FunctionCall{Name: "get_weather", Arguments: `{"city":"北京"}`},
			}},
		}},
	}
	group := &appmodel.Group{EndpointType: appmodel.EndpointTypeChat, EndpointProvider: "deepseek", ThinkingMode: appmodel.ThinkingModeAuto}
	prepared, _, err := prepareInternalRequestForOutbound(&appmodel.Channel{Type: outbound.OutboundTypeOpenAIChat}, request, group)
	if err != nil {
		t.Fatal(err)
	}
	if got := prepared.TransformerMetadata[transmodel.TransformerMetadataGroupEndpointType]; got != "deepseek" {
		t.Fatalf("group endpoint metadata = %q, want deepseek", got)
	}

	httpRequest, err := (&openaioutbound.ChatOutbound{}).TransformRequest(context.Background(), prepared, "https://third-party.example/v1", "test-key")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(httpRequest.Body)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Messages []struct {
			ReasoningContent *string `json:"reasoning_content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Messages) != 1 || payload.Messages[0].ReasoningContent == nil || *payload.Messages[0].ReasoningContent != reasoning {
		t.Fatalf("auto-mode DeepSeek reasoning_content lost: %s", body)
	}
}
