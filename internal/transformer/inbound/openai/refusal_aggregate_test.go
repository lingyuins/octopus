package openai

import (
	"context"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

func TestOpenAIInboundAggregatesRefusalDeltas(t *testing.T) {
	for _, name := range []string{"chat", "responses"} {
		t.Run(name, func(t *testing.T) {
			var adapter model.Inbound = &ChatInbound{}
			if name == "responses" {
				adapter = &ResponseInbound{}
			}
			for _, fragment := range []string{"cannot", " help"} {
				_, err := adapter.TransformStream(context.Background(), makeStreamChunk("refusal", &model.Message{Refusal: fragment}, nil, nil))
				if err != nil {
					t.Fatal(err)
				}
			}
			response, err := adapter.GetInternalResponse(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if response == nil || len(response.Choices) != 1 || response.Choices[0].Message.Refusal != "cannot help" {
				t.Fatalf("refusal aggregation lost fragments: %+v", response)
			}
			response, err = adapter.GetInternalResponse(context.Background())
			if err != nil || response != nil {
				t.Fatalf("aggregation retained consumed chunks: response=%+v err=%v", response, err)
			}
		})
	}
}
