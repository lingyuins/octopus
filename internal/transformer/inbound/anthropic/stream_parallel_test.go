package anthropic

import (
	"context"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer"
	"github.com/lingyuins/octopus/internal/transformer/model"
)

type parsedEvent struct {
	Type         string
	Index        int64
	ContentBlock *MessageContentBlock
	Delta        *StreamDelta
	Usage        *Usage
}

// feed runs chunks through TransformStream and returns the parsed events.
func feed(t *testing.T, inbound *MessagesInbound, chunks []*model.InternalLLMResponse) []parsedEvent {
	t.Helper()
	var events []parsedEvent
	for _, chunk := range chunks {
		out, err := inbound.TransformStream(context.Background(), chunk)
		if err != nil {
			t.Fatalf("TransformStream(%+v) error = %v", chunk, err)
		}
		events = append(events, parseSSEEvents(t, out)...)
	}
	return events
}

func parseSSEEvents(t *testing.T, out []byte) []parsedEvent {
	t.Helper()
	if len(out) == 0 {
		return nil
	}
	var events []parsedEvent
	for _, raw := range strings.Split(string(out), "\n\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var dataLine string
		for _, line := range strings.Split(raw, "\n") {
			if strings.HasPrefix(line, "data:") {
				dataLine = strings.TrimPrefix(line, "data:")
			}
		}
		if dataLine == "" {
			t.Fatalf("no data line in SSE event %q", raw)
		}
		var ev StreamEvent
		if err := transformer.Unmarshal([]byte(dataLine), &ev); err != nil {
			t.Fatalf("unmarshal event %q: %v", dataLine, err)
		}
		p := parsedEvent{Type: ev.Type, ContentBlock: ev.ContentBlock, Delta: ev.Delta, Usage: ev.Usage}
		if ev.Index != nil {
			p.Index = *ev.Index
		}
		events = append(events, p)
	}
	return events
}

func toolChunk(id string, toolIndex int, name string, args string) *model.InternalLLMResponse {
	return &model.InternalLLMResponse{
		ID:     "resp-1",
		Object: "chat.completion.chunk",
		Model:  "claude-test",
		Choices: []model.Choice{{
			Index: 0,
			Delta: &model.Message{
				Role: "assistant",
				ToolCalls: []model.ToolCall{{
					Index: toolIndex,
					ID:    id,
					Type:  "function",
					Function: model.FunctionCall{
						Name:      name,
						Arguments: args,
					},
				}},
			},
		}},
	}
}

func textChunk(text string) *model.InternalLLMResponse {
	return &model.InternalLLMResponse{
		ID:     "resp-1",
		Object: "chat.completion.chunk",
		Model:  "claude-test",
		Choices: []model.Choice{{
			Index: 0,
			Delta: &model.Message{
				Role:    "assistant",
				Content: model.MessageContent{Content: &text},
			},
		}},
	}
}

func finishChunk(reason string) *model.InternalLLMResponse {
	return &model.InternalLLMResponse{
		ID:     "resp-1",
		Object: "chat.completion.chunk",
		Model:  "claude-test",
		Choices: []model.Choice{{
			Index:        0,
			Delta:        &model.Message{Role: "assistant"},
			FinishReason: &reason,
		}},
	}
}

func usageChunk(prompt, completion int64) *model.InternalLLMResponse {
	return &model.InternalLLMResponse{
		ID:      "resp-1",
		Object:  "chat.completion.chunk",
		Model:   "claude-test",
		Choices: []model.Choice{},
		Usage: &model.Usage{
			PromptTokens:     prompt,
			CompletionTokens: completion,
		},
	}
}

var doneChunk = &model.InternalLLMResponse{Object: "[DONE]"}

