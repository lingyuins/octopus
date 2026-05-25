package openai

import (
	"context"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

func TestBuildOpenAIUpstreamURL(t *testing.T) {
	tests := []struct {
		name         string
		baseURL      string
		endpointPath string
		want         string
	}{
		{name: "base url already includes v1 chat path", baseURL: "https://api.example.com/v1", endpointPath: "/v1/chat/completions", want: "https://api.example.com/v1/chat/completions"},
		{name: "nested base path already includes v1 responses path", baseURL: "https://api.example.com/openai/v1/", endpointPath: "/v1/responses", want: "https://api.example.com/openai/v1/responses"},
		{name: "base url without path keeps endpoint prefix", baseURL: "https://api.example.com", endpointPath: "/v1/embeddings", want: "https://api.example.com/v1/embeddings"},
		{name: "custom version root preserves provider path", baseURL: "https://open.bigmodel.cn/api/paas/v4", endpointPath: "/v1/chat/completions", want: "https://open.bigmodel.cn/api/paas/v4/chat/completions"},
		{name: "explicit custom endpoint path is preserved", baseURL: "https://open.bigmodel.cn/api/paas/v4/chat/completions", endpointPath: "/v1/chat/completions", want: "https://open.bigmodel.cn/api/paas/v4/chat/completions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildOpenAIUpstreamURL(tt.baseURL, tt.endpointPath)
			if err != nil {
				t.Fatalf("buildOpenAIUpstreamURL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("buildOpenAIUpstreamURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChatOutboundTransformRequestPreservesExplicitEndpointPath(t *testing.T) {
	outbound := &ChatOutbound{}
	request := &model.InternalLLMRequest{Model: "glm-4", Messages: []model.Message{{Role: "user", Content: model.MessageContent{Content: stringPtr("hello")}}}}
	req, err := outbound.TransformRequest(context.Background(), request, "https://open.bigmodel.cn/api/paas/v4/chat/completions", "key")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}
	if got, want := req.URL.String(), "https://open.bigmodel.cn/api/paas/v4/chat/completions"; got != want {
		t.Fatalf("req.URL = %q, want %q", got, want)
	}
}

func TestResponseOutboundTransformRequestKeepsSingleV1Prefix(t *testing.T) {
	outbound := &ResponseOutbound{}
	request := &model.InternalLLMRequest{Model: "gpt-4.1"}
	req, err := outbound.TransformRequest(context.Background(), request, "https://api.example.com/openai/v1", "key")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}
	if got, want := req.URL.String(), "https://api.example.com/openai/v1/responses"; got != want {
		t.Fatalf("req.URL = %q, want %q", got, want)
	}
}

func TestEmbeddingOutboundTransformRequestKeepsSingleV1Prefix(t *testing.T) {
	outbound := &EmbeddingOutbound{}
	request := &model.InternalLLMRequest{Model: "text-embedding-3-small", EmbeddingInput: &model.EmbeddingInput{Single: stringPtr("hello")}}
	req, err := outbound.TransformRequest(context.Background(), request, "https://api.example.com/v1", "key")
	if err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}
	if got, want := req.URL.String(), "https://api.example.com/v1/embeddings"; got != want {
		t.Fatalf("req.URL = %q, want %q", got, want)
	}
}

func stringPtr(v string) *string { return &v }
