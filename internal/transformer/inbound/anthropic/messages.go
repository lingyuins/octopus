package anthropic

import (
	"context"
	"fmt"
	"sort"

	"github.com/lingyuins/octopus/internal/transformer"
	"github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/lingyuins/octopus/internal/utils/tokenizer"
	"github.com/lingyuins/octopus/internal/utils/xurl"
	"github.com/samber/lo"
)

type MessagesInbound struct {
	// Stream state tracking
	hasStarted  bool
	hasFinished bool
	messageID   string
	modelName   string
	stopReason  *string

	// Block tracking: every block owns a stable Anthropic content index for
	// its whole lifecycle; deltas and stops must target that index.
	thinkingBlockIndex *int64
	textBlockIndex     *int64
	// toolBlockIndex maps each OpenAI tool call index to its block's content
	// index. Tool blocks stay open concurrently; fragments are routed by the
	// owning tool's index.
	toolBlockIndex map[int]int64
	// toolBlockStopped marks tool blocks already closed; no delta after stop.
	toolBlockStopped map[int]bool

	// nextIndex allocates new content block indexes.
	nextIndex int64

	// messageStopped guards the terminal pair is emitted at most once.
	messageStopped bool
	// terminalUsage caches the last usage seen, before or after finish.
	terminalUsage *model.Usage

	inputToken int64

	// Stream chunks storage for aggregation
	streamChunks []*model.InternalLLMResponse
	// storedResponse stores the non-stream response
	storedResponse *model.InternalLLMResponse
}

