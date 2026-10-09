package relay

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/transformer/inbound"
	tmodel "github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

func TestProtocolCompletionWithoutEOF(t *testing.T) {
	tests := []struct {
		name    string
		adapter outbound.OutboundType
		events  []string
	}{
		{
			name: "responses", adapter: outbound.OutboundTypeOpenAIResponse,
			events: []string{
				`{"type":"response.created","response":{"id":"resp_ok","model":"test-model","status":"in_progress"}}`,
				`{"type":"response.output_text.delta","delta":"Hello","output_index":0,"content_index":0}`,
				`{"type":"response.completed","response":{"id":"resp_ok","model":"test-model","status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			},
		},
		{
			name: "responses_incomplete", adapter: outbound.OutboundTypeOpenAIResponse,
			events: []string{
				`{"type":"response.output_text.delta","delta":"Hello","output_index":0,"content_index":0}`,
				`{"type":"response.incomplete","response":{"id":"resp_short","model":"test-model","status":"incomplete","output":[],"incomplete_details":{"reason":"max_output_tokens"},"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			},
		},
		{
			name: "messages", adapter: outbound.OutboundTypeAnthropic,
			events: []string{
				`{"type":"message_start","message":{"id":"msg_ok","model":"test-model","role":"assistant","usage":{"input_tokens":1,"output_tokens":0}}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
				`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
				`{"type":"message_stop"}`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ra, recorder := newStreamTestAttempt(t, context.Background())
			ra.outAdapter = outbound.Get(tt.adapter)
			ra.inAdapter = inbound.Get(inbound.InboundTypeOpenAIChat)
			reader, writer := io.Pipe()
			t.Cleanup(func() { _ = writer.Close() })
			t.Cleanup(func() { _ = reader.Close() })
			done := make(chan error, 1)
			go func() {
				done <- ra.handleStreamResponse(context.Background(), &http.Response{
					StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: reader,
				})
			}()
			go func() {
				for _, event := range tt.events {
					if _, err := io.WriteString(writer, "data: "+event+"\n\n"); err != nil {
						return
					}
				}
			}()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("stream returned %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("protocol completion waited for upstream EOF")
			}
			if !strings.Contains(recorder.Body.String(), "Hello") || strings.Count(recorder.Body.String(), "data: [DONE]") != 1 {
				t.Fatalf("missing text or duplicate terminator: %s", recorder.Body.String())
			}
		})
	}
}

func TestStreamFailedResponsePropagatesError(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(map[bool]string{false: "before_output", true: "after_output"}[partial], func(t *testing.T) {
			ra, recorder := newStreamTestAttempt(t, context.Background())
			ra.outAdapter = outbound.Get(outbound.OutboundTypeOpenAIResponse)
			ra.inAdapter = inbound.Get(inbound.InboundTypeOpenAIChat)
			body := ""
			if partial {
				body = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello\"}\n\n"
			}
			body += "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_failure\",\"status\":\"failed\",\"error\":{\"code\":\"server_error\",\"message\":\"generation failed\"}}}\n\n"
			err := ra.handleStreamResponse(context.Background(), &http.Response{
				StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
				Body: io.NopCloser(strings.NewReader(body)),
			})
			if err == nil || !strings.Contains(err.Error(), "generation failed") {
				t.Fatalf("failure was lost: err=%v body=%s", err, recorder.Body.String())
			}
			if !partial && ra.c.Writer.Written() {
				t.Fatal("failure before output must remain eligible for retry")
			}
			if partial && !strings.Contains(recorder.Body.String(), "event: error") {
				t.Fatalf("partial stream missing error event: %s", recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "data: [DONE]") {
				t.Fatal("failed stream was finalized as success")
			}
		})
	}
}

type brokenStreamOutbound struct{ stubOutboundAdapter }

func (brokenStreamOutbound) TransformStream(context.Context, []byte) (*tmodel.InternalLLMResponse, error) {
	return nil, errors.New("malformed stream event")
}

func TestBufferedStreamSessionCanRetryBeforeOutput(t *testing.T) {
	ra, _ := newStreamTestAttempt(t, context.Background())
	ra.group = &dbmodel.Group{ReasoningBufferStrategy: "buffer"}
	ra.outAdapter = reasoningOnlyOutbound{}
	session := attachStreamSession(t, ra)
	err := ra.handleStreamResponse(context.Background(), &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader("data: thinking\n\ndata: [DONE]\n\n")),
	})
	if !errors.Is(err, errEmptyOutput) || session.IsDone() {
		t.Fatalf("retryable attempt prematurely finished session: %v", err)
	}
	ra.outAdapter = stubOutboundAdapter{}
	err = ra.handleStreamResponse(context.Background(), &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader("data: hello\n\ndata: [DONE]\n\n")),
	})
	if err != nil || !session.IsDone() {
		t.Fatalf("successful retry did not finalize session: %v", err)
	}
}

