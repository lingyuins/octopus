package openai

import (
	"context"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

func TestChatInboundMarksOpenAIChatCompletionFormat(t *testing.T) {
	inbound := &ChatInbound{}

	req, err := inbound.TransformRequest(context.Background(), []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}
	if req.RawAPIFormat != model.APIFormatOpenAIChatCompletion {
		t.Fatalf("RawAPIFormat = %q, want %q", req.RawAPIFormat, model.APIFormatOpenAIChatCompletion)
	}
}
