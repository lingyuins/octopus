package openai

import (
	"context"
	"reflect"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

func TestResponseInboundMixedRefusalParts(t *testing.T) {
	tests := []struct {
		name  string
		types []string
		texts []string
	}{
		{name: "refusal_to_text", types: []string{"refusal", "output_text"}, texts: []string{"R1", "T1"}},
		{name: "alternating", types: []string{"output_text", "refusal", "output_text", "refusal"}, texts: []string{"T1", "R1", "T2", "R2"}},
		{name: "refusal_first_alternating", types: []string{"refusal", "output_text", "refusal", "output_text"}, texts: []string{"R1", "T1", "R2", "T2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &ResponseInbound{}
			var events []ResponsesStreamEvent
			feed := func(chunk *model.InternalLLMResponse) {
				t.Helper()
				data, err := adapter.TransformStream(context.Background(), chunk)
				if err != nil {
					t.Fatal(err)
				}
				events = append(events, collectCompletedEvents(t, data)...)
			}
			for idx, typ := range tt.types {
				// Multiple deltas per part must accumulate locally, not across switches.
				for _, fragment := range []string{tt.texts[idx][:1], tt.texts[idx][1:]} {
					delta := textDelta(fragment)
					if typ == "refusal" {
						delta = &model.Message{Role: "assistant", Refusal: fragment}
					}
					feed(makeStreamChunk("mixed", delta, nil, nil))
				}
			}
			feed(makeStreamChunk("mixed", nil, strPtrDone("stop"), nil))
			feed(&model.InternalLLMResponse{Object: "[DONE]"})

			added := 0
			done := 0
			textDone := 0
			partDeltas := make(map[int]string)
			var closedParts []ResponsesItem
			for _, event := range events {
				switch event.Type {
				case "response.content_part.added":
					if event.ContentIndex == nil || *event.ContentIndex != added || event.Part.Type != tt.types[added] {
						t.Fatalf("part %d has inconsistent added event: %+v", added, event)
					}
					added++
				case "response.output_text.delta", "response.refusal.delta":
					if event.ContentIndex == nil || *event.ContentIndex != added-1 || done != added-1 {
						t.Fatalf("delta targets a closed or wrong part: %+v", event)
					}
					partDeltas[*event.ContentIndex] += event.Delta
				case "response.output_text.done", "response.refusal.done":
					if event.ContentIndex == nil || *event.ContentIndex != textDone {
						t.Fatalf("text/refusal done has wrong index: %+v", event)
					}
					value := event.Text
					if tt.types[textDone] == "refusal" {
						value = event.Refusal
					}
					if value != tt.texts[textDone] {
						t.Fatalf("part %d done = %q, want %q", textDone, value, tt.texts[textDone])
					}
					textDone++
				case "response.content_part.done":
					if event.ContentIndex == nil || *event.ContentIndex != done || event.Part.Type != tt.types[done] {
						t.Fatalf("part %d has inconsistent done event: %+v", done, event)
					}
					value := event.Part.Text
					part := ResponsesItem{Type: event.Part.Type, Text: value}
					if event.Part.Type == "refusal" {
						value = event.Part.Refusal
						part.Refusal = value
					}
					if value == nil || *value != tt.texts[done] || partDeltas[done] != *value {
						t.Fatalf("part %d delta/done mismatch: %+v, deltas=%q", done, event.Part, partDeltas[done])
					}
					closedParts = append(closedParts, part)
					done++
				}
			}
			if added != len(tt.types) || done != added || textDone != added {
				t.Fatalf("incomplete part lifecycle: added=%d done=%d textDone=%d", added, done, textDone)
			}
			itemDone := findEvent(t, events, "response.output_item.done")
			completed := findEvent(t, events, "response.completed")
			if !reflect.DeepEqual(itemDone.Item.Content.Items, closedParts) ||
				len(completed.Response.Output) != 1 ||
				!reflect.DeepEqual(completed.Response.Output[0].Content.Items, closedParts) {
				t.Fatalf("snapshot differs from ordered closed parts: item=%+v final=%+v", itemDone.Item, completed.Response.Output)
			}
		})
	}
}

func TestResponseInboundMixedPartsResetBetweenMessageItems(t *testing.T) {
	adapter := &ResponseInbound{}
	var events []ResponsesStreamEvent
	for _, delta := range []*model.Message{
		{Refusal: "R1"}, textDelta("T1"), toolCallDelta(0, "call", "f", "{}"), textDelta("T2"), {Refusal: "R2"},
	} {
		data, err := adapter.TransformStream(context.Background(), makeStreamChunk("reset", delta, nil, nil))
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, collectCompletedEvents(t, data)...)
	}
	data, err := adapter.TransformStream(context.Background(), &model.InternalLLMResponse{Object: "[DONE]"})
	if err != nil {
		t.Fatal(err)
	}
	events = append(events, collectCompletedEvents(t, data)...)
	output := findEvent(t, events, "response.completed").Response.Output
	if len(output) != 3 || output[0].Content.Items[0].Type != "refusal" ||
		output[2].Content.Items[0].Type != "output_text" || *output[2].Content.Items[0].Text != "T2" ||
		*output[2].Content.Items[1].Refusal != "R2" {
		t.Fatalf("parts leaked between message items: %+v", output)
	}
	var indexes []int
	for _, event := range events {
		if event.Type == "response.content_part.added" {
			indexes = append(indexes, *event.ContentIndex)
		}
	}
	if !reflect.DeepEqual(indexes, []int{0, 1, 0, 1}) {
		t.Fatalf("part indexes did not reset per message: %v", indexes)
	}
}
