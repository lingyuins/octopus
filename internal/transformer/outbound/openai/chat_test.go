package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

func TestChatOutboundTransformRequest_NormalizesOpenAICompatMessages(t *testing.T) {
	outbound := &ChatOutbound{}
	toolCallID := "call_1"
	first := "first block"
	second := "second block"
	toolFirst := "tool output 1"
	toolSecond := "tool output 2"

	request := &model.InternalLLMRequest{
		Model: "gpt-4o-mini",
		Messages: []model.Message{
			{
				Role: "user",
				Content: model.MessageContent{
					MultipleContent: []model.MessageContentPart{
						{Type: "text", Text: &first},
						{Type: "text", Text: &second},
					},
				},
			},
			{
				Role: "assistant",
				ToolCalls: []model.ToolCall{
					{
						ID:   toolCallID,
						Type: "function",
						Function: model.FunctionCall{
							Name:      "lookup_weather",
							Arguments: `{"city":"Shanghai"}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: &toolCallID,
				Content: model.MessageContent{
					MultipleContent: []model.MessageContentPart{
						{Type: "text", Text: &toolFirst},
						{Type: "text", Text: &toolSecond},
					},
				},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://nullapi.example.com", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			Role      string           `json:"role"`
			Content   json.RawMessage  `json:"content"`
			ToolCalls []model.ToolCall `json:"tool_calls,omitempty"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(got.Messages))
	}

	assertJSONEncodedString(t, got.Messages[0].Content, "first block\nsecond block")
	assertJSONEncodedString(t, got.Messages[1].Content, "")
	assertJSONEncodedString(t, got.Messages[2].Content, "tool output 1\ntool output 2")

	if len(got.Messages[1].ToolCalls) != 1 {
		t.Fatalf("expected assistant tool_calls to be preserved, got %d", len(got.Messages[1].ToolCalls))
	}

	if request.Messages[0].Content.Content != nil {
		t.Fatalf("expected original request user content to remain unflattened")
	}
	if len(request.Messages[0].Content.MultipleContent) != 2 {
		t.Fatalf("expected original request user content parts to stay intact")
	}
	if request.Messages[1].Content.Content != nil {
		t.Fatalf("expected original request assistant content to remain nil")
	}
}

func TestChatOutboundTransformRequest_PreservesMixedMultiPartContent(t *testing.T) {
	outbound := &ChatOutbound{}
	text := "look at this image"
	imageURL := "https://example.com/image.png"

	request := &model.InternalLLMRequest{
		Model: "gpt-4o-mini",
		Messages: []model.Message{
			{
				Role: "user",
				Content: model.MessageContent{
					MultipleContent: []model.MessageContentPart{
						{Type: "text", Text: &text},
						{Type: "image_url", ImageURL: &model.ImageURL{URL: imageURL}},
					},
				},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://nullapi.example.com", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}

	var contentParts []map[string]any
	if err := json.Unmarshal(got.Messages[0].Content, &contentParts); err != nil {
		t.Fatalf("expected mixed multipart content to stay as array, got error: %v", err)
	}
	if len(contentParts) != 2 {
		t.Fatalf("expected 2 content parts, got %d", len(contentParts))
	}
}

func TestChatOutboundTransformRequest_PreservesReasoningContentForDeepSeekToolContinuation(t *testing.T) {
	outbound := &ChatOutbound{}
	reasoning := "need one more tool round"
	content := ""
	toolCallID := "call_1"

	request := &model.InternalLLMRequest{
		Model: "deepseek-v4-pro",
		Messages: []model.Message{
			{
				Role: "assistant",
				Content: model.MessageContent{
					Content: &content,
				},
				ReasoningContent: &reasoning,
				ToolCalls: []model.ToolCall{
					{
						ID:   toolCallID,
						Type: "function",
						Function: model.FunctionCall{
							Name:      "lookup_weather",
							Arguments: `{"city":"Shanghai"}`,
						},
					},
				},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.deepseek.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			ReasoningContent *string `json:"reasoning_content,omitempty"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}
	if got.Messages[0].ReasoningContent == nil || *got.Messages[0].ReasoningContent != reasoning {
		t.Fatalf("expected reasoning_content %q, got %#v", reasoning, got.Messages[0].ReasoningContent)
	}
	if request.Messages[0].ReasoningContent == nil || *request.Messages[0].ReasoningContent != reasoning {
		t.Fatalf("expected original request reasoning_content to stay intact")
	}
}

func TestChatOutboundTransformRequest_PreservesReasoningContentForMimoToolContinuation(t *testing.T) {
	outbound := &ChatOutbound{}
	reasoning := "need one more tool round"
	content := ""
	toolCallID := "call_1"

	request := &model.InternalLLMRequest{
		Model: "mimo-v2.5-pro",
		Messages: []model.Message{
			{
				Role: "assistant",
				Content: model.MessageContent{
					Content: &content,
				},
				ReasoningContent: &reasoning,
				ToolCalls: []model.ToolCall{
					{
						ID:   toolCallID,
						Type: "function",
						Function: model.FunctionCall{
							Name:      "lookup_weather",
							Arguments: `{"city":"Shanghai"}`,
						},
					},
				},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.xiaomimimo.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			ReasoningContent *string `json:"reasoning_content,omitempty"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}
	if got.Messages[0].ReasoningContent == nil || *got.Messages[0].ReasoningContent != reasoning {
		t.Fatalf("expected reasoning_content %q, got %#v", reasoning, got.Messages[0].ReasoningContent)
	}
	if request.Messages[0].ReasoningContent == nil || *request.Messages[0].ReasoningContent != reasoning {
		t.Fatalf("expected original request reasoning_content to stay intact")
	}
}

func TestChatOutboundTransformRequest_AttachesStandaloneDeepSeekReasoningToNextToolCall(t *testing.T) {
	outbound := &ChatOutbound{}
	reasoning := "need one more tool round"
	content := ""
	toolCallID := "call_1"

	request := &model.InternalLLMRequest{
		Model: "deepseek-v4-pro",
		Messages: []model.Message{
			{
				Role:             "assistant",
				ReasoningContent: &reasoning,
			},
			{
				Role: "assistant",
				Content: model.MessageContent{
					Content: &content,
				},
				ToolCalls: []model.ToolCall{
					{
						ID:   toolCallID,
						Type: "function",
						Function: model.FunctionCall{
							Name:      "lookup_weather",
							Arguments: `{"city":"Shanghai"}`,
						},
					},
				},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.deepseek.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			Role             string           `json:"role"`
			ReasoningContent *string          `json:"reasoning_content,omitempty"`
			ToolCalls        []model.ToolCall `json:"tool_calls,omitempty"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) != 1 {
		t.Fatalf("expected standalone reasoning message to be merged, got %d messages: %s", len(got.Messages), body)
	}
	if got.Messages[0].ReasoningContent == nil || *got.Messages[0].ReasoningContent != reasoning {
		t.Fatalf("expected reasoning_content %q on tool-call message, got %#v", reasoning, got.Messages[0].ReasoningContent)
	}
	if len(got.Messages[0].ToolCalls) != 1 {
		t.Fatalf("expected tool_calls to be preserved, got %d", len(got.Messages[0].ToolCalls))
	}
	if request.Messages[0].ReasoningContent == nil || *request.Messages[0].ReasoningContent != reasoning {
		t.Fatalf("expected original standalone reasoning message to stay intact")
	}
}

func TestChatOutboundTransformRequest_DropsTrailingStandaloneDeepSeekReasoning(t *testing.T) {
	outbound := &ChatOutbound{}
	reasoning := "orphan reasoning"

	request := &model.InternalLLMRequest{
		Model: "deepseek-v4-pro",
		Messages: []model.Message{
			{
				Role:             "assistant",
				ReasoningContent: &reasoning,
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.deepseek.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			ReasoningContent *string `json:"reasoning_content,omitempty"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) != 0 {
		t.Fatalf("expected trailing standalone reasoning to be dropped, got %d messages: %s", len(got.Messages), body)
	}
}

func TestChatOutboundTransformRequest_DropsStandaloneDeepSeekReasoningBeforeFinalAnswer(t *testing.T) {
	outbound := &ChatOutbound{}
	reasoning := "final answer reasoning is optional and ignored"
	content := "final answer"

	request := &model.InternalLLMRequest{
		Model: "deepseek-v4-pro",
		Messages: []model.Message{
			{
				Role:             "assistant",
				ReasoningContent: &reasoning,
			},
			{
				Role: "assistant",
				Content: model.MessageContent{
					Content: &content,
				},
			},
			{
				Role: "user",
				Content: model.MessageContent{
					Content: loPtr("next question"),
				},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.deepseek.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			Role             string  `json:"role"`
			ReasoningContent *string `json:"reasoning_content,omitempty"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) != 2 {
		t.Fatalf("expected standalone final-answer reasoning to be dropped, got %d messages: %s", len(got.Messages), body)
	}
	for i, msg := range got.Messages {
		if msg.ReasoningContent != nil {
			t.Fatalf("expected no reasoning_content on message %d, got %q", i, *msg.ReasoningContent)
		}
	}
}

func TestChatOutboundTransformRequest_AttachesStandaloneDeepSeekReasoningAcrossAssistantTextToToolCall(t *testing.T) {
	outbound := &ChatOutbound{}
	reasoning := "tool reasoning emitted separately"
	finalContent := "previous final answer"
	toolContent := ""
	toolCallID := "call_1"

	request := &model.InternalLLMRequest{
		Model: "deepseek-v4-pro",
		Messages: []model.Message{
			{
				Role:             "assistant",
				ReasoningContent: &reasoning,
			},
			{
				Role: "assistant",
				Content: model.MessageContent{
					Content: &finalContent,
				},
			},
			{
				Role: "assistant",
				Content: model.MessageContent{
					Content: &toolContent,
				},
				ToolCalls: []model.ToolCall{
					{
						ID:   toolCallID,
						Type: "function",
						Function: model.FunctionCall{
							Name:      "lookup_weather",
							Arguments: `{"city":"Shanghai"}`,
						},
					},
				},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.deepseek.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			Role             string           `json:"role"`
			ReasoningContent *string          `json:"reasoning_content,omitempty"`
			ToolCalls        []model.ToolCall `json:"tool_calls,omitempty"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) != 2 {
		t.Fatalf("expected standalone reasoning to be dropped then attached to tool call, got %d messages: %s", len(got.Messages), body)
	}
	if got.Messages[0].ReasoningContent != nil {
		t.Fatalf("expected final answer message to omit reasoning_content, got %q", *got.Messages[0].ReasoningContent)
	}
	if got.Messages[1].ReasoningContent == nil || *got.Messages[1].ReasoningContent != reasoning {
		t.Fatalf("expected reasoning_content %q on tool-call message, got %#v", reasoning, got.Messages[1].ReasoningContent)
	}
	if len(got.Messages[1].ToolCalls) != 1 {
		t.Fatalf("expected tool_calls to be preserved, got %d", len(got.Messages[1].ToolCalls))
	}
}

func TestChatOutboundTransformRequest_PreservesReasoningContentForDeepSeekFollowUpTurn(t *testing.T) {
	outbound := &ChatOutbound{}
	reasoning := "finished reasoning from the prior turn"
	content := "final answer"

	request := &model.InternalLLMRequest{
		Model: "deepseek-v4-pro",
		Messages: []model.Message{
			{
				Role: "assistant",
				Content: model.MessageContent{
					Content: &content,
				},
				ReasoningContent: &reasoning,
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.deepseek.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			ReasoningContent *string `json:"reasoning_content,omitempty"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}
	if got.Messages[0].ReasoningContent == nil || *got.Messages[0].ReasoningContent != reasoning {
		t.Fatalf("expected reasoning_content %q to be preserved for DeepSeek follow-up turns, got %#v", reasoning, got.Messages[0].ReasoningContent)
	}
	if request.Messages[0].ReasoningContent == nil || *request.Messages[0].ReasoningContent != reasoning {
		t.Fatalf("expected original request reasoning_content to stay intact")
	}
}

func TestChatOutboundTransformRequest_PreservesReasoningContentForDeepSeekEndpointCategory(t *testing.T) {
	outbound := &ChatOutbound{}
	reasoning := "preserve via deepseek endpoint category"
	content := "final answer"
	toolCallID := "call_123"
	toolCallName := "search"
	toolCallArgs := "{}"

	request := &model.InternalLLMRequest{
		Model: "custom-chat-model",
		Messages: []model.Message{
			{
				Role: "assistant",
				Content: model.MessageContent{
					Content: &content,
				},
				ReasoningContent: &reasoning,
				ToolCalls: []model.ToolCall{
					{
						ID:   toolCallID,
						Type: "function",
						Function: model.FunctionCall{
							Name:      toolCallName,
							Arguments: toolCallArgs,
						},
					},
				},
			},
		},
		TransformerMetadata: map[string]string{
			model.TransformerMetadataGroupEndpointType: "deepseek",
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://proxy.example.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			ReasoningContent *string `json:"reasoning_content,omitempty"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}
	if got.Messages[0].ReasoningContent == nil || *got.Messages[0].ReasoningContent != reasoning {
		t.Fatalf("expected reasoning_content %q to be preserved for deepseek endpoint category, got %#v", reasoning, got.Messages[0].ReasoningContent)
	}
}

func TestChatOutboundTransformRequest_PreservesReasoningContentForDeepSeekReasonerFollowUpTurn(t *testing.T) {
	outbound := &ChatOutbound{}
	reasoning := "reasoner chain of thought"
	content := "final answer"

	request := &model.InternalLLMRequest{
		Model: "deepseek-reasoner",
		Messages: []model.Message{
			{
				Role: "assistant",
				Content: model.MessageContent{
					Content: &content,
				},
				ReasoningContent: &reasoning,
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.deepseek.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			ReasoningContent *string `json:"reasoning_content,omitempty"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}
	if got.Messages[0].ReasoningContent == nil || *got.Messages[0].ReasoningContent != reasoning {
		t.Fatalf("expected reasoning_content %q to be preserved for deepseek-reasoner follow-up turns, got %#v", reasoning, got.Messages[0].ReasoningContent)
	}
}

func TestChatOutboundTransformRequest_ClearsReasoningContentForGenericOpenAICompat(t *testing.T) {
	outbound := &ChatOutbound{}
	reasoning := "should not be sent to generic openai compat"
	content := ""

	request := &model.InternalLLMRequest{
		Model: "gpt-4o-mini",
		Messages: []model.Message{
			{
				Role: "assistant",
				Content: model.MessageContent{
					Content: &content,
				},
				ReasoningContent: &reasoning,
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.openai.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			ReasoningContent *string `json:"reasoning_content,omitempty"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}
	if got.Messages[0].ReasoningContent != nil {
		t.Fatalf("expected reasoning_content to be cleared for generic openai compat, got %q", *got.Messages[0].ReasoningContent)
	}
}

func TestChatOutboundTransformRequest_NormalizesReasoningAliasForDeepSeekToolContinuation(t *testing.T) {
	outbound := &ChatOutbound{}
	reasoning := "provider-specific reasoning alias"
	content := ""
	toolCallID := "call_2"

	request := &model.InternalLLMRequest{
		Model: "deepseek-chat",
		Messages: []model.Message{
			{
				Role: "assistant",
				Content: model.MessageContent{
					Content: &content,
				},
				Reasoning: &reasoning,
				ToolCalls: []model.ToolCall{
					{
						ID:   toolCallID,
						Type: "function",
						Function: model.FunctionCall{
							Name:      "lookup_weather",
							Arguments: `{"city":"Shanghai"}`,
						},
					},
				},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.deepseek.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			ReasoningContent *string `json:"reasoning_content,omitempty"`
			Reasoning        *string `json:"reasoning,omitempty"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}
	if got.Messages[0].ReasoningContent == nil || *got.Messages[0].ReasoningContent != reasoning {
		t.Fatalf("expected reasoning_content %q, got %#v", reasoning, got.Messages[0].ReasoningContent)
	}
	if got.Messages[0].Reasoning != nil {
		t.Fatalf("expected reasoning alias to be normalized away for DeepSeek, got %q", *got.Messages[0].Reasoning)
	}
	if request.Messages[0].Reasoning == nil || *request.Messages[0].Reasoning != reasoning {
		t.Fatalf("expected original request reasoning to stay intact")
	}
}

func TestChatOutboundTransformRequest_ClearsReasoningAliasForGenericOpenAICompat(t *testing.T) {
	outbound := &ChatOutbound{}
	reasoning := "should not be sent as reasoning alias"
	content := ""

	request := &model.InternalLLMRequest{
		Model: "gpt-4o-mini",
		Messages: []model.Message{
			{
				Role: "assistant",
				Content: model.MessageContent{
					Content: &content,
				},
				Reasoning: &reasoning,
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.openai.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			Reasoning *string `json:"reasoning,omitempty"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}
	if got.Messages[0].Reasoning != nil {
		t.Fatalf("expected reasoning alias to be cleared for generic openai compat, got %q", *got.Messages[0].Reasoning)
	}
}

func TestChatOutboundTransformRequest_OmitsNoneReasoningEffort(t *testing.T) {
	outbound := &ChatOutbound{}
	stream := false

	request := &model.InternalLLMRequest{
		Model:             "mimo-v2.5-pro",
		ReasoningEffort:   "none",
		Store:             &stream,
		ParallelToolCalls: &stream,
		Messages: []model.Message{
			{
				Role: "user",
				Content: model.MessageContent{
					Content: loPtr("Use the get_current_time tool."),
				},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://token-plan-cn.xiaomimimo.com/v1", "tp-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		ReasoningEffort *string `json:"reasoning_effort,omitempty"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if got.ReasoningEffort != nil {
		t.Fatalf("expected reasoning_effort to be omitted, got %q", *got.ReasoningEffort)
	}
	if request.ReasoningEffort != "none" {
		t.Fatalf("expected original request reasoning_effort to stay intact, got %q", request.ReasoningEffort)
	}
}

func TestChatOutboundTransformRequest_MapsDeepSeekThinkingControls(t *testing.T) {
	outbound := &ChatOutbound{}
	stream := false

	request := &model.InternalLLMRequest{
		Model:           "DeepSeek-V4-Pro",
		ReasoningEffort: "xhigh",
		Store:           &stream,
		ExtraBody:       json.RawMessage(`{"thinking":{"type":"enabled"},"foo":"bar"}`),
		Messages: []model.Message{
			{
				Role: "user",
				Content: model.MessageContent{
					Content: loPtr("hello"),
				},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.deepseek.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		ReasoningEffort string         `json:"reasoning_effort,omitempty"`
		ExtraBody       map[string]any `json:"extra_body,omitempty"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if got.ReasoningEffort != "max" {
		t.Fatalf("expected deepseek reasoning_effort max, got %q", got.ReasoningEffort)
	}

	thinking, ok := got.ExtraBody["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("expected extra_body.thinking to be preserved, got %#v", got.ExtraBody)
	}
	if thinking["type"] != "enabled" {
		t.Fatalf("expected extra_body.thinking.type enabled, got %#v", thinking["type"])
	}
	if got.ExtraBody["foo"] != "bar" {
		t.Fatalf("expected extra_body custom fields to be preserved, got %#v", got.ExtraBody["foo"])
	}
}

func TestChatOutboundTransformRequest_MapsMimoThinkingControls(t *testing.T) {
	outbound := &ChatOutbound{}
	stream := false

	request := &model.InternalLLMRequest{
		Model:           "mimo-v2.5-pro",
		ReasoningEffort: "xhigh",
		Store:           &stream,
		ExtraBody:       json.RawMessage(`{"thinking":{"type":"enabled"},"foo":"bar"}`),
		Messages: []model.Message{
			{
				Role: "user",
				Content: model.MessageContent{
					Content: loPtr("hello"),
				},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.xiaomimimo.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		ReasoningEffort string         `json:"reasoning_effort,omitempty"`
		ExtraBody       map[string]any `json:"extra_body,omitempty"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if got.ReasoningEffort != "max" {
		t.Fatalf("expected mimo reasoning_effort max, got %q", got.ReasoningEffort)
	}

	thinking, ok := got.ExtraBody["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("expected extra_body.thinking to be preserved, got %#v", got.ExtraBody)
	}
	if thinking["type"] != "enabled" {
		t.Fatalf("expected extra_body.thinking.type enabled, got %#v", thinking["type"])
	}
	if got.ExtraBody["foo"] != "bar" {
		t.Fatalf("expected extra_body custom fields to be preserved, got %#v", got.ExtraBody["foo"])
	}
	if got.ReasoningEffort != "max" {
		t.Fatalf("expected mimo reasoning_effort max, got %q", got.ReasoningEffort)
	}
}

func TestChatOutboundTransformRequest_SetsDefaultMimoMaxCompletionTokens(t *testing.T) {
	outbound := &ChatOutbound{}
	request := &model.InternalLLMRequest{
		Model: "mimo-v2.5-pro",
		Messages: []model.Message{{
			Role:    "user",
			Content: model.MessageContent{Content: loPtr("hello")},
		}},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.xiaomimimo.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		MaxCompletionTokens *int64 `json:"max_completion_tokens,omitempty"`
		MaxTokens           *int64 `json:"max_tokens,omitempty"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	// When the client does not specify any token limit, Octopus should not
	// inject a default — let the upstream provider decide.
	if got.MaxCompletionTokens != nil {
		t.Fatalf("expected max_completion_tokens to be omitted when client sends none, got %#v", got.MaxCompletionTokens)
	}
	if got.MaxTokens != nil {
		t.Fatalf("expected max_tokens to be omitted, got %#v", got.MaxTokens)
	}
}

func TestChatOutboundTransformRequest_RaisesSmallMimoMaxTokens(t *testing.T) {
	outbound := &ChatOutbound{}
	maxTokens := int64(50)
	request := &model.InternalLLMRequest{
		Model:     "mimo-v2.5-pro",
		MaxTokens: &maxTokens,
		Messages: []model.Message{{
			Role:    "user",
			Content: model.MessageContent{Content: loPtr("hello")},
		}},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.xiaomimimo.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		MaxCompletionTokens *int64 `json:"max_completion_tokens,omitempty"`
		MaxTokens           *int64 `json:"max_tokens,omitempty"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if got.MaxCompletionTokens == nil || *got.MaxCompletionTokens != 10926 {
		t.Fatalf("expected max_completion_tokens 10926, got %#v", got.MaxCompletionTokens)
	}
	if got.MaxTokens != nil {
		t.Fatalf("expected max_tokens to be omitted after upgrade, got %#v", got.MaxTokens)
	}
}

func TestChatOutboundTransformRequest_DisablesDeepSeekThinkingWhenReasoningEffortNone(t *testing.T) {
	outbound := &ChatOutbound{}
	stream := false

	request := &model.InternalLLMRequest{
		Model:           "DeepSeek-V4-Pro",
		ReasoningEffort: "none",
		Store:           &stream,
		Messages: []model.Message{
			{
				Role: "user",
				Content: model.MessageContent{
					Content: loPtr("hello"),
				},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.deepseek.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	bodyText := string(body)
	if strings.Contains(bodyText, `"reasoning_effort":"none"`) {
		t.Fatalf("expected invalid reasoning_effort none to be omitted, got %s", bodyText)
	}

	var got struct {
		ReasoningEffort *string `json:"reasoning_effort,omitempty"`
		ExtraBody       struct {
			Thinking struct {
				Type string `json:"type"`
			} `json:"thinking"`
		} `json:"extra_body,omitempty"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if got.ReasoningEffort != nil {
		t.Fatalf("expected reasoning_effort to be omitted, got %q", *got.ReasoningEffort)
	}
	if got.ExtraBody.Thinking.Type != "disabled" {
		t.Fatalf("expected deepseek thinking to be disabled, got %q", got.ExtraBody.Thinking.Type)
	}
}

func TestChatOutboundTransformRequest_DisablesMimoThinkingWhenReasoningEffortNone(t *testing.T) {
	outbound := &ChatOutbound{}
	stream := false

	request := &model.InternalLLMRequest{
		Model:           "mimo-v2.5-pro",
		ReasoningEffort: "none",
		Store:           &stream,
		Messages: []model.Message{
			{
				Role: "user",
				Content: model.MessageContent{
					Content: loPtr("hello"),
				},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.xiaomimimo.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	bodyText := string(body)
	if strings.Contains(bodyText, `"reasoning_effort":"none"`) {
		t.Fatalf("expected invalid reasoning_effort none to be omitted, got %s", bodyText)
	}

	var got struct {
		ReasoningEffort *string `json:"reasoning_effort,omitempty"`
		ExtraBody       struct {
			Thinking struct {
				Type string `json:"type"`
			} `json:"thinking"`
		} `json:"extra_body,omitempty"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if got.ReasoningEffort != nil {
		t.Fatalf("expected reasoning_effort to be omitted, got %q", *got.ReasoningEffort)
	}
	if got.ExtraBody.Thinking.Type != "disabled" {
		t.Fatalf("expected mimo thinking to be disabled, got %q", got.ExtraBody.Thinking.Type)
	}
}

func TestChatOutboundTransformRequest_DeepSeekThinkingDisabledOverridesReasoningEffort(t *testing.T) {
	outbound := &ChatOutbound{}
	stream := false

	request := &model.InternalLLMRequest{
		Model:           "deepseek-v4-pro",
		ReasoningEffort: "high",
		Store:           &stream,
		ExtraBody:       json.RawMessage(`{"thinking":{"type":"disabled"}}`),
		Messages: []model.Message{
			{
				Role: "user",
				Content: model.MessageContent{
					Content: loPtr("hello"),
				},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.deepseek.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		ReasoningEffort *string `json:"reasoning_effort,omitempty"`
		ExtraBody       struct {
			Thinking struct {
				Type string `json:"type"`
			} `json:"thinking"`
		} `json:"extra_body,omitempty"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if got.ReasoningEffort != nil {
		t.Fatalf("expected reasoning_effort to be omitted when thinking disabled, got %q", *got.ReasoningEffort)
	}
	if got.ExtraBody.Thinking.Type != "disabled" {
		t.Fatalf("expected deepseek thinking type disabled, got %q", got.ExtraBody.Thinking.Type)
	}
}

func assertJSONEncodedString(t *testing.T, raw json.RawMessage, want string) {
	t.Helper()

	var got string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("expected JSON string %q, got %s (err=%v)", want, string(raw), err)
	}
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func loPtr[T any](v T) *T {
	return &v
}

func TestChatOutboundTransformResponse_NormalizesReasoningUsage(t *testing.T) {
	outbound := &ChatOutbound{}
	response := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl-1",
			"object":"chat.completion",
			"created":1,
			"model":"mimo-v2.5-pro",
			"choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"stop"}],
			"usage":{
				"prompt_tokens":257,
				"completion_tokens":10,
				"total_tokens":260,
				"prompt_tokens_details":{"cached_tokens":192},
				"completion_tokens_details":{"reasoning_tokens":49}
			}
		}`)),
	}

	got, err := outbound.TransformResponse(context.Background(), response)
	if err != nil {
		t.Fatalf("TransformResponse() error = %v", err)
	}
	if got.Usage == nil || got.Usage.CompletionTokensDetails == nil {
		t.Fatal("expected usage with completion token details")
	}
	if got.Usage.CompletionTokens != 49 {
		t.Fatalf("completion_tokens = %d, want 49", got.Usage.CompletionTokens)
	}
	if got.Usage.TotalTokens != 306 {
		t.Fatalf("total_tokens = %d, want 306", got.Usage.TotalTokens)
	}
}

func TestChatOutboundTransformRequest_MimoPreservesReasoningContentForHistoricalToolCalls(t *testing.T) {
	outbound := &ChatOutbound{}
	reasoningContent := "history reasoning for mimo tool call"
	request := &model.InternalLLMRequest{
		Model: "mimo-v2.5-pro",
		Messages: []model.Message{
			{
				Role:    "assistant",
				Content: model.MessageContent{},
				ToolCalls: []model.ToolCall{{
					ID:   "call_mimo_history_1",
					Type: "function",
					Function: model.FunctionCall{
						Name:      "lookup_weather",
						Arguments: `{"city":"Beijing"}`,
					},
				}},
				ReasoningContent: &reasoningContent,
			},
			{
				Role:       "tool",
				ToolCallID: loPtr("call_mimo_history_1"),
				Content:    model.MessageContent{Content: loPtr("Sunny 25°C")},
			},
			{
				Role:    "user",
				Content: model.MessageContent{Content: loPtr("How about Shanghai?")},
			},
		},
	}

	httpReq, err := outbound.TransformRequest(context.Background(), request, "https://api.xiaomimimo.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var got struct {
		Messages []struct {
			Role             string           `json:"role"`
			ReasoningContent *string          `json:"reasoning_content,omitempty"`
			ToolCalls        []model.ToolCall `json:"tool_calls,omitempty"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal outbound body: %v", err)
	}

	if len(got.Messages) < 1 {
		t.Fatalf("expected at least one message, got %d", len(got.Messages))
	}
	assistant := got.Messages[0]
	if assistant.Role != "assistant" {
		t.Fatalf("expected first message role assistant, got %q", assistant.Role)
	}
	if len(assistant.ToolCalls) != 1 {
		t.Fatalf("expected assistant tool_calls to be preserved, got %d", len(assistant.ToolCalls))
	}
	if assistant.ReasoningContent == nil || *assistant.ReasoningContent != reasoningContent {
		t.Fatalf("expected reasoning_content %q to be preserved for mimo historical tool call, got %#v", reasoningContent, assistant.ReasoningContent)
	}
}

func TestChatOutboundTransformResponse_ParsesMimoExtendedFields(t *testing.T) {
	outbound := &ChatOutbound{}
	response := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl-mimo-1",
			"object":"chat.completion",
			"created":1,
			"model":"mimo-v2.5-pro",
			"choices":[{
				"index":0,
				"message":{
					"role":"assistant",
					"content":"hello",
					"reasoning_content":"thinking",
					"error_message":"",
					"final_text_preview":"hello preview",
					"annotations":[{
						"type":"web_search",
						"url":"https://example.com",
						"title":"Example",
						"summary":"summary",
						"site_name":"Example Site",
						"logo_url":"https://example.com/logo.png",
						"publish_time":"2025-12-16T00:00:00Z"
					}]
				},
				"finish_reason":"stop"
			}],
			"usage":{
				"prompt_tokens":100,
				"completion_tokens":20,
				"total_tokens":120,
				"prompt_tokens_details":{"cached_tokens":5,"audio_tokens":1,"image_tokens":2,"video_tokens":3},
				"completion_tokens_details":{"reasoning_tokens":7},
				"web_search_usage":{"tool_usage":2,"page_usage":8}
			}
		}`)),
	}

	got, err := outbound.TransformResponse(context.Background(), response)
	if err != nil {
		t.Fatalf("TransformResponse() error = %v", err)
	}
	if len(got.Choices) != 1 || got.Choices[0].Message == nil {
		t.Fatal("expected one parsed choice message")
	}
	msg := got.Choices[0].Message
	if msg.FinalTextPreview != "hello preview" {
		t.Fatalf("final_text_preview = %q, want %q", msg.FinalTextPreview, "hello preview")
	}
	if len(msg.Annotations) != 1 || msg.Annotations[0].URL != "https://example.com" {
		t.Fatalf("annotations not parsed as expected: %+v", msg.Annotations)
	}
	if got.Usage == nil || got.Usage.WebSearchUsage == nil {
		t.Fatal("expected web_search_usage to be parsed")
	}
	if got.Usage.WebSearchUsage.ToolUsage != 2 || got.Usage.WebSearchUsage.PageUsage != 8 {
		t.Fatalf("web_search_usage = %+v, want tool=2 page=8", got.Usage.WebSearchUsage)
	}
	if got.Usage.PromptTokensDetails == nil || got.Usage.PromptTokensDetails.ImageTokens != 2 || got.Usage.PromptTokensDetails.VideoTokens != 3 {
		t.Fatalf("prompt_tokens_details not parsed as expected: %+v", got.Usage.PromptTokensDetails)
	}
}
