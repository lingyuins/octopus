package openai

import (
	"context"
	"strings"
	"testing"

	"github.com/samber/lo"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

// TestInboundResponsesInputMarshalNilItemsEmitsEmptyArray mirrors the outbound
// test: the inbound side has the same MarshalJSON shape and the same
// protocol-type constraint.
func TestInboundResponsesInputMarshalNilItemsEmitsEmptyArray(t *testing.T) {
	in := ResponsesInput{}
	out, err := in.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if string(out) != "[]" {
		t.Fatalf("nil Items must marshal as [], got %s", string(out))
	}
}

// TestConvertToResponsesAPIResponseContentFilterMapsToIncomplete verifies the
// non-streaming inbound mapping reuses the streaming helper, so the wire
// status for content_filter is "incomplete" on both paths.
func TestConvertToResponsesAPIResponseContentFilterMapsToIncomplete(t *testing.T) {
	in := &ResponseInbound{}
	resp := &model.InternalLLMResponse{
		ID:     "x",
		Object: "chat.completion",
		Model:  "m",
		Choices: []model.Choice{
			{
				Index:        0,
				Message:      &model.Message{Role: "assistant", Content: model.MessageContent{Content: lo.ToPtr("filtered")}},
				FinishReason: lo.ToPtr("content_filter"),
			},
		},
	}
	out, err := in.TransformResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("TransformResponse: %v", err)
	}
	if !strings.Contains(string(out), `"status":"incomplete"`) {
		t.Fatalf("content_filter must produce incomplete status, got %s", string(out))
	}
}

// TestResponseInboundStreamRefusalRoundTrip exercises the refusal flow on the
// chat→responses stream direction: a refusal delta on the internal chunk must
// produce response.refusal.delta and a refusal content part in the final
// snapshot.
func TestResponseInboundStreamRefusalRoundTrip(t *testing.T) {
	in := &ResponseInbound{}
	ctx := context.Background()

	refusalDelta := &model.Message{
		Role:    "assistant",
		Refusal: "cannot help",
	}
	chunk := &model.InternalLLMResponse{
		ID:     "r1",
		Object: "chat.completion.chunk",
		Model:  "m",
		Choices: []model.Choice{
			{Index: 0, Delta: refusalDelta},
		},
	}
	out, err := in.TransformStream(ctx, chunk)
	if err != nil {
		t.Fatalf("delta: %v", err)
	}
	if !strings.Contains(string(out), "response.refusal.delta") {
		t.Fatalf("expected refusal delta event, got: %s", string(out))
	}

	// Terminate the stream.
	finalChunk := &model.InternalLLMResponse{
		ID:     "r1",
		Object: "chat.completion.chunk",
		Model:  "m",
		Choices: []model.Choice{
			{Index: 0, Delta: &model.Message{}, FinishReason: lo.ToPtr("stop")},
		},
	}
	if _, err := in.TransformStream(ctx, finalChunk); err != nil {
		t.Fatalf("finish: %v", err)
	}
	doneOut, err := in.TransformStream(ctx, &model.InternalLLMResponse{Object: "[DONE]"})
	if err != nil {
		t.Fatalf("[DONE]: %v", err)
	}
	if !strings.Contains(string(doneOut), `"refusal"`) {
		t.Fatalf("expected refusal content in completed snapshot, got: %s", string(doneOut))
	}
	if !strings.Contains(string(doneOut), "cannot help") {
		t.Fatalf("expected refusal text in completed snapshot, got: %s", string(doneOut))
	}
}
