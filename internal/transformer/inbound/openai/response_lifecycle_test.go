package openai

import (
	"context"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

func TestResponseInboundParallelToolLifecycle(t *testing.T) {
	for _, interleave := range []bool{false, true} {
		name := "parallel"
		if interleave {
			name = "text_interleave"
		}
		t.Run(name, func(t *testing.T) {
			adapter := &ResponseInbound{}
			chunks := []*model.InternalLLMResponse{
				makeStreamChunk("r", &model.Message{ToolCalls: []model.ToolCall{
					{Index: 0, ID: "call_a", Type: "function", Function: model.FunctionCall{Name: "weather", Arguments: `{"city":`}},
					{Index: 1, ID: "call_b", Type: "function", Function: model.FunctionCall{Name: "time", Arguments: `{"tz":`}},
				}}, nil, nil),
			}
			if interleave {
				chunks = append(chunks, makeStreamChunk("r", textDelta("checking"), nil, nil))
			}
			chunks = append(chunks,
				makeStreamChunk("r", toolCallDelta(0, "", "", `"Paris"}`), nil, nil),
				makeStreamChunk("r", toolCallDelta(1, "", "", `"UTC"}`), nil, nil),
				makeStreamChunk("r", nil, strPtrDone("tool_calls"), nil),
				makeStreamChunk("r", nil, nil, &model.Usage{TotalTokens: 15}),
				&model.InternalLLMResponse{Object: "[DONE]"},
			)
			open := make(map[int]string)
			indices := make(map[string]int)
			arguments := make(map[string]string)
			stops := make(map[string]int)
			var completed *ResponsesResponse
			for _, chunk := range chunks {
				wire, err := adapter.TransformStream(context.Background(), chunk)
				if err != nil {
					t.Fatal(err)
				}
				for _, event := range collectCompletedEvents(t, wire) {
					switch event.Type {
					case "response.output_item.added":
						if event.OutputIndex == nil || event.Item == nil {
							t.Fatalf("invalid added event: %+v", event)
						}
						if _, exists := open[*event.OutputIndex]; exists {
							t.Fatalf("output index reused: %d", *event.OutputIndex)
						}
						open[*event.OutputIndex] = event.Item.ID
						indices[event.Item.ID] = *event.OutputIndex
					case "response.function_call_arguments.delta":
						if event.ItemID == nil || event.OutputIndex == nil || open[*event.OutputIndex] != *event.ItemID {
							t.Fatalf("delta does not target its open tool: %+v, open=%v", event, open)
						}
						arguments[*event.ItemID] += event.Delta
					case "response.output_item.done":
						if event.Item == nil || event.OutputIndex == nil || open[*event.OutputIndex] != event.Item.ID {
							t.Fatalf("done does not match an open item: %+v", event)
						}
						stops[event.Item.ID]++
						delete(open, *event.OutputIndex)
					case "response.completed":
						completed = event.Response
					}
				}
			}
			if len(open) != 0 || completed == nil {
				t.Fatalf("unfinished stream: open=%v completed=%v", open, completed)
			}
			want := map[string]string{"call_a": `{"city":"Paris"}`, "call_b": `{"tz":"UTC"}`}
			for id, args := range want {
				if arguments[id] != args || stops[id] != 1 {
					t.Fatalf("tool %s args=%q stops=%d", id, arguments[id], stops[id])
				}
			}
			for outputIndex, item := range completed.Output {
				if indices[item.ID] != outputIndex {
					t.Fatalf("snapshot item %s index=%d, emitted=%d", item.ID, outputIndex, indices[item.ID])
				}
				if item.Type == "function_call" && item.Arguments != want[item.CallID] {
					t.Fatalf("snapshot arguments truncated: %+v", item)
				}
			}
		})
	}
}

func TestResponseInboundGeneratedToolIDsRemainStable(t *testing.T) {
	adapter := &ResponseInbound{}
	chunks := []*model.InternalLLMResponse{
		makeStreamChunk("r", toolCallDelta(0, "", "first", "{"), nil, nil),
		makeStreamChunk("r", toolCallDelta(1, "", "second", "{"), nil, nil),
		makeStreamChunk("r", toolCallDelta(0, "", "", "}"), nil, nil),
		makeStreamChunk("r", toolCallDelta(1, "", "", "}"), nil, nil),
		makeStreamChunk("r", nil, strPtrDone("tool_calls"), nil),
		{Object: "[DONE]"},
	}
	ids := make(map[int]string)
	for _, chunk := range chunks {
		wire, err := adapter.TransformStream(context.Background(), chunk)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range collectCompletedEvents(t, wire) {
			if event.OutputIndex == nil {
				continue
			}
			idx := *event.OutputIndex
			switch event.Type {
			case "response.output_item.added":
				ids[idx] = event.Item.ID
			case "response.function_call_arguments.delta", "response.function_call_arguments.done":
				if event.ItemID == nil || *event.ItemID != ids[idx] {
					t.Fatalf("generated ID changed: %+v, ids=%v", event, ids)
				}
			}
		}
	}
	if ids[0] == "" || ids[1] == "" || ids[0] == ids[1] {
		t.Fatalf("generated tool IDs not distinct: %v", ids)
	}
}

func TestResponseInboundUsesFinalUsageAtTerminator(t *testing.T) {
	adapter := &ResponseInbound{}
	chunks := []*model.InternalLLMResponse{
		makeStreamChunk("r", textDelta("hello"), nil, &model.Usage{PromptTokens: 10, CompletionTokens: 1, TotalTokens: 11}),
		makeStreamChunk("r", nil, strPtrDone("stop"), nil),
		makeStreamChunk("r", nil, nil, &model.Usage{PromptTokens: 10, CompletionTokens: 3, TotalTokens: 13}),
		makeStreamChunk("r", nil, nil, &model.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}),
	}
	for _, chunk := range chunks {
		wire, err := adapter.TransformStream(context.Background(), chunk)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(wire), "response.completed") {
			t.Fatal("completion emitted before final usage was known")
		}
	}
	wire, err := adapter.TransformStream(context.Background(), &model.InternalLLMResponse{Object: "[DONE]"})
	if err != nil {
		t.Fatal(err)
	}
	completed := findEvent(t, collectCompletedEvents(t, wire), "response.completed")
	if completed.Response.Usage == nil || completed.Response.Usage.TotalTokens != 15 {
		t.Fatalf("stale final usage: %+v", completed.Response.Usage)
	}
	wire, err = adapter.TransformStream(context.Background(), &model.InternalLLMResponse{Object: "[DONE]"})
	if err != nil || len(wire) != 0 {
		t.Fatalf("duplicate terminator: err=%v wire=%s", err, wire)
	}
}
