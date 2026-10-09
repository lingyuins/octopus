package openai

import (
	"context"
	"github.com/lingyuins/octopus/internal/transformer"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

type ChatInbound struct {
	// streamChunks stores stream chunks for aggregation
	streamChunks []*model.InternalLLMResponse
	// storedResponse stores the non-stream response
	storedResponse *model.InternalLLMResponse
}

func (i *ChatInbound) TransformRequest(ctx context.Context, body []byte) (*model.InternalLLMRequest, error) {
	var request model.InternalLLMRequest
	if err := transformer.Unmarshal(body, &request); err != nil {
		return nil, err
	}
	request.RawAPIFormat = model.APIFormatOpenAIChatCompletion
	return &request, nil
}

func (i *ChatInbound) TransformResponse(ctx context.Context, response *model.InternalLLMResponse) ([]byte, error) {
	// Store the response for later retrieval
	i.storedResponse = response

	body, err := transformer.Marshal(response)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (i *ChatInbound) TransformStream(ctx context.Context, stream *model.InternalLLMResponse) ([]byte, error) {
	if stream.Object == "[DONE]" {
		return []byte("data: [DONE]\n\n"), nil
	}

	// Store the chunk for aggregation
	i.streamChunks = append(i.streamChunks, stream)

	var body []byte
	var err error

	// OpenAI Chat Completion stream chunks must always carry a "choices" array,
	// and every choice must carry a "delta" field — even an empty object on the
	// final finish_reason chunk. Strict clients (e.g. RikkaHub) treat a missing
	// or null choices[0].delta as a protocol violation and throw. Since
	// model.Choice.Delta is omitempty, stream chunks are serialized through a
	// dedicated type that always emits both fields per the OpenAI SSE spec.
	if stream.Object == "chat.completion.chunk" {
		body, err = marshalChatChunk(stream)
	} else {
		body, err = transformer.Marshal(stream)
	}

	if err != nil {
		return nil, err
	}
	return []byte("data: " + string(body) + "\n\n"), nil
}

// streamChoice renders a chat.completion.chunk choice. The Delta field shadows
// model.Choice.Delta (which is omitempty) so that "delta" is always emitted,
// matching the OpenAI streaming spec — including an empty {} on the terminal
// finish_reason chunk.
type streamChoice struct {
	model.Choice
	Delta *model.Message `json:"delta"`
}

// marshalChatChunk serializes an OpenAI chat completion stream chunk, ensuring
// "choices" is always an array (possibly empty) and each choice always carries
// a non-null "delta" field. It does not mutate the input chunk, so the stored
// internal representation used for aggregation stays untouched.
func marshalChatChunk(stream *model.InternalLLMResponse) ([]byte, error) {
	choices := make([]streamChoice, 0, len(stream.Choices))
	for _, choice := range stream.Choices {
		delta := choice.Delta
		if delta == nil {
			delta = &model.Message{}
		}
		choices = append(choices, streamChoice{Choice: choice, Delta: delta})
	}

	type alias model.InternalLLMResponse
	aux := &struct {
		*alias
		Choices []streamChoice `json:"choices"`
	}{
		alias:   (*alias)(stream),
		Choices: choices,
	}
	return transformer.Marshal(aux)
}

// GetInternalResponse returns the complete internal response for logging, statistics, etc.
// For streaming: aggregates all stored stream chunks into a complete response
// For non-streaming: returns the stored response
func (i *ChatInbound) GetInternalResponse(ctx context.Context) (*model.InternalLLMResponse, error) {
	// Return stored response for non-stream scenario
	if i.storedResponse != nil {
		return i.storedResponse, nil
	}

	// Aggregate stream chunks for stream scenario
	if len(i.streamChunks) == 0 {
		return nil, nil
	}

	// Use the first chunk as the base
	firstChunk := i.streamChunks[0]
	result := &model.InternalLLMResponse{
		ID:                firstChunk.ID,
		Object:            "chat.completion",
		Created:           firstChunk.Created,
		Model:             firstChunk.Model,
		SystemFingerprint: firstChunk.SystemFingerprint,
		ServiceTier:       firstChunk.ServiceTier,
	}

	// Aggregate choices by index
	choicesMap := make(map[int]*model.Choice)

	for _, chunk := range i.streamChunks {
		// Update ID and Model if they appear in later chunks (some providers send these later)
		if chunk.ID != "" {
			result.ID = chunk.ID
		}
		if chunk.Model != "" {
			result.Model = chunk.Model
		}

		// Capture usage from the last chunk that has it
		if chunk.Usage != nil {
			result.Usage = chunk.Usage
		}

		for _, choice := range chunk.Choices {
			existingChoice, exists := choicesMap[choice.Index]
			if !exists {
				existingChoice = &model.Choice{
					Index:   choice.Index,
					Message: &model.Message{},
				}
				choicesMap[choice.Index] = existingChoice
			}

			// Aggregate delta content into message
			if choice.Delta != nil {
				delta := choice.Delta

				// Set role if present
				if delta.Role != "" {
					existingChoice.Message.Role = delta.Role
				}

				// Append content (handle both string content and multipart content)
				if delta.Content.Content != nil {
					if existingChoice.Message.Content.Content == nil {
						existingChoice.Message.Content.Content = new(string)
					}
					*existingChoice.Message.Content.Content += *delta.Content.Content
				}

				// Append multipart content (for images, audio, etc.)
				if len(delta.Content.MultipleContent) > 0 {
					existingChoice.Message.Content.MultipleContent = append(
						existingChoice.Message.Content.MultipleContent,
						delta.Content.MultipleContent...,
					)
				}

				// Append images (used by Gemini via OpenAI compat endpoint for image generation)
				if len(delta.Images) > 0 {
					existingChoice.Message.Content.MultipleContent = append(
						existingChoice.Message.Content.MultipleContent,
						delta.Images...,
					)
				}

				// Append reasoning content (supports both reasoning_content and reasoning fields)
				if delta.GetReasoningContent() != "" {
					if existingChoice.Message.ReasoningContent == nil {
						existingChoice.Message.ReasoningContent = new(string)
					}
					*existingChoice.Message.ReasoningContent += delta.GetReasoningContent()
				}

				if delta.ReasoningSignature != nil {
					signature := *delta.ReasoningSignature
					if delta.ReasoningSignatureFormat != model.APIFormatOpenAIResponse && existingChoice.Message.ReasoningSignature != nil {
						signature = *existingChoice.Message.ReasoningSignature + signature
					}
					existingChoice.Message.ReasoningSignature = &signature
					existingChoice.Message.ReasoningSignatureFormat = delta.ReasoningSignatureFormat
				}

				// Aggregate tool calls
				for _, toolCall := range delta.ToolCalls {
					existingChoice.Message.ToolCalls = mergeToolCall(existingChoice.Message.ToolCalls, toolCall)
				}

				// Append refusal if present
				if delta.Refusal != "" {
					existingChoice.Message.Refusal += delta.Refusal
				}
			}

			// Capture finish reason
			if choice.FinishReason != nil {
				existingChoice.FinishReason = choice.FinishReason
			}

			// Capture logprobs
			if choice.Logprobs != nil {
				if existingChoice.Logprobs == nil {
					existingChoice.Logprobs = &model.LogprobsContent{}
				}
				existingChoice.Logprobs.Content = append(existingChoice.Logprobs.Content, choice.Logprobs.Content...)
			}
		}
	}

	result.Choices = model.SortedChoicesByIndex(choicesMap)

	// Clear stored chunks after aggregation
	i.streamChunks = nil

	return result, nil
}

// mergeToolCall merges a tool call delta into the existing tool calls slice
func mergeToolCall(toolCalls []model.ToolCall, delta model.ToolCall) []model.ToolCall {
	// Find existing tool call by index
	for i, tc := range toolCalls {
		if tc.Index == delta.Index {
			// Merge the delta into existing tool call
			if delta.ID != "" {
				toolCalls[i].ID = delta.ID
			}
			if delta.Type != "" {
				toolCalls[i].Type = delta.Type
			}
			if delta.Function.Name != "" {
				toolCalls[i].Function.Name += delta.Function.Name
			}
			if delta.Function.Arguments != "" {
				toolCalls[i].Function.Arguments += delta.Function.Arguments
			}
			return toolCalls
		}
	}

	// New tool call, add it
	return append(toolCalls, delta)
}