func (i *MessagesInbound) TransformRequest(ctx context.Context, body []byte) (*model.InternalLLMRequest, error) {
	var anthropicReq MessageRequest
	if err := transformer.Unmarshal(body, &anthropicReq); err != nil {
		return nil, err
	}
	if anthropicReq.MaxTokens < 1 {
		anthropicReq.MaxTokens = 1
	}
	chatReq := &model.InternalLLMRequest{
		Model:               anthropicReq.Model,
		MaxTokens:           &anthropicReq.MaxTokens,
		Temperature:         anthropicReq.Temperature,
		TopP:                anthropicReq.TopP,
		Stream:              anthropicReq.Stream,
		Metadata:            map[string]string{},
		RawAPIFormat:        model.APIFormatAnthropicMessage,
		TransformerMetadata: map[string]string{},
	}
	if anthropicReq.Metadata != nil {
		chatReq.Metadata["user_id"] = anthropicReq.Metadata.UserID
	}

	// Convert messages
	messages := make([]model.Message, 0, len(anthropicReq.Messages))

	// Add system message if present
	if anthropicReq.System != nil {
		if anthropicReq.System.Prompt != nil {
			systemContent := anthropicReq.System.Prompt
			messages = append(messages, model.Message{
				Role: "system",
				Content: model.MessageContent{
					Content: systemContent,
				},
			})
			i.inputToken += int64(tokenizer.CountTokens(*systemContent, chatReq.Model))
		} else if len(anthropicReq.System.MultiplePrompts) > 0 {
			// Mark that system was originally in array format
			chatReq.TransformerMetadata["anthropic_system_array_format"] = "true"

			for _, prompt := range anthropicReq.System.MultiplePrompts {
				msg := model.Message{
					Role: "system",
					Content: model.MessageContent{
						Content: &prompt.Text,
					},
					CacheControl: convertToLLMCacheControl(prompt.CacheControl),
				}
				i.inputToken += int64(tokenizer.CountTokens(prompt.Text, chatReq.Model))
				messages = append(messages, msg)
			}
		}
	}

	// Convert Anthropic messages to ChatCompletionMessage
	for msgIndex, msg := range anthropicReq.Messages {
		chatMsg := model.Message{
			Role: msg.Role,
		}

		var (
			hasContent    bool
			hasToolResult bool
		)

		// Convert content

		if msg.Content.Content != nil {
			chatMsg.Content = model.MessageContent{
				Content: msg.Content.Content,
			}
			hasContent = true
			i.inputToken += int64(tokenizer.CountTokens(*msg.Content.Content, chatReq.Model))
		} else if len(msg.Content.MultipleContent) > 0 {
			contentParts := make([]model.MessageContentPart, 0, len(msg.Content.MultipleContent))

			var (
				reasoningContent      string
				hasReasoningInContent bool
			)

			var reasoningSignature string

			for _, block := range msg.Content.MultipleContent {
				switch block.Type {
				case "thinking":
					// Keep thinking content in MultipleContent to preserve order
					if block.Thinking != nil && *block.Thinking != "" {
						reasoningContent = *block.Thinking
						hasReasoningInContent = true
					}

					if block.Signature != nil && *block.Signature != "" {
						reasoningSignature = *block.Signature
					}
				case "text":
					contentParts = append(contentParts, model.MessageContentPart{
						Type:         "text",
						Text:         block.Text,
						CacheControl: convertToLLMCacheControl(block.CacheControl),
					})
					i.inputToken += int64(tokenizer.CountTokens(*block.Text, chatReq.Model))
					hasContent = true
				case "image":
					if block.Source != nil {
						part := model.MessageContentPart{
							Type:         "image_url",
							CacheControl: convertToLLMCacheControl(block.CacheControl),
						}
						if block.Source.Type == "base64" {
							// Convert Anthropic image format to OpenAI format
							imageURL := fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, block.Source.Data)
							part.ImageURL = &model.ImageURL{
								URL: imageURL,
							}
						} else {
							part.ImageURL = &model.ImageURL{
								URL: block.Source.URL,
							}
						}

						contentParts = append(contentParts, part)
						hasContent = true
					}
				case "tool_result":
					hasToolResult = true
					if block.Content != nil {
						toolMsg := model.Message{
							Role:            "tool",
							MessageIndex:    lo.ToPtr(msgIndex),
							ToolCallID:      block.ToolUseID,
							CacheControl:    convertToLLMCacheControl(block.CacheControl),
							ToolCallIsError: block.IsError,
						}

						if block.Content.Content != nil {
							toolMsg.Content = model.MessageContent{
								Content: block.Content.Content,
							}
						} else if len(block.Content.MultipleContent) > 0 {
							// Handle multiple content blocks in tool_result
							toolContentParts := make([]model.MessageContentPart, 0, len(block.Content.MultipleContent))
							for _, contentBlock := range block.Content.MultipleContent {
								switch contentBlock.Type {
								case "text":
									toolContentParts = append(toolContentParts, model.MessageContentPart{
										Type: "text",
										Text: contentBlock.Text,
									})
									if contentBlock.Text != nil {
										i.inputToken += int64(tokenizer.CountTokens(*contentBlock.Text, chatReq.Model))
									}
								case "image":
									if contentBlock.Source != nil && contentBlock.Source.Type == "base64" {
										imageURL := fmt.Sprintf("data:%s;base64,%s", contentBlock.Source.MediaType, contentBlock.Source.Data)
										toolContentParts = append(toolContentParts, model.MessageContentPart{
											Type: "image_url",
											ImageURL: &model.ImageURL{
												URL: imageURL,
											},
										})
									}
								default:
									// Preserve unknown content types as-is for forward compatibility
									if contentBlock.Text != nil {
										toolContentParts = append(toolContentParts, model.MessageContentPart{
											Type: contentBlock.Type,
											Text: contentBlock.Text,
										})
									}
								}
							}

							toolMsg.Content = model.MessageContent{
								MultipleContent: toolContentParts,
							}
						}

						messages = append(messages, toolMsg)
					}
				case "tool_use":
					chatMsg.ToolCalls = append(chatMsg.ToolCalls, model.ToolCall{
						ID:   block.ID,
						Type: "function",
						Function: model.FunctionCall{
							Name:      lo.FromPtr(block.Name),
							Arguments: string(block.Input),
						},
						CacheControl: convertToLLMCacheControl(block.CacheControl),
					})
					hasContent = true
				}
			}

			// Check if it's a simple text-only message (single text block)
			if len(contentParts) == 1 && contentParts[0].Type == "text" {
				// Convert single text block to simple content format for compatibility
				chatMsg.Content = model.MessageContent{
					Content: contentParts[0].Text,
				}
				// Preserve cache control at message level when simplifying
				if contentParts[0].CacheControl != nil {
					chatMsg.CacheControl = contentParts[0].CacheControl
				}

				hasContent = true
			} else if len(contentParts) > 0 {
				chatMsg.Content = model.MessageContent{
					MultipleContent: contentParts,
				}
				hasContent = true
			}

			// Assign reasoning content and signature if present
			if reasoningContent != "" && hasReasoningInContent {
				chatMsg.ReasoningContent = &reasoningContent
			}

			if reasoningSignature != "" {
				chatMsg.ReasoningSignature = &reasoningSignature
			}
		}

		if !hasContent {
			continue
		}

		// If this message had tool_result blocks, set MessageIndex so we can match it later
		if hasToolResult {
			chatMsg.MessageIndex = lo.ToPtr(msgIndex)
		}

		messages = append(messages, chatMsg)
	}

	chatReq.Messages = messages

	// Convert tools
	if len(anthropicReq.Tools) > 0 {
		tools := make([]model.Tool, 0, len(anthropicReq.Tools))
		for _, tool := range anthropicReq.Tools {
			llmTool := model.Tool{
				Type: "function",
				Function: model.Function{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
				CacheControl: convertToLLMCacheControl(tool.CacheControl),
			}
			tools = append(tools, llmTool)
			i.inputToken += int64(tokenizer.CountTokens(tool.Name, chatReq.Model))
			i.inputToken += int64(tokenizer.CountTokens(tool.Description, chatReq.Model))
			i.inputToken += int64(tokenizer.CountTokens(string(tool.InputSchema), chatReq.Model))
		}
		i.inputToken += int64(len(tools) * 3)

		chatReq.Tools = tools
	}

	// Convert stop sequences
	if len(anthropicReq.StopSequences) > 0 {
		if len(anthropicReq.StopSequences) == 1 {
			chatReq.Stop = &model.Stop{
				Stop: &anthropicReq.StopSequences[0],
			}
		} else {
			chatReq.Stop = &model.Stop{
				MultipleStop: anthropicReq.StopSequences,
			}
		}
	}

	// Convert thinking configuration to reasoning effort and preserve budget
	if anthropicReq.Thinking != nil {
		switch anthropicReq.Thinking.Type {
		case ThinkingTypeEnabled:
			if anthropicReq.Thinking.BudgetTokens != nil {
				chatReq.ReasoningEffort = thinkingBudgetToReasoningEffort(*anthropicReq.Thinking.BudgetTokens)
				chatReq.ReasoningBudget = anthropicReq.Thinking.BudgetTokens
			} else {
				log.Warnf("thinking type is 'enabled' but budget_tokens is nil, thinking will be ignored")
			}
		case ThinkingTypeAdaptive:
			effort := EffortHigh
			if anthropicReq.OutputConfig != nil && anthropicReq.OutputConfig.Effort != "" {
				effort = anthropicReq.OutputConfig.Effort
			}
			chatReq.ReasoningEffort = effort
			chatReq.AdaptiveThinking = true
		case ThinkingTypeDisabled:
			// Explicitly disabled, nothing to do
		default:
			log.Warnf("unknown thinking type: %s", anthropicReq.Thinking.Type)
		}
	}
	return chatReq, nil
}

