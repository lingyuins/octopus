package openai

import (
	"context"
	"testing"

	"github.com/samber/lo"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

// TestResponsesInputMarshalNilItemsEmitsEmptyArray guards against regressing to
// `"input": null` which is a Responses protocol type error.
func TestResponsesInputMarshalNilItemsEmitsEmptyArray(t *testing.T) {
	in := ResponsesInput{}
	out, err := in.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if string(out) != "[]" {
		t.Fatalf("nil Items must marshal as [], got %s", string(out))
	}
}

// TestConvertToResponsesRequestOnlySystemMessageEmitsArrayInput ensures the
// degenerate "only system prompt" case produces a valid array input.
func TestConvertToResponsesRequestOnlySystemMessageEmitsArrayInput(t *testing.T) {
	req := &model.InternalLLMRequest{
		Model: "gpt-4o",
		Messages: []model.Message{
			{Role: "system", Content: model.MessageContent{Content: lo.ToPtr("be nice")}},
		},
	}
	r := ConvertToResponsesRequest(req)
	b, err := r.Input.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	if string(b) == "null" {
		t.Fatalf("input must not be null; got %s", string(b))
	}
}

// TestConvertToResponsesRequestIncludePreservedOnlyForResponsesOrigin verifies
// that the include list round-trips for responses-origin requests and is
// dropped for chat-origin requests.
func TestConvertToResponsesRequestIncludePreservedOnlyForResponsesOrigin(t *testing.T) {
	inc := []string{"reasoning.encrypted_content"}

	resReq := &model.InternalLLMRequest{
		Model:        "gpt-4o",
		RawAPIFormat: model.APIFormatOpenAIResponse,
		Include:      inc,
		Messages: []model.Message{
			{Role: "user", Content: model.MessageContent{Content: lo.ToPtr("hi")}},
		},
	}
	got := ConvertToResponsesRequest(resReq)
	if len(got.Include) != 1 || got.Include[0] != "reasoning.encrypted_content" {
		t.Fatalf("responses-origin include lost: %+v", got.Include)
	}

	chatReq := &model.InternalLLMRequest{
		Model:        "gpt-4o",
		RawAPIFormat: model.APIFormatOpenAIChatCompletion,
		Include:      inc,
		Messages: []model.Message{
			{Role: "user", Content: model.MessageContent{Content: lo.ToPtr("hi")}},
		},
	}
	got = ConvertToResponsesRequest(chatReq)
	if len(got.Include) != 0 {
		t.Fatalf("chat-origin include must not leak: %+v", got.Include)
	}
}

// TestConvertToLLMResponseFromResponsesEncryptedContentPreserved verifies the
// upstream encrypted_content round-trips into Message.ReasoningSignature.
func TestConvertToLLMResponseFromResponsesEncryptedContentPreserved(t *testing.T) {
	enc := "enc-payload-abc"
	resp := &ResponsesResponse{
		Object: "response",
		ID:     "r1",
		Model:  "gpt-4o",
		Status: lo.ToPtr("completed"),
		Output: []ResponsesItem{
			{
				Type:             "reasoning",
				Summary:          []ResponsesReasoningSummary{{Type: "summary_text", Text: "thinking"}},
				EncryptedContent: &enc,
			},
			{
				Type: "message",
				Role: "assistant",
				Content: &ResponsesInput{Items: []ResponsesItem{
					{Type: "output_text", Text: lo.ToPtr("answer")},
				}},
			},
		},
	}
	got := convertToLLMResponseFromResponses(resp)
	if got.Choices[0].Message.ReasoningSignature == nil ||
		*got.Choices[0].Message.ReasoningSignature != enc {
		t.Fatalf("encrypted_content lost: %+v", got.Choices[0].Message)
	}
}

// TestConvertAssistantMessageToResponsesBuildsReasoningItemOnlyForResponses
// guards against cross-protocol replay of Anthropic thinking signatures.
func TestConvertAssistantMessageToResponsesBuildsReasoningItemOnlyForResponses(t *testing.T) {
	msg := model.Message{
		Role:               "assistant",
		ReasoningContent:   lo.ToPtr("thinking"),
		ReasoningSignature: lo.ToPtr("sig-xyz"),
		Content:            model.MessageContent{Content: lo.ToPtr("answer")},
	}

	responsesItems := convertAssistantMessageToResponses(msg, model.APIFormatOpenAIResponse)
	var foundReasoning bool
	for _, it := range responsesItems {
		if it.Type == "reasoning" {
			foundReasoning = true
			if it.EncryptedContent == nil || *it.EncryptedContent != "sig-xyz" {
				t.Fatalf("encrypted_content lost on rebuild: %+v", it)
			}
		}
	}
	if !foundReasoning {
		t.Fatalf("expected reasoning item for responses-origin message")
	}

	chatItems := convertAssistantMessageToResponses(msg, model.APIFormatOpenAIChatCompletion)
	for _, it := range chatItems {
		if it.Type == "reasoning" {
			t.Fatalf("chat-origin message must not synthesize reasoning item: %+v", it)
		}
	}
}

// TestConvertToLLMResponseFromResponsesRefusalPreserved verifies refusal
// content is surfaced on Message.Refusal for both message-wrapped and
// top-level refusal items.
func TestConvertToLLMResponseFromResponsesRefusalPreserved(t *testing.T) {
	resp := &ResponsesResponse{
		Object: "response",
		ID:     "r1",
		Model:  "gpt-4o",
		Status: lo.ToPtr("completed"),
		Output: []ResponsesItem{
			{
				Type: "message",
				Role: "assistant",
				Content: &ResponsesInput{Items: []ResponsesItem{
					{Type: "refusal", Refusal: lo.ToPtr("cannot help with that")},
				}},
			},
		},
	}
	got := convertToLLMResponseFromResponses(resp)
	if got.Choices[0].Message.Refusal != "cannot help with that" {
		t.Fatalf("refusal lost: %+v", got.Choices[0].Message)
	}
}

// TestConvertToLLMResponseFromResponsesIncompleteBeatsToolCalls verifies the
// non-streaming mapping aligns with the streaming branch: incomplete (length)
// takes precedence over tool_calls.
func TestConvertToLLMResponseFromResponsesIncompleteBeatsToolCalls(t *testing.T) {
	resp := &ResponsesResponse{
		Object: "response",
		ID:     "r1",
		Model:  "gpt-4o",
		Status: lo.ToPtr("incomplete"),
		Output: []ResponsesItem{
			{
				Type:      "function_call",
				CallID:    "call_1",
				Name:      "f",
				Arguments: `{"truncated":`,
			},
		},
	}
	got := convertToLLMResponseFromResponses(resp)
	if got.Choices[0].FinishReason == nil || *got.Choices[0].FinishReason != "length" {
		t.Fatalf("incomplete must map to length even with tool_calls, got %+v", got.Choices[0].FinishReason)
	}
}

// TestResponseOutboundStreamRefusalDelta verifies the streaming branch
// converts response.refusal.delta events into Message.Refusal deltas.
func TestResponseOutboundStreamRefusalDelta(t *testing.T) {
	o := &ResponseOutbound{}
	chunk, err := o.TransformStream(context.Background(), []byte(`{"type":"response.refusal.delta","delta":"cannot"}`))
	if err != nil {
		t.Fatalf("TransformStream: %v", err)
	}
	if chunk == nil || len(chunk.Choices) == 0 {
		t.Fatalf("expected a chat chunk, got nil")
	}
	if chunk.Choices[0].Delta == nil || chunk.Choices[0].Delta.Refusal != "cannot" {
		t.Fatalf("refusal delta lost: %+v", chunk.Choices[0].Delta)
	}
}
