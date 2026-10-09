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
	tmodel "github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

type reasoningWireItem struct {
	Type             string              `json:"type"`
	Text             string              `json:"text"`
	Refusal          string              `json:"refusal"`
	EncryptedContent string              `json:"encrypted_content"`
	Summary          []reasoningWireItem `json:"summary"`
	Content          []reasoningWireItem `json:"content"`
}

func TestRelayResponsesNonStreamRefusalAndEncryptedReasoning(t *testing.T) {
	for _, summary := range []string{`[]`, `[{"type":"summary_text","text":"thinking"}]`} {
		t.Run(summary, func(t *testing.T) {
			ra, recorder := newStreamTestAttempt(t, context.Background())
			ra.outAdapter = outbound.Get(outbound.OutboundTypeOpenAIResponse)
			ra.inAdapter = inbound.Get(inbound.InboundTypeOpenAIResponse)
			ra.internalRequest.Stream = nil
			body := `{"id":"resp1","model":"test","status":"completed","output":[{"type":"reasoning","summary":` + summary + `,"encrypted_content":"opaque"},{"type":"message","role":"assistant","content":[{"type":"refusal","refusal":"cannot help"}]}]}`
			err := ra.handleResponse(context.Background(), &http.Response{
				StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)),
			})
			if err != nil {
				t.Fatalf("valid refusal retried or conversion failed: %v", err)
			}
			var response struct {
				Output []reasoningWireItem `json:"output"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != http.StatusOK || len(response.Output) != 2 {
				t.Fatalf("wrong client response: %d %s", recorder.Code, recorder.Body.String())
			}
			reasoning := response.Output[0]
			if reasoning.Type != "reasoning" || reasoning.EncryptedContent != "opaque" || reasoning.Summary == nil {
				t.Fatalf("encrypted reasoning or summary array lost: %s", recorder.Body.String())
			}
			if summary == `[]` && len(reasoning.Summary) != 0 {
				t.Fatalf("fabricated summary: %+v", reasoning.Summary)
			}
			if summary != `[]` && (len(reasoning.Summary) != 1 || reasoning.Summary[0].Text != "thinking") {
				t.Fatalf("summary lost: %+v", reasoning.Summary)
			}
			message := response.Output[1]
			if message.Type != "message" || len(message.Content) != 1 || message.Content[0].Type != "refusal" || message.Content[0].Refusal != "cannot help" {
				t.Fatalf("refusal became empty output_text: %s", recorder.Body.String())
			}
		})
	}
}

func TestRelayResponsesPureRefusalIsVisible(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, client := range []inbound.InboundType{inbound.InboundTypeOpenAIResponse, inbound.InboundTypeOpenAIChat} {
			for _, session := range []bool{false, true} {
				if !stream && session {
					continue
				}
				t.Run(testRefusalName(stream, client, session), func(t *testing.T) {
					ra, recorder := newStreamTestAttempt(t, context.Background())
					ra.group = &dbmodel.Group{ReasoningBufferStrategy: "buffer"}
					ra.outAdapter = outbound.Get(outbound.OutboundTypeOpenAIResponse)
					ra.inAdapter = inbound.Get(client)
					if session {
						attachStreamSession(t, ra)
					}
					body := `{"id":"refusal","model":"test","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"refusal","refusal":"cannot help"}]}]}`
					response := &http.Response{StatusCode: http.StatusOK}
					var err error
					if stream {
						body = "data: {\"type\":\"response.refusal.delta\",\"delta\":\"cannot\"}\n\n" +
							"data: {\"type\":\"response.refusal.delta\",\"delta\":\" help\"}\n\n" +
							"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"refusal\",\"status\":\"completed\",\"output\":[]}}\n\n"
						response.Header = http.Header{"Content-Type": []string{"text/event-stream"}}
						response.Body = io.NopCloser(strings.NewReader(body))
						err = ra.handleStreamResponse(context.Background(), response)
					} else {
						ra.internalRequest.Stream = nil
						response.Body = io.NopCloser(strings.NewReader(body))
						err = ra.handleResponse(context.Background(), response)
					}
					if err != nil || !strings.Contains(recorder.Body.String(), `"refusal"`) || !strings.Contains(recorder.Body.String(), "cannot") {
						t.Fatalf("refusal not delivered: err=%v body=%s", err, recorder.Body.String())
					}
					if stream && strings.Count(recorder.Body.String(), "data: [DONE]") != 1 {
						t.Fatalf("missing client termination: %s", recorder.Body.String())
					}
					if stream {
						ra.collectResponse()
					}
					if stream && (ra.metrics.InternalResponse == nil || ra.metrics.InternalResponse.Choices[0].Message.Refusal != "cannot help") {
						t.Fatalf("relay aggregation lost refusal: %+v", ra.metrics.InternalResponse)
					}
				})
			}
		}
	}
}

func testRefusalName(stream bool, client inbound.InboundType, session bool) string {
	name := "nonstream"
	if stream {
		name = "buffered_stream"
	}
	if client == inbound.InboundTypeOpenAIChat {
		name += "/chat"
	} else {
		name += "/responses"
	}
	if session {
		name += "/session"
	} else {
		name += "/direct"
	}
	return name
}