func (i *MessagesInbound) TransformResponse(ctx context.Context, response *model.InternalLLMResponse) ([]byte, error) {
	// Store the response for later retrieval
	i.storedResponse = response

	resp := &Message{
		ID:    response.ID,
		Type:  "message",
		Role:  "assistant",
		Model: response.Model,
	}

	// Convert choices to content blocks
	if len(response.Choices) > 0 {
		choice := response.Choices[0]

		var message *model.Message

		if choice.Message != nil {
			message = choice.Message
		} else if choice.Delta != nil {
			message = choice.Delta
		}

		if message != nil {
			var contentBlocks []MessageContentBlock

			// Handle reasoning content (thinking) first if present
			if message.ReasoningContent != nil && *message.ReasoningContent != "" {
				thinkingBlock := MessageContentBlock{
					Type:     "thinking",
					Thinking: message.ReasoningContent,
				}
				if message.ReasoningSignature != nil && *message.ReasoningSignature != "" {
					thinkingBlock.Signature = message.ReasoningSignature
				} else {
					thinkingBlock.Signature = lo.ToPtr("ANTHROPIC_MAGIC_STRING_TRIGGER_REDACTED_THINKING_46C9A13E193C177646C7398A98432ECCCE4C1253D5E2D82641AC0E52CC2876CB")
				}

				contentBlocks = append(contentBlocks, thinkingBlock)
			}

			// Handle regular content
			if message.Content.Content != nil && *message.Content.Content != "" {
				contentBlocks = append(contentBlocks, MessageContentBlock{
					Type: "text",
					Text: message.Content.Content,
				})
			} else if len(message.Content.MultipleContent) > 0 {
				for _, part := range message.Content.MultipleContent {
					switch part.Type {
					case "text":
						if part.Text != nil {
							contentBlocks = append(contentBlocks, MessageContentBlock{
								Type: "text",
								Text: part.Text,
							})
						}
					case "image_url":
						if part.ImageURL != nil && part.ImageURL.URL != "" {
							// Convert OpenAI image format to Anthropic format
							url := part.ImageURL.URL
							if parsed := xurl.ParseDataURL(url); parsed != nil {
								contentBlocks = append(contentBlocks, MessageContentBlock{
									Type: "image",
									Source: &ImageSource{
										Type:      "base64",
										MediaType: parsed.MediaType,
										Data:      parsed.Data,
									},
								})
							} else {
								contentBlocks = append(contentBlocks, MessageContentBlock{
									Type: "image",
									Source: &ImageSource{
										Type: "url",
										URL:  part.ImageURL.URL,
									},
								})
							}
						}
					}
				}
			}

			// Handle tool calls
			if len(message.ToolCalls) > 0 {
				for _, toolCall := range message.ToolCalls {
					var input transformer.RawMessage
					if toolCall.Function.Arguments != "" {
						// Attempt to use the provided arguments; repair if invalid, fallback to {}
						if transformer.Valid([]byte(toolCall.Function.Arguments)) {
							input = transformer.RawMessage(toolCall.Function.Arguments)
						} else {
							input = transformer.RawMessage("{}")
						}
					} else {
						input = transformer.RawMessage("{}")
					}

					contentBlocks = append(contentBlocks, MessageContentBlock{
						Type:  "tool_use",
						ID:    toolCall.ID,
						Name:  &toolCall.Function.Name,
						Input: input,
					})
				}
			}

			resp.Content = contentBlocks
		}

		// Convert finish reason
		if choice.FinishReason != nil {
			switch *choice.FinishReason {
			case "stop":
				stopReason := "end_turn"
				resp.StopReason = &stopReason
			case "length":
				stopReason := "max_tokens"
				resp.StopReason = &stopReason
			case "tool_calls":
				stopReason := "tool_use"
				resp.StopReason = &stopReason
			default:
				resp.StopReason = choice.FinishReason
			}
		}
	}

	// Convert usage
	if response.Usage != nil {
		usage := &Usage{
			InputTokens:  response.Usage.PromptTokens,
			OutputTokens: response.Usage.CompletionTokens,
		}
		if response.Usage.PromptTokensDetails != nil {
			usage.CacheReadInputTokens = response.Usage.PromptTokensDetails.CachedTokens
			usage.InputTokens -= usage.CacheReadInputTokens
		}
		resp.Usage = usage
	}

	return transformer.Marshal(resp)
}