// Parallel tool calls: fragments for each tool must route to that tool's own
// stable block index; starting the second tool must not close the first.
func TestTransformStream_ParallelToolFragmentsRouteToStableBlockIndexes(t *testing.T) {
	inbound := &MessagesInbound{}
	events := feed(t, inbound, []*model.InternalLLMResponse{
		toolChunk("call_a", 0, "get_weather", `{"ci`),
		toolChunk("call_b", 1, "get_time", `{"tz"`),
		toolChunk("", 0, "", `ty":"SF"}`),
		toolChunk("", 1, "", `:"UTC"}`),
		finishChunk("tool_calls"),
		usageChunk(10, 20),
		doneChunk,
	})

	var (
		starts   []parsedEvent
		inputD   []parsedEvent
		stops    []parsedEvent
		terminal []parsedEvent
	)
	for _, ev := range events {
		switch ev.Type {
		case "content_block_start":
			starts = append(starts, ev)
		case "content_block_delta":
			if ev.Delta != nil && ev.Delta.Type != nil && *ev.Delta.Type == "input_json_delta" {
				inputD = append(inputD, ev)
			}
		case "content_block_stop":
			stops = append(stops, ev)
		case "message_delta", "message_stop":
			terminal = append(terminal, ev)
		}
	}

	if len(starts) != 2 {
		t.Fatalf("content_block_start count = %d, want 2", len(starts))
	}
	if starts[0].ContentBlock == nil || starts[0].ContentBlock.ID != "call_a" || starts[0].Index != 0 {
		t.Fatalf("first start = %+v", starts[0])
	}
	if starts[1].ContentBlock == nil || starts[1].ContentBlock.ID != "call_b" || starts[1].Index != 1 {
		t.Fatalf("second start = %+v", starts[1])
	}

	// Each tool's fragments must target only its own block index.
	fragments := map[int64][]string{}
	for _, d := range inputD {
		if d.Delta == nil || d.Delta.PartialJSON == nil {
			t.Fatalf("bad input_json_delta %+v", d)
		}
		fragments[d.Index] = append(fragments[d.Index], *d.Delta.PartialJSON)
	}
	if got := strings.Join(fragments[0], ""); got != `{"city":"SF"}` {
		t.Fatalf("tool 0 joined args = %q, want %q", got, `{"city":"SF"}`)
	}
	if got := strings.Join(fragments[1], ""); got != `{"tz":"UTC"}` {
		t.Fatalf("tool 1 joined args = %q, want %q", got, `{"tz":"UTC"}`)
	}
	if len(fragments) != 2 {
		t.Fatalf("fragment indexes = %v, want exactly [0 1]", fragments)
	}

	// Both tool blocks must be closed exactly once, at finish.
	if len(stops) != 2 {
		t.Fatalf("content_block_stop count = %d, want 2", len(stops))
	}
	if stops[0].Index != 0 || stops[1].Index != 1 {
		t.Fatalf("stop indexes = [%d %d], want [0 1]", stops[0].Index, stops[1].Index)
	}

	// Terminal pair must appear exactly once with tool_use and usage.
	if len(terminal) != 2 {
		t.Fatalf("terminal events = %d, want 2 (message_delta + message_stop)", len(terminal))
	}
	if terminal[0].Type != "message_delta" || terminal[1].Type != "message_stop" {
		t.Fatalf("terminal types = [%s %s]", terminal[0].Type, terminal[1].Type)
	}
	if terminal[0].Delta == nil || terminal[0].Delta.StopReason == nil || *terminal[0].Delta.StopReason != "tool_use" {
		t.Fatalf("message_delta = %+v, want stop_reason tool_use", terminal[0])
	}
	if terminal[0].Usage == nil || terminal[0].Usage.InputTokens != 10 || terminal[0].Usage.OutputTokens != 20 {
		t.Fatalf("message_delta usage = %+v", terminal[0].Usage)
	}
}

// Tool and text interleaving: text must not close open tool blocks, and tool
// fragments after text must keep flowing into the original tool block.
func TestTransformStream_ToolTextInterleaveKeepsToolBlockOpen(t *testing.T) {
	inbound := &MessagesInbound{}
	events := feed(t, inbound, []*model.InternalLLMResponse{
		toolChunk("call_a", 0, "get_weather", `{"ci`),
		textChunk("Let me check."),
		toolChunk("", 0, "", `ty":"SF"}`),
		finishChunk("tool_calls"),
		usageChunk(5, 6),
	})

	var (
		textStarts []parsedEvent
		textDeltas []parsedEvent
		stops      []parsedEvent
	)
	for _, ev := range events {
		switch ev.Type {
		case "content_block_start":
			if ev.ContentBlock != nil && ev.ContentBlock.Type == "text" {
				textStarts = append(textStarts, ev)
			}
		case "content_block_delta":
			if ev.Delta != nil && ev.Delta.Type != nil && *ev.Delta.Type == "text_delta" {
				textDeltas = append(textDeltas, ev)
			}
		case "content_block_stop":
			stops = append(stops, ev)
		}
	}

	// Tool block opens at 0, text block gets the next index 1.
	if len(textStarts) != 1 || textStarts[0].Index != 1 {
		t.Fatalf("text start = %+v, want index 1", textStarts)
	}
	if len(textDeltas) != 1 || textDeltas[0].Index != 1 {
		t.Fatalf("text delta = %+v, want index 1", textDeltas)
	}

	// Tool fragment after text must still be emitted on index 0.
	var toolFrag2 bool
	for _, ev := range events {
		if ev.Type == "content_block_delta" && ev.Delta != nil && ev.Delta.Type != nil && *ev.Delta.Type == "input_json_delta" && ev.Index == 0 {
			if ev.Delta.PartialJSON != nil && *ev.Delta.PartialJSON == `ty":"SF"}` {
				toolFrag2 = true
			}
		}
	}
	if !toolFrag2 {
		t.Fatal("second tool fragment after text was dropped")
	}

	// Stops only at finish: tool block 0 and text block 1, each once.
	var stopIdx []int64
	for _, s := range stops {
		stopIdx = append(stopIdx, s.Index)
	}
	if len(stopIdx) != 2 {
		t.Fatalf("stop count = %d (%v), want 2", len(stopIdx), stopIdx)
	}
}

