package anthropic

import (
	"context"
	"testing"
)

func TestTransformStream_MessageDeltaAttachesUsageToChunk(t *testing.T) {
	o := &MessageOutbound{}

	// message_start with input tokens only
	start, err := o.TransformStream(context.Background(), []byte(`{
		"type":"message_start",
		"message":{
			"id":"msg_1",
			"model":"claude-test",
			"usage":{"input_tokens":100,"output_tokens":0,"cache_read_input_tokens":20}
		}
	}`))
	if err != nil {
		t.Fatalf("message_start: %v", err)
	}
	if start == nil || start.Usage == nil || start.Usage.PromptTokens != 100 {
		t.Fatalf("message_start usage = %#v", start)
	}

	// message_delta carries output tokens — must appear on this chunk for inbound aggregation
	delta, err := o.TransformStream(context.Background(), []byte(`{
		"type":"message_delta",
		"delta":{"stop_reason":"end_turn"},
		"usage":{"output_tokens":42}
	}`))
	if err != nil {
		t.Fatalf("message_delta: %v", err)
	}
	if delta == nil || delta.Usage == nil {
		t.Fatal("message_delta must attach Usage to chunk")
	}
	if delta.Usage.PromptTokens != 100 {
		t.Fatalf("PromptTokens=%d, want 100 (merged from message_start)", delta.Usage.PromptTokens)
	}
	if delta.Usage.CompletionTokens != 42 {
		t.Fatalf("CompletionTokens=%d, want 42", delta.Usage.CompletionTokens)
	}
	if delta.Usage.PromptTokensDetails == nil || delta.Usage.PromptTokensDetails.CachedTokens != 20 {
		t.Fatalf("cached tokens not preserved: %#v", delta.Usage.PromptTokensDetails)
	}
}

func TestTransformStream_MessageStopStillCarriesUsage(t *testing.T) {
	o := &MessageOutbound{}
	_, _ = o.TransformStream(context.Background(), []byte(`{
		"type":"message_start",
		"message":{"id":"msg_1","model":"claude-test","usage":{"input_tokens":10}}
	}`))
	_, _ = o.TransformStream(context.Background(), []byte(`{
		"type":"message_delta",
		"usage":{"output_tokens":7}
	}`))
	stop, err := o.TransformStream(context.Background(), []byte(`{"type":"message_stop"}`))
	if err != nil {
		t.Fatalf("message_stop: %v", err)
	}
	if stop == nil || stop.Usage == nil {
		t.Fatal("message_stop should still carry usage")
	}
	if stop.Usage.PromptTokens != 10 || stop.Usage.CompletionTokens != 7 {
		t.Fatalf("stop usage = prompt=%d completion=%d", stop.Usage.PromptTokens, stop.Usage.CompletionTokens)
	}
}

func TestTransformStream_MessageStopMarksStreamFinished(t *testing.T) {
	o := &MessageOutbound{}
	_, err := o.TransformStream(context.Background(), []byte(`{
		"type":"message_start",
		"message":{"id":"msg_1","model":"claude-test","usage":{"input_tokens":3}}
	}`))
	if err != nil {
		t.Fatalf("message_start: %v", err)
	}
	stop, err := o.TransformStream(context.Background(), []byte(`{"type":"message_stop"}`))
	if err != nil {
		t.Fatalf("message_stop: %v", err)
	}
	if stop == nil {
		t.Fatal("message_stop returned nil")
	}
	if !stop.StreamFinished {
		t.Fatal("message_stop must set StreamFinished=true")
	}
	// Non-terminal events must not set the flag.
	other, err := o.TransformStream(context.Background(), []byte(`{
		"type":"content_block_delta",
		"index":0,
		"delta":{"type":"text_delta","text":"hi"}
	}`))
	if err != nil {
		t.Fatalf("content_block_delta: %v", err)
	}
	if other != nil && other.StreamFinished {
		t.Fatal("content_block_delta must not set StreamFinished")
	}
}
