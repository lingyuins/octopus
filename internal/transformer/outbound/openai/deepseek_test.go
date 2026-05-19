package openai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

func TestDeepSeekReasoningContentPreserved(t *testing.T) {
	reasoningContent := "用户想了解AI圈的最新消息。今天是2026年5月4日，我需要搜索最近发生的AI相关新闻。"
	toolCallID := "call_00_EmzB0Ri92bbrxmyELHhu5764"
	toolCallID2 := "call_01_D9r4UDTEY9l8W61fL8Ou0341"
	toolCallID3 := "call_02_OiDUbhYi0MP5USGQbI2z9118"

	request := &model.InternalLLMRequest{
		Model: "deepseek-v4-pro-max",
		Messages: []model.Message{
			{
				Role: "user",
				Content: model.MessageContent{
					Content: strPtr("最近ai圈最新的消息是什么？"),
				},
			},
			{
				Role:             "assistant",
				ReasoningContent: &reasoningContent,
			},
			{
				Role: "assistant",
				ToolCalls: []model.ToolCall{
					{
						ID:   toolCallID,
						Type: "function",
						Function: model.FunctionCall{
							Name:      "search_web",
							Arguments: `{"query": "2026年5月 人工智能 最新消息"}`,
						},
					},
				},
			},
			{
				Role: "tool",
				Content: model.MessageContent{
					Content: strPtr(`{"answer":null,"items":[...]}`),
				},
				ToolCallID: &toolCallID,
			},
			{
				Role: "assistant",
				ToolCalls: []model.ToolCall{
					{
						ID:   toolCallID2,
						Type: "function",
						Function: model.FunctionCall{
							Name:      "search_web",
							Arguments: `{"query": "AI 新闻 2026年5月"}`,
						},
					},
				},
			},
			{
				Role: "tool",
				Content: model.MessageContent{
					Content: strPtr(`{"answer":null,"items":[...]}`),
				},
				ToolCallID: &toolCallID2,
			},
			{
				Role: "assistant",
				ToolCalls: []model.ToolCall{
					{
						ID:   toolCallID3,
						Type: "function",
						Function: model.FunctionCall{
							Name:      "search_web",
							Arguments: `{"query": "2026-05 AI 突破 进展"}`,
						},
					},
				},
			},
		},
		Stream: boolPtr(true),
		TransformerMetadata: map[string]string{
			model.TransformerMetadataGroupEndpointType: "deepseek",
		},
	}

	baseURL := "https://api.deepseek.com/v1"

	compatRequest := cloneRequestForOpenAICompat(request)
	sanitizeRequestForOpenAICompat(compatRequest, baseURL, false)

	for i := range compatRequest.Messages {
		normalizeMessageForOpenAICompat(&compatRequest.Messages[i])
	}

	body, err := json.Marshal(compatRequest)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	prettyJSON, _ := json.MarshalIndent(parsed, "", "  ")
	t.Logf("Request JSON:\n%s", string(prettyJSON))

	messages, ok := parsed["messages"].([]interface{})
	if !ok {
		t.Fatal("no messages in output")
	}

	t.Logf("Total messages: %d", len(messages))

	foundReasoning := false
	firstToolCallHasReasoning := false
	for i, msgRaw := range messages {
		msg, ok := msgRaw.(map[string]interface{})
		if !ok {
			continue
		}
		role := msg["role"]
		rc, hasRC := msg["reasoning_content"]
		hasToolCalls := msg["tool_calls"] != nil

		t.Logf("  [%d] role=%v, has_reasoning_content=%v, has_tool_calls=%v",
			i, role, hasRC, hasToolCalls)

		if hasRC {
			foundReasoning = true
			rcStr, _ := rc.(string)
			if !strings.Contains(rcStr, "用户想了解") {
				t.Errorf("message %d: reasoning_content lost original text", i)
			}
			if hasToolCalls {
				firstToolCallHasReasoning = true
			}
		}
	}

	if !foundReasoning {
		t.Error("NO reasoning_content found in any message — DeepSeek will reject!")
	}

	if !firstToolCallHasReasoning {
		t.Error("First tool_calls message should have reasoning_content attached from standalone reasoning message")
	}

	// Verify extra_body does NOT force thinking:disabled when no reasoning_effort is set
	if extraBody, ok := parsed["extra_body"]; ok {
		eb, _ := extraBody.(map[string]interface{})
		if thinking, hasThinking := eb["thinking"]; hasThinking {
			th, _ := thinking.(map[string]interface{})
			if thType, ok := th["type"].(string); ok && thType == "disabled" {
				t.Error("extra_body.thinking.type should NOT be 'disabled' when reasoning_effort is not set. DeepSeek V4 defaults to thinking ON.")
			}
		}
	}
}

