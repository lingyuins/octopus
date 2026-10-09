package openai

import (
	"context"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer"
	"github.com/lingyuins/octopus/internal/transformer/model"
)

func makeStreamChunk(id string, delta *model.Message, finishReason *string, usage *model.Usage) *model.InternalLLMResponse {
	chunk := &model.InternalLLMResponse{
		ID:     id,
		Object: "chat.completion.chunk",
		Model:  "gpt-test",
		Choices: []model.Choice{
			{
				Index: 0,
				Delta: delta,
			},
		},
	}
	if finishReason != nil {
		chunk.Choices[0].FinishReason = finishReason
	}
	chunk.Usage = usage
	return chunk
}

func textDelta(s string) *model.Message {
	return &model.Message{
		Role: "assistant",
		Content: model.MessageContent{
			Content: &s,
		},
	}
}

func reasoningDelta(s string) *model.Message {
	return &model.Message{
		Role:             "assistant",
		ReasoningContent: &s,
	}
}

func toolCallDelta(index int, id, name, args string) *model.Message {
	return &model.Message{
		Role: "assistant",
		ToolCalls: []model.ToolCall{
			{
				Index: index,
				ID:    id,
				Type:  "function",
				Function: model.FunctionCall{
					Name:      name,
					Arguments: args,
				},
			},
		},
	}
}

// collectCompletedEvents parses SSE output lines and returns decoded events.
func collectCompletedEvents(t *testing.T, output []byte) []ResponsesStreamEvent {
	t.Helper()
	if len(output) == 0 {
		return nil
	}
	var events []ResponsesStreamEvent
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}
		var ev ResponsesStreamEvent
		if err := transformer.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
			t.Fatalf("failed to unmarshal event %q: %v", line, err)
		}
		events = append(events, ev)
	}
	return events
}

func findEvent(t *testing.T, events []ResponsesStreamEvent, typ string) *ResponsesStreamEvent {
	t.Helper()
	for i := range events {
		if events[i].Type == typ {
			return &events[i]
		}
	}
	t.Fatalf("event %q not found in %d events", typ, len(events))
	return nil
}

// TestResponseInboundDeferredCompletionWithLateUsage verifies that the
// response.completed event is deferred until usage arrives after the
// finish_reason, and carries the full output snapshot.
func TestResponseInboundDeferredCompletionWithLateUsage(t *testing.T) {
	inbound := &ResponseInbound{}
	ctx := context.Background()

	out, err := inbound.TransformStream(ctx, makeStreamChunk("resp-1", textDelta("Hello"), nil, nil))
	if err != nil {
		t.Fatalf("TransformStream text delta: %v", err)
	}
	if strings.Contains(string(out), "response.completed") {
		t.Fatal("completed event emitted before finish_reason")
	}

	out, err = inbound.TransformStream(ctx, makeStreamChunk("resp-1", nil, strPtrDone("stop"), nil))
	if err != nil {
		t.Fatalf("TransformStream finish: %v", err)
	}
	if strings.Contains(string(out), "response.completed") {
		t.Fatal("completed event emitted before usage arrived (deferred completion)")
	}

	out, err = inbound.TransformStream(ctx, makeStreamChunk("resp-1", nil, nil, &model.Usage{
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
	}))
	if err != nil {
		t.Fatalf("TransformStream usage: %v", err)
	}
	if strings.Contains(string(out), "response.completed") {
		t.Fatal("usage alone must not emit completion before the protocol terminator")
	}
	out, err = inbound.TransformStream(ctx, &model.InternalLLMResponse{Object: "[DONE]"})
	if err != nil {
		t.Fatal(err)
	}

	events := collectCompletedEvents(t, out)
	completed := findEvent(t, events, "response.completed")
	if completed.Response == nil {
		t.Fatal("completed event missing response payload")
	}
	if completed.Response.Usage == nil {
		t.Fatal("completed event missing usage")
	}
	if completed.Response.Usage.InputTokens != 10 || completed.Response.Usage.OutputTokens != 5 {
		t.Fatalf("completed usage = %+v", completed.Response.Usage)
	}
	if len(completed.Response.Output) != 1 {
		t.Fatalf("completed output snapshot should have 1 message item, got %d", len(completed.Response.Output))
	}
	msg := completed.Response.Output[0]
	if msg.Type != "message" || msg.Content == nil || len(msg.Content.Items) == 0 {
		t.Fatalf("snapshot output[0] = %+v", msg)
	}
	if msg.Content.Items[0].Text == nil || *msg.Content.Items[0].Text != "Hello" {
		t.Fatalf("snapshot text = %+v", msg.Content.Items[0])
	}
}

