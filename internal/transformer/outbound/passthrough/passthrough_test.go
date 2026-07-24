package passthrough

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

func TestTransformRequestOpenAIChatUsesRawBodyAndChatPath(t *testing.T) {
	raw := []byte(`{"model":"client-model","messages":[{"role":"user","content":"hello"}],"custom":true}`)
	stream := false
	req, err := (&Outbound{}).TransformRequest(context.Background(), &model.InternalLLMRequest{
		RawRequest:   raw,
		RawAPIFormat: model.APIFormatOpenAIChatCompletion,
		Stream:       &stream,
		Query:        mapQuery("beta", "1"),
	}, "https://example.com/api", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest error: %v", err)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != string(raw) {
		t.Fatalf("body = %s, want raw %s", string(body), string(raw))
	}
	if req.URL.String() != "https://example.com/api/v1/chat/completions?beta=1" {
		t.Fatalf("url = %s", req.URL.String())
	}
	if got := req.Header.Get("Authorization"); got != "Bearer sk-test" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := req.Header.Get("api-key"); got != "sk-test" {
		t.Fatalf("api-key = %q", got)
	}
}

func TestTransformRequestOpenAIResponsesUsesRawBodyAndResponsesPath(t *testing.T) {
	raw := []byte(`{"model":"client-model","input":"hello"}`)
	req, err := (&Outbound{}).TransformRequest(context.Background(), &model.InternalLLMRequest{
		RawRequest:   raw,
		RawAPIFormat: model.APIFormatOpenAIResponse,
	}, "https://example.com/v1", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest error: %v", err)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != string(raw) {
		t.Fatalf("body = %s, want raw %s", string(body), string(raw))
	}
	if req.URL.String() != "https://example.com/v1/responses" {
		t.Fatalf("url = %s", req.URL.String())
	}
}

func TestTransformRequestAnthropicUsesRawBodyAndMessagesPath(t *testing.T) {
	raw := []byte(`{"model":"client-model","messages":[{"role":"user","content":"hello"}],"max_tokens":32}`)
	stream := true
	req, err := (&Outbound{}).TransformRequest(context.Background(), &model.InternalLLMRequest{
		RawRequest:   raw,
		RawAPIFormat: model.APIFormatAnthropicMessage,
		Stream:       &stream,
		Query:        mapQuery("trace", "1"),
	}, "https://anthropic.example.com/api", "sk-ant")
	if err != nil {
		t.Fatalf("TransformRequest error: %v", err)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != string(raw) {
		t.Fatalf("body = %s, want raw %s", string(body), string(raw))
	}
	if req.URL.String() != "https://anthropic.example.com/api/messages?trace=1" {
		t.Fatalf("url = %s", req.URL.String())
	}
	if got := req.Header.Get("X-API-Key"); got != "sk-ant" {
		t.Fatalf("X-API-Key = %q", got)
	}
	if got := req.Header.Get("Accept"); got != "text/event-stream" {
		t.Fatalf("Accept = %q", got)
	}
}

func TestTransformRequestRewritesResolvedUpstreamModel(t *testing.T) {
	raw := []byte(`{"model":"group-name","messages":[{"role":"user","content":"hello"}],"max_tokens":32,"custom":true}`)
	stream := false
	req, err := (&Outbound{}).TransformRequest(context.Background(), &model.InternalLLMRequest{
		Model:        "claude-sonnet-4-20250514",
		RawRequest:   raw,
		RawAPIFormat: model.APIFormatAnthropicMessage,
		Stream:       &stream,
	}, "https://anthropic.example.com/api", "sk-ant")
	if err != nil {
		t.Fatalf("TransformRequest error: %v", err)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if got["model"] != "claude-sonnet-4-20250514" {
		t.Fatalf("model = %#v, want rewritten upstream model", got["model"])
	}
	if got["custom"] != true {
		t.Fatalf("custom field should be preserved, got %#v", got["custom"])
	}
	if req.URL.String() != "https://anthropic.example.com/api/messages" {
		t.Fatalf("url = %s", req.URL.String())
	}
}

func TestTransformRequestKeepsBodyWhenModelUnchanged(t *testing.T) {
	raw := []byte(`{"model":"client-model","messages":[{"role":"user","content":"hello"}],"custom":true}`)
	req, err := (&Outbound{}).TransformRequest(context.Background(), &model.InternalLLMRequest{
		Model:        "client-model",
		RawRequest:   raw,
		RawAPIFormat: model.APIFormatOpenAIChatCompletion,
	}, "https://example.com/api", "sk-test")
	if err != nil {
		t.Fatalf("TransformRequest error: %v", err)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != string(raw) {
		t.Fatalf("body should remain byte-identical when model is unchanged:\n got %s\nwant %s", string(body), string(raw))
	}
}

func TestTransformRequestRejectsUnsupportedFormat(t *testing.T) {
	_, err := (&Outbound{}).TransformRequest(context.Background(), &model.InternalLLMRequest{
		RawRequest:   []byte(`{"model":"x"}`),
		RawAPIFormat: model.APIFormatOpenAIEmbedding,
	}, "https://example.com", "sk-test")
	if err == nil || !strings.Contains(err.Error(), "does not support") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}

func mapQuery(key, value string) map[string][]string {
	return map[string][]string{key: []string{value}}
}