func TestDeepSeekReasoningEffortEnablesThinking(t *testing.T) {
	toolCallID := "call_00_test"
	request := &model.InternalLLMRequest{
		Model: "deepseek-v4-pro-max",
		Messages: []model.Message{
			{
				Role: "user",
				Content: model.MessageContent{
					Content: strPtr("hello"),
				},
			},
			{
				Role: "assistant",
				ToolCalls: []model.ToolCall{
					{
						ID:   toolCallID,
						Type: "function",
						Function: model.FunctionCall{
							Name:      "test",
							Arguments: `{}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: &toolCallID,
				Content: model.MessageContent{
					Content: strPtr("result"),
				},
			},
		},
		ReasoningEffort: "max",
		TransformerMetadata: map[string]string{
			model.TransformerMetadataGroupEndpointType: "deepseek",
		},
	}

	compatRequest := cloneRequestForOpenAICompat(request)
	sanitizeRequestForOpenAICompat(compatRequest, "https://api.deepseek.com/v1", false)

	body, err := json.Marshal(compatRequest)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	prettyJSON, _ := json.MarshalIndent(parsed, "", "  ")
	t.Logf("Request JSON:\n%s", string(prettyJSON))

	extraBody, ok := parsed["extra_body"].(map[string]interface{})
	if !ok {
		t.Fatal("extra_body should be present when reasoning_effort is set")
	}

	thinking, ok := extraBody["thinking"].(map[string]interface{})
	if !ok {
		t.Fatal("extra_body.thinking should be present when reasoning_effort is set")
	}

	if thType, ok := thinking["type"].(string); !ok || thType != "enabled" {
		t.Errorf("extra_body.thinking.type should be 'enabled' when reasoning_effort is set, got: %v", thinking["type"])
	}
}

func TestDeepSeekNoThinkingWhenNotDetected(t *testing.T) {
	// Test that reasoning is stripped when target is NOT DeepSeek
	reasoningContent := "some reasoning"
	request := &model.InternalLLMRequest{
		Model: "gpt-4o",
		Messages: []model.Message{
			{Role: "user", Content: model.MessageContent{Content: strPtr("hello")}},
			{Role: "assistant", ReasoningContent: &reasoningContent, Content: model.MessageContent{Content: strPtr("hi")}},
		},
		TransformerMetadata: map[string]string{
			model.TransformerMetadataGroupEndpointType: "chat",
		},
	}

	compatRequest := cloneRequestForOpenAICompat(request)
	sanitizeRequestForOpenAICompat(compatRequest, "https://api.openai.com/v1", false)

	body, err := json.Marshal(compatRequest)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	messages := parsed["messages"].([]interface{})
	assistantMsg := messages[1].(map[string]interface{})

	if _, hasRC := assistantMsg["reasoning_content"]; hasRC {
		t.Error("reasoning_content should be stripped for non-DeepSeek targets")
	}

	// extra_body should NOT be present for non-DeepSeek
	if _, hasExtra := parsed["extra_body"]; hasExtra {
		t.Error("extra_body should be stripped for non-DeepSeek targets")
	}
}

func TestDeepSeekDistinctReasoningPerTurn(t *testing.T) {
	reasoning1 := "reasoning 1.1: search current AI news"
	reasoning2 := "reasoning 1.2: refine search query"
	reasoning3 := "reasoning 1.3: summarize search results"

	request := &model.InternalLLMRequest{
		Model: "deepseek-v4-pro-max",
		Messages: []model.Message{
			{Role: "user", Content: model.MessageContent{Content: strPtr("latest AI news")}},
			{Role: "assistant", ReasoningContent: &reasoning1},
			{Role: "assistant", ToolCalls: []model.ToolCall{{ID: "c0", Type: "function", Function: model.FunctionCall{Name: "search", Arguments: `{}`}}}},
			{Role: "tool", ToolCallID: strPtr("c0"), Content: model.MessageContent{Content: strPtr("result0")}},
			{Role: "assistant", ReasoningContent: &reasoning2},
			{Role: "assistant", ToolCalls: []model.ToolCall{{ID: "c1", Type: "function", Function: model.FunctionCall{Name: "search", Arguments: `{}`}}}},
			{Role: "tool", ToolCallID: strPtr("c1"), Content: model.MessageContent{Content: strPtr("result1")}},
			{Role: "assistant", ReasoningContent: &reasoning3},
			{Role: "assistant", ToolCalls: []model.ToolCall{{ID: "c2", Type: "function", Function: model.FunctionCall{Name: "search", Arguments: `{}`}}}},
			{Role: "tool", ToolCallID: strPtr("c2"), Content: model.MessageContent{Content: strPtr("result2")}},
		},
		TransformerMetadata: map[string]string{
			model.TransformerMetadataGroupEndpointType: "deepseek",
		},
	}

	compatRequest := cloneRequestForOpenAICompat(request)
	sanitizeRequestForOpenAICompat(compatRequest, "https://api.deepseek.com/v1", false)
	for i := range compatRequest.Messages {
		normalizeMessageForOpenAICompat(&compatRequest.Messages[i])
	}

	body, err := json.Marshal(compatRequest)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	messages := parsed["messages"].([]interface{})
	toolCallReasonings := make([]string, 0)
	for _, msgRaw := range messages {
		msg := msgRaw.(map[string]interface{})
		if msg["tool_calls"] != nil {
			reasoningContent, _ := msg["reasoning_content"].(string)
			toolCallReasonings = append(toolCallReasonings, reasoningContent)
		}
	}

	if len(toolCallReasonings) != 3 {
		t.Fatalf("expected 3 tool_calls messages with reasoning, got %d", len(toolCallReasonings))
	}

	if toolCallReasonings[0] != reasoning1 {
		t.Errorf("tool_call[0] has wrong reasoning: %q, expected %q", toolCallReasonings[0], reasoning1)
	}
	if toolCallReasonings[1] != reasoning2 {
		t.Errorf("tool_call[1] has wrong reasoning: %q, expected %q", toolCallReasonings[1], reasoning2)
	}
	if toolCallReasonings[2] != reasoning3 {
		t.Errorf("tool_call[2] has wrong reasoning: %q, expected %q", toolCallReasonings[2], reasoning3)
	}
}

func TestMimoReasoningContentPreservedForXiaomiModel(t *testing.T) {
	reasoningContent := "need to think before tool call"
	request := &model.InternalLLMRequest{
		Model: "Xiaomi-Token-Plan",
		Messages: []model.Message{
			{Role: "user", Content: model.MessageContent{Content: strPtr("hello")}},
			{Role: "assistant", ReasoningContent: &reasoningContent},
			{Role: "assistant", ToolCalls: []model.ToolCall{{
				ID:   "call_mimo_0",
				Type: "function",
				Function: model.FunctionCall{
					Name:      "plan",
					Arguments: `{}`,
				},
			}}},
		},
		TransformerMetadata: map[string]string{
			model.TransformerMetadataGroupEndpointType: "mimo",
		},
	}

	compatRequest := cloneRequestForOpenAICompat(request)
	sanitizeRequestForOpenAICompat(compatRequest, "https://api.xiaomimimo.com/v1", true)
	for i := range compatRequest.Messages {
		normalizeMessageForOpenAICompat(&compatRequest.Messages[i])
	}

	body, err := json.Marshal(compatRequest)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	messages, ok := parsed["messages"].([]interface{})
	if !ok {
		t.Fatal("no messages in output")
	}

	foundToolCallReasoning := false
	for _, msgRaw := range messages {
		msg, ok := msgRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if msg["tool_calls"] == nil {
			continue
		}
		rc, _ := msg["reasoning_content"].(string)
		if rc == reasoningContent {
			foundToolCallReasoning = true
			break
		}
	}

	if !foundToolCallReasoning {
		t.Fatal("mimo/xiaomi tool_calls message should preserve reasoning_content")
	}
}

func TestMimoReasoningPreservedByChannelTypeEvenWhenGroupLooksChat(t *testing.T) {
	reasoningContent := "thinking for mimo tool call"
	request := &model.InternalLLMRequest{
		Model: "mimo-v2.5-pro",
		Messages: []model.Message{
			{Role: "user", Content: model.MessageContent{Content: strPtr("hello")}},
			{Role: "assistant", ReasoningContent: &reasoningContent},
			{Role: "assistant", ToolCalls: []model.ToolCall{{
				ID:   "call_mimo_chat_group",
				Type: "function",
				Function: model.FunctionCall{Name: "plan", Arguments: `{}`},
			}}},
		},
		TransformerMetadata: map[string]string{
			model.TransformerMetadataGroupEndpointType: "chat",
		},
	}

	compatRequest := cloneRequestForOpenAICompat(request)
	sanitizeRequestForOpenAICompat(compatRequest, "https://example.com/v1", true)
	for i := range compatRequest.Messages {
		normalizeMessageForOpenAICompat(&compatRequest.Messages[i])
	}

	body, err := json.Marshal(compatRequest)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	messages := parsed["messages"].([]interface{})
	foundToolCallReasoning := false
	for _, msgRaw := range messages {
		msg := msgRaw.(map[string]interface{})
		if msg["tool_calls"] == nil {
			continue
		}
		rc, _ := msg["reasoning_content"].(string)
		if rc == reasoningContent {
			foundToolCallReasoning = true
			break
		}
	}

	if !foundToolCallReasoning {
		t.Fatal("mimo channel type should preserve reasoning_content even when group endpoint type is chat")
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