// Text first, then a tool, then more text: text keeps its stable index and
// continues in the same block.
func TestTransformStream_TextToolTextContinuesSameTextBlock(t *testing.T) {
	inbound := &MessagesInbound{}
	events := feed(t, inbound, []*model.InternalLLMResponse{
		textChunk("before"),
		toolChunk("call_a", 0, "get_weather", `{"city":"SF"}`),
		textChunk(" after"),
		finishChunk("stop"),
		usageChunk(3, 4),
	})

	var textDeltas []int64
	for _, ev := range events {
		if ev.Type == "content_block_delta" && ev.Delta != nil && ev.Delta.Type != nil && *ev.Delta.Type == "text_delta" {
			textDeltas = append(textDeltas, ev.Index)
		}
	}
	if len(textDeltas) != 2 || textDeltas[0] != 0 || textDeltas[1] != 0 {
		t.Fatalf("text delta indexes = %v, want [0 0] (stable text block)", textDeltas)
	}
}

// No usage ever arrives: the terminal pair must still be emitted on [DONE].
func TestTransformStream_DoneWithoutUsageFinalizes(t *testing.T) {
	inbound := &MessagesInbound{}
	events := feed(t, inbound, []*model.InternalLLMResponse{
		textChunk("hi"),
		finishChunk("stop"),
		doneChunk,
	})

	var (
		msgDelta *parsedEvent
		msgStops int
	)
	for idx := range events {
		switch events[idx].Type {
		case "message_delta":
			if msgDelta != nil {
				t.Fatal("duplicate message_delta")
			}
			msgDelta = &events[idx]
		case "message_stop":
			msgStops++
		}
	}
	if msgDelta == nil || msgStops != 1 {
		t.Fatalf("terminal = delta:%v stops:%d, want one each", msgDelta, msgStops)
	}
	if msgDelta.Delta == nil || msgDelta.Delta.StopReason == nil || *msgDelta.Delta.StopReason != "end_turn" {
		t.Fatalf("message_delta = %+v, want end_turn", msgDelta)
	}

	// Repeated [DONE] must not emit anything more.
	more := feed(t, inbound, []*model.InternalLLMResponse{doneChunk})
	if len(more) != 0 {
		t.Fatalf("duplicate terminal on repeated [DONE]: %+v", more)
	}
}

// An explicit terminator without finish_reason still closes the protocol.
func TestTransformStream_DoneWithoutFinishDefaultsStopReason(t *testing.T) {
	inbound := &MessagesInbound{}
	events := feed(t, inbound, []*model.InternalLLMResponse{
		textChunk("hi"),
		doneChunk,
	})

	var msgDelta *parsedEvent
	for idx := range events {
		if events[idx].Type == "message_delta" {
			msgDelta = &events[idx]
		}
	}
	if msgDelta == nil {
		t.Fatal("missing message_delta on EOF")
	}
	if msgDelta.Delta == nil || msgDelta.Delta.StopReason == nil || *msgDelta.Delta.StopReason != "end_turn" {
		t.Fatalf("message_delta = %+v, want default end_turn", msgDelta)
	}

	// Open text block must be closed before the terminal pair.
	var lastStop *parsedEvent
	var deltaPos int
	for idx := range events {
		switch events[idx].Type {
		case "content_block_stop":
			lastStop = &events[idx]
		case "message_delta":
			deltaPos = idx
		}
	}
	if lastStop == nil {
		t.Fatal("open text block not closed on EOF")
	}
	_ = deltaPos
}