func (i *MessagesInbound) TransformStream(ctx context.Context, stream *model.InternalLLMResponse) ([]byte, error) {
	// [DONE] marker: upstream terminal or relay-synthesized EOF. Finalizes the
	// protocol even when no usage ever arrived.
	if stream.Object == "[DONE]" {
		if i.messageStopped || !i.hasStarted {
			return nil, nil
		}
		events, err := i.finalizeMessage()
		if err != nil || len(events) == 0 {
			return nil, err
		}
		return joinSSEEvents(events), nil
	}

	// Store the chunk for aggregation
	i.streamChunks = append(i.streamChunks, stream)

	var events [][]byte

	// Initialize message ID and model from first chunk
	if i.messageID == "" && stream.ID != "" {
		i.messageID = stream.ID
	}
	if i.modelName == "" && stream.Model != "" {
		i.modelName = stream.Model
	}

	// Generate message_start event if this is the first chunk
	if !i.hasStarted {
		i.hasStarted = true

		usage := &Usage{
			InputTokens:  i.inputToken,
			OutputTokens: 1,
		}
		if stream.Usage != nil {
			usage = i.convertUsage(stream.Usage)
		}

		startEvent := StreamEvent{
			Type: "message_start",
			Message: &StreamMessage{
				ID:      i.messageID,
				Type:    "message",
				Role:    "assistant",
				Model:   i.modelName,
				Content: []MessageContentBlock{},
				Usage:   usage,
			},
		}

		data, err := transformer.Marshal(startEvent)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal message_start event: %w", err)
		}
		events = append(events, formatSSEEvent("message_start", data))
	}

	// Cache usage whenever it appears; the terminal message_delta carries it.
	if stream.Usage != nil {
		i.terminalUsage = stream.Usage
	}

	// Process the current chunk
	if len(stream.Choices) > 0 {
		choice := stream.Choices[0]

		// Handle reasoning content (thinking) delta
		if choice.Delta != nil && choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
			i.stopTextBlock(&events)

			if i.thinkingBlockIndex == nil {
				idx := i.nextIndex
				i.thinkingBlockIndex = &idx
				i.nextIndex++

				startEvent := StreamEvent{
					Type:  "content_block_start",
					Index: i.thinkingBlockIndex,
					ContentBlock: &MessageContentBlock{
						Type:      "thinking",
						Thinking:  lo.ToPtr(""),
						Signature: lo.ToPtr(""),
					},
				}
				data, err := transformer.Marshal(startEvent)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal content_block_start event: %w", err)
				}
				events = append(events, formatSSEEvent("content_block_start", data))
			}

			deltaEvent := StreamEvent{
				Type:  "content_block_delta",
				Index: i.thinkingBlockIndex,
				Delta: &StreamDelta{
					Type:     lo.ToPtr("thinking_delta"),
					Thinking: choice.Delta.ReasoningContent,
				},
			}
			data, err := transformer.Marshal(deltaEvent)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal content_block_delta event: %w", err)
			}
			events = append(events, formatSSEEvent("content_block_delta", data))
		}

		// Add signature delta if signature is available; it targets the active
		// thinking block only.
		if choice.Delta != nil && choice.Delta.ReasoningSignature != nil && *choice.Delta.ReasoningSignature != "" && i.thinkingBlockIndex != nil {
			sigEvent := StreamEvent{
				Type:  "content_block_delta",
				Index: i.thinkingBlockIndex,
				Delta: &StreamDelta{
					Type:      lo.ToPtr("signature_delta"),
					Signature: choice.Delta.ReasoningSignature,
				},
			}
			data, err := transformer.Marshal(sigEvent)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal signature_delta event: %w", err)
			}
			events = append(events, formatSSEEvent("content_block_delta", data))
		}

		// Handle content delta. Text and tool arguments may interleave; text
		// gets its own stable block and never closes open tool blocks.
		if choice.Delta != nil && choice.Delta.Content.Content != nil && *choice.Delta.Content.Content != "" {
			i.stopThinkingBlock(&events)

			if i.textBlockIndex == nil {
				idx := i.nextIndex
				i.textBlockIndex = &idx
				i.nextIndex++

				startEvent := StreamEvent{
					Type:  "content_block_start",
					Index: i.textBlockIndex,
					ContentBlock: &MessageContentBlock{
						Type: "text",
						Text: lo.ToPtr(""),
					},
				}
				data, err := transformer.Marshal(startEvent)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal content_block_start event: %w", err)
				}
				events = append(events, formatSSEEvent("content_block_start", data))
			}

			deltaEvent := StreamEvent{
				Type:  "content_block_delta",
				Index: i.textBlockIndex,
				Delta: &StreamDelta{
					Type: lo.ToPtr("text_delta"),
					Text: choice.Delta.Content.Content,
				},
			}
			data, err := transformer.Marshal(deltaEvent)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal content_block_delta event: %w", err)
			}
			events = append(events, formatSSEEvent("content_block_delta", data))
		}

		// Handle tool calls: one stable block per OpenAI tool index, blocks
		// stay open concurrently until finish/DONE.
		if choice.Delta != nil && len(choice.Delta.ToolCalls) > 0 {
			i.stopThinkingBlock(&events)
			// Note: text block stays open alongside tool blocks.

			if i.toolBlockIndex == nil {
				i.toolBlockIndex = make(map[int]int64)
			}
			if i.toolBlockStopped == nil {
				i.toolBlockStopped = make(map[int]bool)
			}

			for _, deltaToolCall := range choice.Delta.ToolCalls {
				toolCallIndex := deltaToolCall.Index

				if _, seen := i.toolBlockIndex[toolCallIndex]; !seen {
					i.toolBlockIndex[toolCallIndex] = i.nextIndex
					i.toolBlockStopped[toolCallIndex] = false
					i.nextIndex++

					startIndex := i.toolBlockIndex[toolCallIndex]
					startEvent := StreamEvent{
						Type:  "content_block_start",
						Index: &startIndex,
						ContentBlock: &MessageContentBlock{
							Type:  "tool_use",
							ID:    deltaToolCall.ID,
							Name:  &deltaToolCall.Function.Name,
							Input: transformer.RawMessage("{}"),
						},
					}
					data, err := transformer.Marshal(startEvent)
					if err != nil {
						return nil, fmt.Errorf("failed to marshal content_block_start event: %w", err)
					}
					events = append(events, formatSSEEvent("content_block_start", data))
				}

				// Route fragments to the owning tool's block; drop them after
				// the block was stopped.
				if deltaToolCall.Function.Arguments != "" && !i.toolBlockStopped[toolCallIndex] {
					idx := i.toolBlockIndex[toolCallIndex]
					deltaEvent := StreamEvent{
						Type:  "content_block_delta",
						Index: &idx,
						Delta: &StreamDelta{
							Type:        lo.ToPtr("input_json_delta"),
							PartialJSON: &deltaToolCall.Function.Arguments,
						},
					}
					data, err := transformer.Marshal(deltaEvent)
					if err != nil {
						return nil, fmt.Errorf("failed to marshal content_block_delta event: %w", err)
					}
					events = append(events, formatSSEEvent("content_block_delta", data))
				}
			}
		}

		// Handle finish reason
		if choice.FinishReason != nil && !i.hasFinished {
			i.hasFinished = true

			var stopReason string
			switch *choice.FinishReason {
			case "stop":
				stopReason = "end_turn"
			case "length":
				stopReason = "max_tokens"
			case "tool_calls":
				stopReason = "tool_use"
			default:
				stopReason = "end_turn"
			}

			// Final usage is only known at the protocol terminal marker.
			i.stopReason = &stopReason

			i.closeAllBlocks(&events)
		}
	}

	if len(events) == 0 {
		return nil, nil
	}

	return joinSSEEvents(events), nil
}

