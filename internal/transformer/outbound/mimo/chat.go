package mimo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	transformermodel "github.com/lingyuins/octopus/internal/transformer/model"
	openaioutbound "github.com/lingyuins/octopus/internal/transformer/outbound/openai"
)

type ChatOutbound struct {
	openaioutbound.ChatOutbound
}

func (o *ChatOutbound) TransformRequest(ctx context.Context, request *transformermodel.InternalLLMRequest, baseURL, key string) (*http.Request, error) {
	compatRequest := openaioutbound.CloneRequestForOpenAICompat(request)
	openaioutbound.SanitizeRequestForOpenAICompat(compatRequest, baseURL, true)

	for i := range compatRequest.Messages {
		if compatRequest.Messages[i].Role == "developer" {
			compatRequest.Messages[i].Role = "system"
		}
		promoteSingleTextContentToArray(&compatRequest.Messages[i])
	}

	openaioutbound.NormalizeMessagesForOpenAICompat(compatRequest.Messages)

	if compatRequest.Stream != nil && *compatRequest.Stream {
		if compatRequest.StreamOptions == nil {
			compatRequest.StreamOptions = &transformermodel.StreamOptions{IncludeUsage: true}
		} else if !compatRequest.StreamOptions.IncludeUsage {
			compatRequest.StreamOptions.IncludeUsage = true
		}
	}

	body, err := json.Marshal(compatRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("api-key", key)

	upstreamURL, err := openaioutbound.BuildOpenAIUpstreamURL(baseURL, "/v1/chat/completions")
	if err != nil {
		return nil, err
	}
	parsedURL, err := url.Parse(upstreamURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse built upstream url: %w", err)
	}
	req.URL = parsedURL
	req.Method = http.MethodPost
	return req, nil
}

func promoteSingleTextContentToArray(message *transformermodel.Message) {
	if message == nil || message.Content.Content == nil || len(message.Content.MultipleContent) > 0 {
		return
	}
	text := *message.Content.Content
	message.Content.Content = nil
	message.Content.MultipleContent = []transformermodel.MessageContentPart{{
		Type: "text",
		Text: &text,
	}}
}