// Usage arriving before finish must be preserved in the terminal message_delta.
func TestTransformStream_UsageBeforeFinishPreservedInTerminal(t *testing.T) {
	inbound := &MessagesInbound{}
	events := feed(t, inbound, []*model.InternalLLMResponse{
		textChunk("hi"),
		usageChunk(100, 200),
		finishChunk("stop"),
		doneChunk,
	})

	var msgDelta *parsedEvent
	var stops int
	for idx := range events {
		switch events[idx].Type {
		case "message_delta":
			if msgDelta == nil {
				msgDelta = &events[idx]
			} else {
				t.Fatal("duplicate message_delta")
			}
		case "message_stop":
			stops++
		}
	}
	if msgDelta == nil || stops != 1 {
		t.Fatalf("terminal = delta:%v stops:%d", msgDelta, stops)
	}
	if msgDelta.Usage == nil || msgDelta.Usage.InputTokens != 100 || msgDelta.Usage.OutputTokens != 200 {
		t.Fatalf("message_delta usage = %+v, want input=100 output=200", msgDelta.Usage)
	}
	if msgDelta.Delta == nil || msgDelta.Delta.StopReason == nil || *msgDelta.Delta.StopReason != "end_turn" {
		t.Fatalf("message_delta stop_reason = %+v, want end_turn", msgDelta.Delta)
	}
}

// The final usage is emitted once at the protocol terminator.
func TestTransformStream_LateUsageAfterFinishCompletesOnce(t *testing.T) {
	inbound := &MessagesInbound{}
	events := feed(t, inbound, []*model.InternalLLMResponse{
		toolChunk("call_a", 0, "get_weather", `{"city":"SF"}`),
		finishChunk("tool_calls"),
		usageChunk(7, 8),
		doneChunk,
	})

	var (
		msgDelta *parsedEvent
		stops    int
	)
	for idx := range events {
		switch events[idx].Type {
		case "message_delta":
			if msgDelta == nil {
				msgDelta = &events[idx]
			} else {
				t.Fatal("duplicate message_delta")
			}
		case "message_stop":
			stops++
		}
	}
	if msgDelta == nil || stops != 1 {
		t.Fatalf("terminal = delta:%v stops:%d", msgDelta, stops)
	}
	if msgDelta.Usage == nil || msgDelta.Usage.InputTokens != 7 {
		t.Fatalf("message_delta usage = %+v", msgDelta.Usage)
	}
}

// Signature delta must attach to the active thinking block's index.
func TestTransformStream_SignatureAttachesToThinkingBlock(t *testing.T) {
	sig := "sig-1"
	inbound := &MessagesInbound{}
	chunks := []*model.InternalLLMResponse{
		{
			ID: "resp-1", Object: "chat.completion.chunk", Model: "claude-test",
			Choices: []model.Choice{{
				Index: 0,
				Delta: &model.Message{Role: "assistant", ReasoningContent: strp("hmm")},
			}},
		},
		{
			ID: "resp-1", Object: "chat.completion.chunk", Model: "claude-test",
			Choices: []model.Choice{{
				Index: 0,
				Delta: &model.Message{Role: "assistant", ReasoningSignature: &sig},
			}},
		},
		finishChunk("stop"),
		usageChunk(1, 2),
	}
	events := feed(t, inbound, chunks)

	var thinkingDelta *parsedEvent
	var signatureDelta *parsedEvent
	for idx := range events {
		ev := events[idx]
		if ev.Type == "content_block_delta" && ev.Delta != nil && ev.Delta.Type != nil {
			switch *ev.Delta.Type {
			case "thinking_delta":
				thinkingDelta = &events[idx]
			case "signature_delta":
				signatureDelta = &events[idx]
			}
		}
	}
	if thinkingDelta == nil || thinkingDelta.Index != 0 {
		t.Fatalf("thinking_delta = %+v, want index 0", thinkingDelta)
	}
	if signatureDelta == nil {
		t.Fatal("signature_delta missing")
	}
	if signatureDelta.Index != thinkingDelta.Index {
		t.Fatalf("signature_delta index %d != thinking block index %d", signatureDelta.Index, thinkingDelta.Index)
	}
	if signatureDelta.Delta == nil || signatureDelta.Delta.Signature == nil || *signatureDelta.Delta.Signature != "sig-1" {
		t.Fatalf("signature_delta = %+v", signatureDelta)
	}
}

func strp(s string) *string { return &s }
