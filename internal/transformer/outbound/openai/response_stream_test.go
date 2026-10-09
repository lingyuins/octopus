package openai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer"
	"github.com/lingyuins/octopus/internal/transformer/model"
)

func TestResponseOutboundRequestTextFormatSchema(t *testing.T) {
	req := &model.InternalLLMRequest{
		Model: "gpt-4o",
		ResponseFormat: &model.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &model.ResponseFormatJSONSchema{
				Name:        "my_schema",
				Description: "extract result",
				Schema:      transformer.RawMessage(`{"type":"object","properties":{"a":{"type":"string"}}}`),
				Strict:      strictBoolPtr(true),
			},
		},
	}

	got := ConvertToResponsesRequest(req)
	if got.Text == nil || got.Text.Format == nil {
		t.Fatal("text.format lost in Responses->Chat conversion")
	}
	format := got.Text.Format
	if format.Name != "my_schema" {
		t.Fatalf("Name = %q", format.Name)
	}
	if format.Description != "extract result" {
		t.Fatalf("Description = %q", format.Description)
	}
	if format.Strict == nil || !*format.Strict {
		t.Fatalf("Strict = %v, want true", format.Strict)
	}
	if format.Schema == nil || !strings.Contains(string(format.Schema), `"a"`) {
		t.Fatalf("Schema = %s", format.Schema)
	}

	// Round-trip through the wire to ensure the fields serialize.
	body, err := transformer.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(body), `"name":"my_schema"`) || !strings.Contains(string(body), `"strict":true`) {
		t.Fatalf("serialized body missing format fields: %s", body)
	}
}

func TestResponseOutboundRequestTextFormatStrictFalsePreserved(t *testing.T) {
	req := &model.InternalLLMRequest{
		Model: "gpt-4o",
		ResponseFormat: &model.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &model.ResponseFormatJSONSchema{
				Name:   "s",
				Schema: transformer.RawMessage(`{"type":"object"}`),
				Strict: strictBoolPtr(false),
			},
		},
	}
	got := ConvertToResponsesRequest(req)
	if got.Text.Format.Strict == nil || *got.Text.Format.Strict {
		t.Fatalf("Strict = %v, want explicit false", got.Text.Format.Strict)
	}
}