func TestResponsesEncryptedReasoningRequestRoundTrip(t *testing.T) {
	ctx := context.Background()
	for _, summary := range []string{`[]`, `[{"type":"summary_text","text":"thinking"}]`} {
		t.Run(summary, func(t *testing.T) {
			adapter := inbound.Get(inbound.InboundTypeOpenAIResponse)
			request, err := adapter.TransformRequest(ctx, []byte(`{"model":"test","include":["reasoning.encrypted_content"],"input":[{"type":"reasoning","summary":`+summary+`,"encrypted_content":"opaque"}]}`))
			if err != nil {
				t.Fatal(err)
			}
			upstream, err := outbound.Get(outbound.OutboundTypeOpenAIResponse).TransformRequest(ctx, request, "https://example.com", "")
			if err != nil {
				t.Fatal(err)
			}
			defer upstream.Body.Close()
			var result struct {
				Input   []reasoningWireItem `json:"input"`
				Include []string            `json:"include"`
			}
			if err := json.NewDecoder(upstream.Body).Decode(&result); err != nil {
				t.Fatal(err)
			}
			if len(result.Input) != 1 || result.Input[0].Type != "reasoning" || result.Input[0].EncryptedContent != "opaque" || result.Input[0].Summary == nil ||
				len(result.Include) != 1 || result.Include[0] != "reasoning.encrypted_content" {
				t.Fatalf("reasoning history lost on wire: %+v", result)
			}
		})
	}
}

func TestReasoningSignaturesStayWithinTheirProtocol(t *testing.T) {
	ctx := context.Background()
	for _, source := range []outbound.OutboundType{outbound.OutboundTypeOpenAIResponse, outbound.OutboundTypeAnthropic} {
		for _, client := range []inbound.InboundType{inbound.InboundTypeOpenAIResponse, inbound.InboundTypeAnthropic} {
			t.Run(testSignatureName(source, client), func(t *testing.T) {
				body := `{"id":"resp1","model":"test","status":"completed","output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"thinking"}],"encrypted_content":"openai-opaque"},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"answer"}]}]}`
				if source == outbound.OutboundTypeAnthropic {
					body = `{"id":"msg1","model":"test","role":"assistant","stop_reason":"end_turn","content":[{"type":"thinking","thinking":"thinking","signature":"anthropic-signature"},{"type":"text","text":"answer"}]}`
				}
				internal, err := outbound.Get(source).TransformResponse(ctx, &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))})
				if err != nil {
					t.Fatal(err)
				}
				internal = cloneInternalResponse(internal)
				if internal == nil {
					t.Fatal("internal response clone failed")
				}
				data, err := inbound.Get(client).TransformResponse(ctx, internal)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(data), "thinking") || !strings.Contains(string(data), "answer") {
					t.Fatalf("visible reasoning or answer lost: %s", data)
				}
				if source == outbound.OutboundTypeOpenAIResponse && client == inbound.InboundTypeAnthropic && strings.Contains(string(data), "openai-opaque") {
					t.Fatalf("OpenAI ciphertext leaked as Anthropic signature: %s", data)
				}
				if source == outbound.OutboundTypeAnthropic && client == inbound.InboundTypeOpenAIResponse && strings.Contains(string(data), "anthropic-signature") {
					t.Fatalf("Anthropic signature leaked as OpenAI ciphertext: %s", data)
				}
				if source == outbound.OutboundTypeOpenAIResponse && client == inbound.InboundTypeOpenAIResponse && !strings.Contains(string(data), `"encrypted_content":"openai-opaque"`) {
					t.Fatalf("native ciphertext lost: %s", data)
				}
				if source == outbound.OutboundTypeAnthropic && client == inbound.InboundTypeAnthropic && !strings.Contains(string(data), `"signature":"anthropic-signature"`) {
					t.Fatalf("native signature lost: %s", data)
				}

				for _, withTool := range []bool{false, true} {
					message := *internal.Choices[0].Message
					if withTool {
						message.ToolCalls = []tmodel.ToolCall{{ID: "call", Type: "function", Function: tmodel.FunctionCall{Name: "f", Arguments: "{}"}}}
					}
					request := &tmodel.InternalLLMRequest{Model: "test", RawAPIFormat: tmodel.APIFormatOpenAIResponse, Messages: []tmodel.Message{message}}
					upstream, err := outbound.Get(outbound.OutboundTypeAnthropic).TransformRequest(ctx, request, "https://example.com", "")
					if err != nil {
						t.Fatal(err)
					}
					requestBody, err := io.ReadAll(upstream.Body)
					upstream.Body.Close()
					if err != nil {
						t.Fatal(err)
					}
					if source == outbound.OutboundTypeOpenAIResponse && strings.Contains(string(requestBody), "openai-opaque") {
						t.Fatalf("OpenAI ciphertext leaked into Anthropic history: %s", requestBody)
					}
					if source == outbound.OutboundTypeAnthropic && !strings.Contains(string(requestBody), `"signature":"anthropic-signature"`) {
						t.Fatalf("native signature lost in Anthropic history: %s", requestBody)
					}
				}
			})
		}
	}
}

func testSignatureName(source outbound.OutboundType, client inbound.InboundType) string {
	name := "responses"
	if source == outbound.OutboundTypeAnthropic {
		name = "anthropic"
	}
	if client == inbound.InboundTypeAnthropic {
		return name + "/anthropic"
	}
	return name + "/responses"
}