// TestResponseInboundSyntheticDoneFinalizesWithoutUsage verifies the relay's
// synthetic [DONE] marker finalizes an unfinished (finished-but-no-usage)
// stream with the cached output snapshot, and does not duplicate completion.
func TestResponseInboundSyntheticDoneFinalizesWithoutUsage(t *testing.T) {
	inbound := &ResponseInbound{}
	ctx := context.Background()

	_, err := inbound.TransformStream(ctx, makeStreamChunk("resp-1", textDelta("Hi"), nil, nil))
	if err != nil {
		t.Fatalf("TransformStream text delta: %v", err)
	}
	out, err := inbound.TransformStream(ctx, makeStreamChunk("resp-1", nil, strPtrDone("stop"), nil))
	if err != nil {
		t.Fatalf("TransformStream finish: %v", err)
	}
	if strings.Contains(string(out), "response.completed") {
		t.Fatal("completed emitted before [DONE] without usage")
	}

	out, err = inbound.TransformStream(ctx, &model.InternalLLMResponse{Object: "[DONE]"})
	if err != nil {
		t.Fatalf("TransformStream [DONE]: %v", err)
	}
	events := collectCompletedEvents(t, out)
	completed := findEvent(t, events, "response.completed")
	if completed.Response == nil {
		t.Fatal("[DONE] finalize missing response")
	}
	if len(completed.Response.Output) != 1 {
		t.Fatalf("[DONE] snapshot should have 1 item, got %d", len(completed.Response.Output))
	}
	if completed.Response.Output[0].Content.Items[0].Text == nil || *completed.Response.Output[0].Content.Items[0].Text != "Hi" {
		t.Fatalf("[DONE] snapshot text wrong: %+v", completed.Response.Output[0])
	}
	if !strings.Contains(string(out), "data: [DONE]") {
		t.Fatal("[DONE] marker not emitted")
	}

	// A second [DONE] must not duplicate the completion.
	out, err = inbound.TransformStream(ctx, &model.InternalLLMResponse{Object: "[DONE]"})
	if err != nil {
		t.Fatalf("TransformStream second [DONE]: %v", err)
	}
	if strings.Contains(string(out), "response.completed") {
		t.Fatal("duplicate response.completed on second [DONE]")
	}
}

// TestResponseInboundFinishStatusMapping verifies finish_reason status
// preservation in the completed event.
func TestResponseInboundFinishStatusMapping(t *testing.T) {
	cases := map[string]string{
		"stop":           "completed",
		"length":         "incomplete",
		"content_filter": "incomplete",
		"error":          "failed",
		"tool_calls":     "completed",
	}
	for finish, wantStatus := range cases {
		inbound := &ResponseInbound{}
		out1, err := inbound.TransformStream(context.Background(), makeStreamChunk("r", textDelta("x"), strPtrDone(finish), nil))
		if err != nil {
			t.Fatalf("finish %q: %v", finish, err)
		}
		out2, err := inbound.TransformStream(context.Background(), &model.InternalLLMResponse{Object: "[DONE]"})
		if err != nil {
			t.Fatalf("finish %q [DONE]: %v", finish, err)
		}
		events := collectCompletedEvents(t, append(out1, out2...))
		wantType := "response.completed"
		if wantStatus == "incomplete" {
			wantType = "response.incomplete"
		} else if wantStatus == "failed" {
			wantType = "response.failed"
		}
		completed := findEvent(t, events, wantType)
		if completed.Response == nil || completed.Response.Status == nil {
			t.Fatalf("finish %q: completed response/status nil", finish)
		}
		if *completed.Response.Status != wantStatus {
			t.Fatalf("finish %q: status = %q, want %q", finish, *completed.Response.Status, wantStatus)
		}
	}
}

// TestResponseInboundSnapshotToolFirstThenText verifies the snapshot preserves
// emission order (output_index) and item IDs: a tool call emitted before text
// must appear before the text item in the completed snapshot.
func TestResponseInboundSnapshotToolFirstThenText(t *testing.T) {
	inbound := &ResponseInbound{}
	ctx := context.Background()

	// Tool call first.
	_, err := inbound.TransformStream(ctx, makeStreamChunk("r", toolCallDelta(0, "call_1", "get_weather", `{"city":`), nil, nil))
	if err != nil {
		t.Fatalf("tool delta: %v", err)
	}
	// Then text.
	_, err = inbound.TransformStream(ctx, makeStreamChunk("r", textDelta("after tool"), nil, nil))
	if err != nil {
		t.Fatalf("text delta: %v", err)
	}
	_, err = inbound.TransformStream(ctx, makeStreamChunk("r", nil, strPtrDone("stop"), &model.Usage{TotalTokens: 3}))
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	out, err := inbound.TransformStream(ctx, &model.InternalLLMResponse{Object: "[DONE]"})
	if err != nil {
		t.Fatal(err)
	}

	events := collectCompletedEvents(t, out)
	completed := findEvent(t, events, "response.completed")
	if completed.Response == nil || len(completed.Response.Output) != 2 {
		t.Fatalf("snapshot should have 2 items (tool first, then message), got %d", len(completed.Response.Output))
	}

	first := completed.Response.Output[0]
	if first.Type != "function_call" {
		t.Fatalf("snapshot[0].Type = %q, want function_call (emission order must be preserved)", first.Type)
	}
	if first.CallID != "call_1" || first.Name != "get_weather" || first.Arguments != `{"city":` {
		t.Fatalf("snapshot[0] tool call = %+v", first)
	}
	second := completed.Response.Output[1]
	if second.Type != "message" || second.Content == nil || len(second.Content.Items) == 0 {
		t.Fatalf("snapshot[1] = %+v", second)
	}
	if second.Content.Items[0].Text == nil || *second.Content.Items[0].Text != "after tool" {
		t.Fatalf("snapshot[1] text = %+v", second.Content.Items[0])
	}
}