func TestResponseOutboundStreamCompletedSetsStreamFinished(t *testing.T) {
	o := &ResponseOutbound{}
	resp, err := o.TransformStream(context.Background(), []byte(`{"type":"response.completed","response":{"id":"r1","status":"completed","usage":{"input_tokens":5,"output_tokens":3,"total_tokens":8}}}`))
	if err != nil {
		t.Fatalf("TransformStream: %v", err)
	}
	if !resp.StreamFinished {
		t.Fatal("response.completed must set StreamFinished")
	}
	if len(resp.Choices) != 1 || resp.Choices[0].FinishReason == nil || *resp.Choices[0].FinishReason != "stop" {
		t.Fatalf("choices = %+v", resp.Choices)
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 8 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

func TestResponseOutboundStreamCompletedWithToolCallsFinish(t *testing.T) {
	o := &ResponseOutbound{}
	// function_call item seen during the stream
	_, err := o.TransformStream(context.Background(), []byte(`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","call_id":"call_1","name":"get_weather"}}`))
	if err != nil {
		t.Fatalf("item.added: %v", err)
	}
	resp, err := o.TransformStream(context.Background(), []byte(`{"type":"response.completed","response":{"id":"r1","status":"completed"}}`))
	if err != nil {
		t.Fatalf("completed: %v", err)
	}
	if !resp.StreamFinished {
		t.Fatal("StreamFinished not set")
	}
	if resp.Choices[0].FinishReason == nil || *resp.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("finish_reason = %v, want tool_calls", resp.Choices[0].FinishReason)
	}
}

func TestResponseOutboundStreamIncompleteNotErrorPreservesUsage(t *testing.T) {
	o := &ResponseOutbound{}
	resp, err := o.TransformStream(context.Background(), []byte(`{"type":"response.incomplete","response":{"status":"incomplete","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`))
	if err != nil {
		t.Fatalf("incomplete must not be an error: %v", err)
	}
	if !resp.StreamFinished {
		t.Fatal("StreamFinished not set on incomplete")
	}
	if resp.Choices[0].FinishReason == nil || *resp.Choices[0].FinishReason != "length" {
		t.Fatalf("finish_reason = %v, want length", resp.Choices[0].FinishReason)
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 3 {
		t.Fatalf("usage = %+v, want preserved", resp.Usage)
	}
}

func TestResponseOutboundStreamIncompletePrecedesToolCalls(t *testing.T) {
	o := &ResponseOutbound{}
	_, err := o.TransformStream(context.Background(), []byte(`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","call_id":"call_1","name":"f"}}`))
	if err != nil {
		t.Fatalf("item.added: %v", err)
	}
	resp, err := o.TransformStream(context.Background(), []byte(`{"type":"response.completed","response":{"status":"incomplete"}}`))
	if err != nil {
		t.Fatalf("completed: %v", err)
	}
	if resp.Choices[0].FinishReason == nil || *resp.Choices[0].FinishReason != "length" {
		t.Fatalf("finish_reason = %v, want length (incomplete precedes tool_calls)", resp.Choices[0].FinishReason)
	}
}

func TestResponseOutboundStreamFailedRaisesResponseError(t *testing.T) {
	o := &ResponseOutbound{}
	_, err := o.TransformStream(context.Background(), []byte(`{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","message":"upstream exploded"}}}`))
	if err == nil {
		t.Fatal("response.failed must raise an error")
	}
	respErr, ok := err.(*model.ResponseError)
	if !ok {
		t.Fatalf("error type = %T, want *model.ResponseError", err)
	}
	if respErr.StatusCode != 502 {
		t.Fatalf("StatusCode = %d, want 502", respErr.StatusCode)
	}
	if respErr.Detail.Message != "upstream exploded" {
		t.Fatalf("Message = %q", respErr.Detail.Message)
	}
}

func TestResponseOutboundStreamFailedWithoutPayloadRaisesGenericError(t *testing.T) {
	o := &ResponseOutbound{}
	_, err := o.TransformStream(context.Background(), []byte(`{"type":"response.failed","response":{"status":"failed"}}`))
	if err == nil {
		t.Fatal("response.failed without payload must still raise an error")
	}
	respErr, ok := err.(*model.ResponseError)
	if !ok {
		t.Fatalf("error type = %T, want *model.ResponseError", err)
	}
	if respErr.StatusCode != 502 {
		t.Fatalf("StatusCode = %d, want 502", respErr.StatusCode)
	}
}

func TestResponseOutboundStreamCompletedFailedStatusRaisesError(t *testing.T) {
	o := &ResponseOutbound{}
	_, err := o.TransformStream(context.Background(), []byte(`{"type":"response.completed","response":{"status":"failed","error":{"code":"rate_limit_exceeded","message":"slow down"}}}`))
	if err == nil {
		t.Fatal("response.completed with failed status must raise an error")
	}
	respErr, ok := err.(*model.ResponseError)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if respErr.StatusCode != 429 {
		t.Fatalf("StatusCode = %d, want 429 (rate_limit_exceeded)", respErr.StatusCode)
	}
}

func TestResponseOutboundStreamErrorEventRaises(t *testing.T) {
	o := &ResponseOutbound{}
	_, err := o.TransformStream(context.Background(), []byte(`{"type":"error","code":"rate_limit_error","message":"too many requests"}`))
	if err == nil {
		t.Fatal("error event must raise")
	}
	respErr, ok := err.(*model.ResponseError)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if respErr.StatusCode != 429 {
		t.Fatalf("StatusCode = %d, want 429", respErr.StatusCode)
	}
	if respErr.Detail.Code != "rate_limit_error" {
		t.Fatalf("Code = %q", respErr.Detail.Code)
	}
}

func TestResponseOutboundStreamErrorEventEmptyPayloadRaisesGeneric(t *testing.T) {
	o := &ResponseOutbound{}
	_, err := o.TransformStream(context.Background(), []byte(`{"type":"error"}`))
	if err == nil {
		t.Fatal("empty error event must still raise")
	}
	if _, ok := err.(*model.ResponseError); !ok {
		t.Fatalf("error type = %T", err)
	}
}

func TestResponsesErrorCodeStringAndNumeric(t *testing.T) {
	var stringCode ResponsesError
	if err := transformer.Unmarshal([]byte(`{"code":"server_error","message":"x"}`), &stringCode); err != nil {
		t.Fatalf("string code unmarshal: %v", err)
	}
	if stringCode.String() != "server_error" {
		t.Fatalf("String() = %q", stringCode.String())
	}

	var numericCode ResponsesError
	if err := transformer.Unmarshal([]byte(`{"code":500,"message":"x"}`), &numericCode); err != nil {
		t.Fatalf("numeric code unmarshal: %v", err)
	}
	if numericCode.String() != "500" {
		t.Fatalf("String() = %q, want 500", numericCode.String())
	}
}

func TestResponseOutboundNonStreamFailedStatusRaisesError(t *testing.T) {
	o := &ResponseOutbound{}
	_, err := o.TransformResponse(context.Background(), httpResponse(200, `{"object":"response","id":"r","status":"failed","error":{"code":"server_error","message":"boom"}}`))
	if err == nil {
		t.Fatal("non-stream failed status must raise")
	}
	respErr, ok := err.(*model.ResponseError)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if respErr.StatusCode != 502 || respErr.Detail.Message != "boom" {
		t.Fatalf("ResponseError = %+v", respErr)
	}
}

func TestResponsesOverloadedMapsToBadGateway(t *testing.T) {
	if got := responsesErrorStatusCode("overloaded"); got != http.StatusBadGateway {
		t.Fatalf("overloaded status = %d, want 502", got)
	}
}

func strictBoolPtr(b bool) *bool { return &b }

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