func (i *MessagesInbound) finalizeMessage() ([][]byte, error) {
	if i.messageStopped {
		return nil, nil
	}

	var events [][]byte

	i.closeAllBlocks(&events)

	// No finish reason ever arrived (EOF without finish): still emit a
	// terminal pair so the client protocol completes.
	stopReason := "end_turn"
	if i.stopReason != nil {
		stopReason = *i.stopReason
	} else if len(i.toolBlockIndex) > 0 {
		stopReason = "tool_use"
	}

	msgDeltaEvent := StreamEvent{
		Type: "message_delta",
		Delta: &StreamDelta{
			StopReason: &stopReason,
		},
	}

	if i.terminalUsage != nil {
		msgDeltaEvent.Usage = i.convertUsage(i.terminalUsage)
	}

	data, err := transformer.Marshal(msgDeltaEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message_delta event: %w", err)
	}
	events = append(events, formatSSEEvent("message_delta", data))

	// Generate message_stop
	msgStopEvent := StreamEvent{
		Type: "message_stop",
	}
	data, err = transformer.Marshal(msgStopEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message_stop event: %w", err)
	}
	events = append(events, formatSSEEvent("message_stop", data))

	i.messageStopped = true

	return events, nil
}

// closeAllBlocks closes every still-open block, each exactly once.
func (i *MessagesInbound) closeAllBlocks(events *[][]byte) {
	i.stopThinkingBlock(events)
	i.stopTextBlock(events)
	i.stopAllToolBlocks(events)
}