func TestStreamConversionErrorIsNotSkipped(t *testing.T) {
	ra, _ := newStreamTestAttempt(t, context.Background())
	ra.outAdapter = brokenStreamOutbound{}
	err := ra.handleStreamResponse(context.Background(), &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader("data: invalid\n\n")),
	})
	if err == nil || !strings.Contains(err.Error(), "malformed stream event") {
		t.Fatalf("conversion failure swallowed: %v", err)
	}
}

func TestStreamEOFCompletesClientProtocol(t *testing.T) {
	for _, client := range []inbound.InboundType{inbound.InboundTypeOpenAIChat, inbound.InboundTypeOpenAIResponse, inbound.InboundTypeAnthropic} {
		t.Run(map[inbound.InboundType]string{inbound.InboundTypeOpenAIChat: "chat", inbound.InboundTypeOpenAIResponse: "responses", inbound.InboundTypeAnthropic: "messages"}[client], func(t *testing.T) {
			ra, recorder := newStreamTestAttempt(t, context.Background())
			ra.outAdapter = outbound.Get(outbound.OutboundTypeOpenAIChat)
			ra.inAdapter = inbound.Get(client)
			body := "data: {\"id\":\"chat1\",\"object\":\"chat.completion.chunk\",\"model\":\"test\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\n" +
				"data: {\"id\":\"chat1\",\"object\":\"chat.completion.chunk\",\"model\":\"test\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"
			err := ra.handleStreamResponse(context.Background(), &http.Response{StatusCode: http.StatusOK,
				Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))})
			if err != nil {
				t.Fatal(err)
			}
			terminal := "data: [DONE]"
			if client == inbound.InboundTypeAnthropic {
				terminal = "event:message_stop"
			}
			if strings.Count(recorder.Body.String(), terminal) != 1 {
				t.Fatalf("missing or duplicate terminal after EOF: %s", recorder.Body.String())
			}
		})
	}
}

func TestBufferedEmptyStreamRemainsRetryableWithoutSession(t *testing.T) {
	ra, recorder := newStreamTestAttempt(t, context.Background())
	ra.group = &dbmodel.Group{ReasoningBufferStrategy: "buffer"}
	ra.outAdapter = reasoningOnlyOutbound{}
	err := ra.handleStreamResponse(context.Background(), &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader("data: thinking\n\ndata: [DONE]\n\n")),
	})
	if !errors.Is(err, errEmptyOutput) || recorder.Body.Len() != 0 || ra.c.Writer.Written() {
		t.Fatalf("empty attempt committed response: err=%v body=%q", err, recorder.Body.String())
	}
}

func TestRelayProtocolErrorKeepsStatusAndDetail(t *testing.T) {
	err := &tmodel.ResponseError{StatusCode: http.StatusBadRequest,
		Detail: tmodel.ErrorDetail{Code: "invalid_prompt", Message: "prompt rejected", Type: "invalid_request_error"}}
	wrapped := errors.Join(errors.New("failed to transform stream event"), err)
	if got := relayProtocolErrorStatus(http.StatusOK, wrapped); got != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", got)
	}
	ra, recorder := newStreamTestAttempt(t, context.Background())
	writeClientTerminalError(ra.c, http.StatusBadRequest, wrapped)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"invalid_prompt"`) {
		t.Fatalf("protocol detail lost: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestInboundAdapterStateResetsBetweenAttempts(t *testing.T) {
	ra, _ := newStreamTestAttempt(t, context.Background())
	ra.newInAdapter = func() tmodel.Inbound { return inbound.Get(inbound.InboundTypeOpenAIResponse) }
	ra.inAdapter = ra.newInAdapter()
	ra.internalRequest.RawRequest = []byte(`{"model":"stub-model","input":"test","stream":true}`)
	if err := ra.resetInboundAdapter(); err != nil {
		t.Fatal(err)
	}
	text := "first attempt"
	if _, err := ra.inAdapter.TransformStream(context.Background(), &tmodel.InternalLLMResponse{
		ID: "first", Choices: []tmodel.Choice{{Delta: &tmodel.Message{Content: tmodel.MessageContent{Content: &text}}}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := ra.resetInboundAdapter(); err != nil {
		t.Fatal(err)
	}
	text = "second attempt"
	data, err := ra.inAdapter.TransformStream(context.Background(), &tmodel.InternalLLMResponse{
		ID: "second", Choices: []tmodel.Choice{{Delta: &tmodel.Message{Content: tmodel.MessageContent{Content: &text}}}},
	})
	if err != nil || !strings.Contains(string(data), "response.created") || strings.Contains(string(data), "first attempt") || strings.Contains(string(data), `"id":"first"`) {
		t.Fatalf("attempt state leaked: err=%v data=%s", err, data)
	}
}
