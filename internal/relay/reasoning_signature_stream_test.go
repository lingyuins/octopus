package relay

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/transformer/inbound"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

func TestRelayResponsesStreamEncryptedReasoning(t *testing.T) {
	for _, mode := range []string{"with_summary", "empty_summary", "terminal_only", "incomplete"} {
		for _, client := range []inbound.InboundType{inbound.InboundTypeOpenAIResponse, inbound.InboundTypeAnthropic} {
			name := mode + "/responses"
			if client == inbound.InboundTypeAnthropic {
				name = mode + "/anthropic"
			}
			t.Run(name, func(t *testing.T) {
				ra, recorder := newStreamTestAttempt(t, context.Background())
				ra.group = &dbmodel.Group{ReasoningBufferStrategy: "buffer"}
				ra.outAdapter = outbound.Get(outbound.OutboundTypeOpenAIResponse)
				ra.inAdapter = inbound.Get(client)
				var body strings.Builder
				feed := func(event string) { body.WriteString("data: " + event + "\n\n") }
				if mode == "with_summary" {
					feed(`{"type":"response.reasoning_summary_text.delta","output_index":0,"delta":"thinking"}`)
				}
				if mode != "terminal_only" && mode != "incomplete" {
					feed(`{"type":"response.output_item.done","output_index":0,"item":{"type":"reasoning","summary":[],"encrypted_content":"opaque"}}`)
				}
				feed(`{"type":"response.output_text.delta","output_index":1,"delta":"answer"}`)
				terminal := "response.completed"
				status := "completed"
				if mode == "incomplete" {
					terminal = "response.incomplete"
					status = "incomplete"
				}
				feed(`{"type":"` + terminal + `","response":{"id":"r1","model":"test","status":"` + status + `","output":[{"type":"reasoning","summary":[],"encrypted_content":"opaque"}]}}`)
				err := ra.handleStreamResponse(context.Background(), &http.Response{StatusCode: http.StatusOK,
					Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body.String()))})
				if err != nil {
					t.Fatal(err)
				}
				ra.collectResponse()
				if ra.metrics.InternalResponse == nil || ra.metrics.InternalResponse.Choices[0].Message.ReasoningSignature == nil ||
					*ra.metrics.InternalResponse.Choices[0].Message.ReasoningSignature != "opaque" {
					t.Fatalf("ciphertext lost or duplicated in aggregation: %+v", ra.metrics.InternalResponse)
				}
				if client == inbound.InboundTypeAnthropic {
					if strings.Contains(recorder.Body.String(), "opaque") || strings.Contains(recorder.Body.String(), "signature_delta") {
						t.Fatalf("ciphertext leaked as Anthropic signature: %s", recorder.Body.String())
					}
					return
				}
				var completedOutput []reasoningWireItem
				var itemDone []reasoningWireItem
				for _, line := range strings.Split(recorder.Body.String(), "\n") {
					if !strings.HasPrefix(line, "data: {") {
						continue
					}
					var event struct {
						Type     string            `json:"type"`
						Item     reasoningWireItem `json:"item"`
						Response struct {
							Output []reasoningWireItem `json:"output"`
						} `json:"response"`
					}
					if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
						t.Fatal(err)
					}
					if event.Type == "response.completed" || event.Type == "response.incomplete" {
						completedOutput = event.Response.Output
					}
					if event.Type == "response.output_item.done" {
						itemDone = append(itemDone, event.Item)
					}
				}
				assertCiphertext := func(items []reasoningWireItem) {
					t.Helper()
					found := 0
					for _, item := range items {
						if item.EncryptedContent == "opaque" {
							found++
							if item.Type != "reasoning" || item.Summary == nil {
								t.Fatalf("invalid reasoning item: %+v", item)
							}
							if mode == "with_summary" && (len(item.Summary) != 1 || item.Summary[0].Text != "thinking") {
								t.Fatalf("summary separated from ciphertext: %+v", item)
							}
						}
					}
					if found != 1 {
						t.Fatalf("ciphertext missing or duplicated: %s", recorder.Body.String())
					}
				}
				assertCiphertext(itemDone)
				assertCiphertext(completedOutput)
			})
		}
	}
}