// stopThinkingBlock closes the thinking block, if open.
func (i *MessagesInbound) stopThinkingBlock(events *[][]byte) {
	if i.thinkingBlockIndex == nil {
		return
	}
	idx := *i.thinkingBlockIndex
	i.thinkingBlockIndex = nil

	stopEvent := StreamEvent{
		Type:  "content_block_stop",
		Index: &idx,
	}
	data, err := transformer.Marshal(stopEvent)
	if err != nil {
		return
	}
	*events = append(*events, formatSSEEvent("content_block_stop", data))
}

// stopTextBlock closes the text block, if open.
func (i *MessagesInbound) stopTextBlock(events *[][]byte) {
	if i.textBlockIndex == nil {
		return
	}
	idx := *i.textBlockIndex
	i.textBlockIndex = nil

	stopEvent := StreamEvent{
		Type:  "content_block_stop",
		Index: &idx,
	}
	data, err := transformer.Marshal(stopEvent)
	if err != nil {
		return
	}
	*events = append(*events, formatSSEEvent("content_block_stop", data))
}

// stopAllToolBlocks closes every open tool block exactly once, in ascending
// tool index order.
func (i *MessagesInbound) stopAllToolBlocks(events *[][]byte) {
	if len(i.toolBlockIndex) == 0 {
		return
	}

	indices := make([]int, 0, len(i.toolBlockIndex))
	for toolIdx := range i.toolBlockIndex {
		if !i.toolBlockStopped[toolIdx] {
			indices = append(indices, toolIdx)
		}
	}
	sort.Ints(indices)

	for _, toolIdx := range indices {
		i.toolBlockStopped[toolIdx] = true
		idx := i.toolBlockIndex[toolIdx]

		stopEvent := StreamEvent{
			Type:  "content_block_stop",
			Index: &idx,
		}
		data, err := transformer.Marshal(stopEvent)
		if err != nil {
			continue
		}
		*events = append(*events, formatSSEEvent("content_block_stop", data))
	}
}

