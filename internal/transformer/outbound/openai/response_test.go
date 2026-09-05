package openai

import (
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

func TestConvertAssistantMessageToResponses_MarksToolCallsCompletedAndSkipsEmptyContent(t *testing.T) {
	empty := ""
	msg := model.Message{
		Role:    "assistant",
		Content: model.MessageContent{Content: &empty},
		ToolCalls: []model.ToolCall{
			{
				ID: "call_01_jCi6YrQJgn7qQhsWk5vD3152",
				Function: model.FunctionCall{
					Name:      "terminal",
					Arguments: `{"command":"ls"}`,
				},
			},
		},
	}

	items := convertAssistantMessageToResponses(msg)
	if len(items) != 1 {
		t.Fatalf("expected only function_call item, got %#v", items)
	}
	if items[0].Type != "function_call" {
		t.Fatalf("type = %q, want function_call", items[0].Type)
	}
	if items[0].CallID != "call_01_jCi6YrQJgn7qQhsWk5vD3152" {
		t.Fatalf("call_id = %q", items[0].CallID)
	}
	if items[0].Status == nil || *items[0].Status != "completed" {
		t.Fatalf("status = %#v, want completed", items[0].Status)
	}
}

func TestConvertAssistantMessageToResponses_KeepsNonEmptyTextMessage(t *testing.T) {
	msg := model.Message{
		Role:    "assistant",
		Content: model.MessageContent{Content: strPtr("done")},
	}
	items := convertAssistantMessageToResponses(msg)
	if len(items) != 1 || items[0].Type != "message" {
		t.Fatalf("expected one message item, got %#v", items)
	}
}

func TestConvertToolMessageToResponses_MarksOutputCompleted(t *testing.T) {
	toolCallID := "call_01_jCi6YrQJgn7qQhsWk5vD3152"
	msg := model.Message{
		Role:       "tool",
		ToolCallID: &toolCallID,
		Content:    model.MessageContent{Content: strPtr(`{"ok":true}`)},
	}
	item := convertToolMessageToResponses(msg)
	if item.Type != "function_call_output" {
		t.Fatalf("type = %q", item.Type)
	}
	if item.Status == nil || *item.Status != "completed" {
		t.Fatalf("status = %#v, want completed", item.Status)
	}
	if item.CallID != toolCallID {
		t.Fatalf("call_id = %q", item.CallID)
	}
}

func TestConvertToResponsesRequest_OmitsNoneReasoningEffort(t *testing.T) {
	req := &model.InternalLLMRequest{
		Model:           "mimo-v2.5-pro",
		ReasoningEffort: "none",
	}

	got := ConvertToResponsesRequest(req)
	if got.Reasoning != nil {
		t.Fatalf("expected reasoning to be omitted, got %#v", got.Reasoning)
	}
}

func TestConvertToResponsesRequest_PreservesValidReasoningEffort(t *testing.T) {
	req := &model.InternalLLMRequest{
		Model:           "o3",
		ReasoningEffort: "high",
	}

	got := ConvertToResponsesRequest(req)
	if got.Reasoning == nil {
		t.Fatalf("expected reasoning to be present")
	}
	if got.Reasoning.Effort != "high" {
		t.Fatalf("expected reasoning effort high, got %q", got.Reasoning.Effort)
	}
}

func TestConvertToResponsesRequest_PreservesMaxAndXHighReasoningEffort(t *testing.T) {
	for _, effort := range []string{"max", "xhigh", "minimal"} {
		req := &model.InternalLLMRequest{
			Model:           "gpt-5.5",
			ReasoningEffort: effort,
		}
		got := ConvertToResponsesRequest(req)
		if got.Reasoning == nil {
			t.Fatalf("effort %q: expected reasoning to be present", effort)
		}
		if got.Reasoning.Effort != effort {
			t.Fatalf("effort %q: got %q", effort, got.Reasoning.Effort)
		}
	}
}

func TestNormalizeOpenAICompatReasoningEffort_PreservesExtendedLevels(t *testing.T) {
	cases := map[string]string{
		"":        "",
		"none":    "",
		"NONE":    "",
		"minimal": "minimal",
		"low":     "low",
		"medium":  "medium",
		"high":    "high",
		"xhigh":   "xhigh",
		"max":     "max",
		"MAX":     "max",
		"bogus":   "",
	}
	for in, want := range cases {
		if got := normalizeOpenAICompatReasoningEffort(in); got != want {
			t.Fatalf("normalizeOpenAICompatReasoningEffort(%q)=%q, want %q", in, got, want)
		}
	}
}