// TestResponseInboundSnapshotPreservesItemIDs verifies the snapshot reuses the
// item IDs already emitted via output_item.added/done events.
func TestResponseInboundSnapshotPreservesItemIDs(t *testing.T) {
	inbound := &ResponseInbound{}
	ctx := context.Background()

	_, err := inbound.TransformStream(ctx, makeStreamChunk("r", reasoningDelta("think"), nil, nil))
	if err != nil {
		t.Fatalf("reasoning delta: %v", err)
	}
	// Text delta closes the reasoning item (output_item.done for reasoning
	// is emitted within this chunk) and opens the message item.
	out, err := inbound.TransformStream(ctx, makeStreamChunk("r", textDelta("answer"), nil, nil))
	if err != nil {
		t.Fatalf("text delta: %v", err)
	}
	out2, err := inbound.TransformStream(ctx, makeStreamChunk("r", nil, strPtrDone("stop"), nil))
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	out = append(out, out2...)

	// Capture item IDs from output_item.done events.
	var doneIDs []string
	for _, ev := range collectCompletedEvents(t, out) {
		if ev.Type == "response.output_item.done" && ev.Item != nil {
			doneIDs = append(doneIDs, ev.Item.ID)
		}
	}
	if len(doneIDs) != 2 {
		t.Fatalf("expected 2 output_item.done events, got %d", len(doneIDs))
	}

	out, err = inbound.TransformStream(ctx, &model.InternalLLMResponse{Object: "[DONE]"})
	if err != nil {
		t.Fatalf("[DONE]: %v", err)
	}
	events := collectCompletedEvents(t, out)
	completed := findEvent(t, events, "response.completed")
	if len(completed.Response.Output) != 2 {
		t.Fatalf("snapshot should have 2 items, got %d", len(completed.Response.Output))
	}
	for i, item := range completed.Response.Output {
		if item.ID != doneIDs[i] {
			t.Fatalf("snapshot[%d].ID = %q, want %q (must reuse emitted item ID)", i, item.ID, doneIDs[i])
		}
	}
	// Reasoning first (emitted first).
	if completed.Response.Output[0].Type != "reasoning" {
		t.Fatalf("snapshot[0].Type = %q, want reasoning", completed.Response.Output[0].Type)
	}
}

// TestResponseInboundRequestTextFormatSchema verifies the Chat->Responses
// direction preserves text.format name/schema/description/strict.
func TestResponseInboundRequestTextFormatSchema(t *testing.T) {
	body := `{
		"model": "gpt-4o",
		"input": "extract",
		"text": {
			"format": {
				"type": "json_schema",
				"name": "my_schema",
				"description": "extract result",
				"schema": {"type": "object", "properties": {"a": {"type": "string"}}},
				"strict": true
			}
		}
	}`
	inbound := &ResponseInbound{}
	req, err := inbound.TransformRequest(context.Background(), []byte(body))
	if err != nil {
		t.Fatalf("TransformRequest: %v", err)
	}
	if req.ResponseFormat == nil {
		t.Fatal("ResponseFormat lost")
	}
	js := req.ResponseFormat.JSONSchema
	if js == nil {
		t.Fatal("JSONSchema lost in Chat->Responses conversion")
	}
	if js.Name != "my_schema" {
		t.Fatalf("Name = %q", js.Name)
	}
	if js.Description != "extract result" {
		t.Fatalf("Description = %q", js.Description)
	}
	if js.Strict == nil || !*js.Strict {
		t.Fatalf("Strict = %v, want true", js.Strict)
	}
	if string(js.Schema) == "" || !strings.Contains(string(js.Schema), `"a"`) {
		t.Fatalf("Schema = %s", js.Schema)
	}
}

func strPtrDone(s string) *string { return &s }