// joinSSEEvents concatenates events with newlines for SSE format.
func joinSSEEvents(events [][]byte) []byte {
	result := make([]byte, 0)
	for idx, event := range events {
		if idx > 0 {
			result = append(result, '\n')
		}
		result = append(result, event...)
	}
	return result
}

func (i *MessagesInbound) convertUsage(usage *model.Usage) *Usage {
	anthropicUsage := &Usage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
	}
	if usage.PromptTokensDetails != nil {
		anthropicUsage.CacheReadInputTokens = usage.PromptTokensDetails.CachedTokens
		anthropicUsage.InputTokens -= anthropicUsage.CacheReadInputTokens
	}
	return anthropicUsage
}

// GetInternalResponse returns the complete internal response for logging, statistics, etc.
// For streaming: aggregates all stored stream chunks into a complete response
// For non-streaming: returns the stored response
func (i *MessagesInbound) GetInternalResponse(ctx context.Context) (*model.InternalLLMResponse, error) {
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
		// Update ID and Model if they appear in later chunks
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

				// Append content
				if delta.Content.Content != nil {
					if existingChoice.Message.Content.Content == nil {
						existingChoice.Message.Content.Content = new(string)
					}
					*existingChoice.Message.Content.Content += *delta.Content.Content
				}

				// Append reasoning content
				if delta.ReasoningContent != nil {
					if existingChoice.Message.ReasoningContent == nil {
						existingChoice.Message.ReasoningContent = new(string)
					}
					*existingChoice.Message.ReasoningContent += *delta.ReasoningContent
				}

				// Aggregate tool calls
				for _, toolCall := range delta.ToolCalls {
					existingChoice.Message.ToolCalls = mergeToolCall(existingChoice.Message.ToolCalls, toolCall)
				}

				// Set refusal if present
				if delta.Refusal != "" {
					existingChoice.Message.Refusal = delta.Refusal
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

// formatSSEEvent 格式化为完整的 SSE 事件格式
func formatSSEEvent(eventType string, data []byte) []byte {
	return []byte(fmt.Sprintf("event:%s\ndata:%s\n\n", eventType, string(data)))
}
