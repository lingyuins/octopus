package relay

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/inbound"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

func TestPrematureStreamEOFIsFailure(t *testing.T) {
	cases := []struct {
		name    string
		adapter outbound.OutboundType
		body    string
	}{
		{"responses", outbound.OutboundTypeOpenAIResponse, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"r\",\"model\":\"test\",\"status\":\"in_progress\"}}\n\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"unfinished\"}\n\n"},
		{"chat", outbound.OutboundTypeOpenAIChat, "data: {\"id\":\"r\",\"model\":\"test\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"unfinished\"}}]}\n\n"},
		{"messages", outbound.OutboundTypeAnthropic, "data: {\"type\":\"message_start\",\"message\":{\"id\":\"r\",\"model\":\"test\"}}\n\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"unfinished\"}}\n\n"},
	}
	for _, tc := range cases {
		for _, client := range []inbound.InboundType{inbound.InboundTypeOpenAIChat, inbound.InboundTypeOpenAIResponse, inbound.InboundTypeAnthropic} {
			t.Run(tc.name+"_"+map[inbound.InboundType]string{inbound.InboundTypeOpenAIChat: "chat", inbound.InboundTypeOpenAIResponse: "responses", inbound.InboundTypeAnthropic: "messages"}[client], func(t *testing.T) {
				ra, recorder := newStreamTestAttempt(t, context.Background())
				ra.outAdapter = outbound.Get(tc.adapter)
				ra.inAdapter = inbound.Get(client)
				session := attachStreamSession(t, ra)
				err := ra.handleStreamResponse(context.Background(), &http.Response{
					StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
					Body: io.NopCloser(strings.NewReader(tc.body)),
				})
				if !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatalf("premature EOF not reported: %v", err)
				}
				body := recorder.Body.String()
				if strings.Contains(body, `"type":"response.completed"`) || strings.Contains(body, "data: [DONE]") || strings.Contains(body, "event:message_stop") {
					t.Fatalf("premature EOF forged a success terminal: %s", body)
				}
				_, done, sessionErr := session.Snapshot(0)
				if !done || !errors.Is(sessionErr, io.ErrUnexpectedEOF) {
					t.Fatalf("session lost truncation error: done=%v err=%v", done, sessionErr)
				}
			})
		}
	}
}

func TestStreamFinalUsageAfterFinish(t *testing.T) {
	for _, client := range []inbound.InboundType{inbound.InboundTypeOpenAIResponse, inbound.InboundTypeAnthropic} {
		t.Run(map[inbound.InboundType]string{inbound.InboundTypeOpenAIResponse: "responses", inbound.InboundTypeAnthropic: "messages"}[client], func(t *testing.T) {
			ra, recorder := newStreamTestAttempt(t, context.Background())
			ra.outAdapter = outbound.Get(outbound.OutboundTypeOpenAIChat)
			ra.inAdapter = inbound.Get(client)
			body := "data: {\"id\":\"r\",\"model\":\"test\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":1,\"total_tokens\":11}}\n\n" +
				"data: {\"id\":\"r\",\"model\":\"test\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
				"data: {\"id\":\"r\",\"model\":\"test\",\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15}}\n\n" +
				"data: [DONE]\n\n"
			err := ra.handleStreamResponse(context.Background(), &http.Response{StatusCode: http.StatusOK,
				Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(recorder.Body.String(), `"output_tokens":5`) {
				t.Fatalf("final output tokens lost: %s", recorder.Body.String())
			}
		})
	}
}

func TestEmptyEOFDoesNotCommitResponseOrSession(t *testing.T) {
	ra, _ := newStreamTestAttempt(t, context.Background())
	session := attachStreamSession(t, ra)
	err := ra.handleStreamResponse(context.Background(), &http.Response{StatusCode: http.StatusOK,
		Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(""))})
	if !errors.Is(err, io.ErrUnexpectedEOF) || ra.c.Writer.Written() || session.IsDone() {
		t.Fatalf("empty EOF is not safely retryable: err=%v written=%v done=%v", err, ra.c.Writer.Written(), session.IsDone())
	}
}

func TestPartialChoiceCompletionDoesNotHideEOF(t *testing.T) {
	ra, _ := newStreamTestAttempt(t, context.Background())
	ra.outAdapter = outbound.Get(outbound.OutboundTypeOpenAIChat)
	body := "data: {\"id\":\"r\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"one\"}},{\"index\":1,\"delta\":{\"content\":\"two\"}}]}\n\n" +
		"data: {\"id\":\"r\",\"choices\":[{\"index\":0,\"finish_reason\":\"stop\"}]}\n\n"
	err := ra.handleStreamResponse(context.Background(), &http.Response{StatusCode: http.StatusOK,
		Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))})
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("unfinished second choice treated as complete: %v", err)
	}
}
