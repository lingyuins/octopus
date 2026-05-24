package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

type ChatOutbound struct{}

func (o *ChatOutbound) TransformRequest(ctx context.Context, request *model.InternalLLMRequest, baseUrl, key string) (*http.Request, error) {
	compatRequest := cloneRequestForOpenAICompat(request)
	isMimoChannel := strings.Contains(strings.ToLower(strings.TrimSpace(baseUrl)), "xiaomimimo")
	sanitizeRequestForOpenAICompat(compatRequest, baseUrl, isMimoChannel)

	// Convert developer role to system role for compatibility
	for i := range compatRequest.Messages {
		if compatRequest.Messages[i].Role == "developer" {
			compatRequest.Messages[i].Role = "system"
		}
	}

	normalizeMessagesForOpenAICompat(compatRequest.Messages)

	if compatRequest.Stream != nil && *compatRequest.Stream {
		if compatRequest.StreamOptions == nil {
			compatRequest.StreamOptions = &model.StreamOptions{IncludeUsage: true}
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

	upstreamURL, err := buildOpenAIUpstreamURL(baseUrl, "/v1/chat/completions")
	if err != nil {
		return nil, err
	}
	parsedUrl, err := url.Parse(upstreamURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse built upstream url: %w", err)
	}
	req.URL = parsedUrl
	req.Method = http.MethodPost
	return req, nil
}

func cloneRequestForOpenAICompat(request *model.InternalLLMRequest) *model.InternalLLMRequest {
	if request == nil {
		return nil
	}

	cloned := *request
	if len(request.Messages) > 0 {
		cloned.Messages = append([]model.Message(nil), request.Messages...)
	}
	if request.StreamOptions != nil {
		streamOptions := *request.StreamOptions
		cloned.StreamOptions = &streamOptions
	}

	return &cloned
}

func sanitizeRequestForOpenAICompat(request *model.InternalLLMRequest, baseURL string, isMimoChannel bool) {
	if request == nil {
		return
	}

	normalizeDeepSeekReasoningCompat(request, baseURL, isMimoChannel)
	applyReasoningCompatTokenBudget(request, baseURL, isMimoChannel)

	// Only apply generic reasoning-effort normalization to non-provider-specific
	// reasoning-compatible targets. DeepSeek and Mimo already handle effort
	// mapping in the call above.
	if !isReasoningCompatRequest(baseURL, request, isMimoChannel) {
		request.ReasoningEffort = normalizeOpenAICompatReasoningEffort(request.ReasoningEffort)
	}

	preserveDeepSeekReasoning := shouldPreserveDeepSeekReasoning(baseURL, request, isMimoChannel)
	if preserveDeepSeekReasoning {
		attachStandaloneDeepSeekReasoningMessages(request)
	}

	for i := range request.Messages {
		sanitizeMessageForOpenAICompat(&request.Messages[i], preserveDeepSeekReasoning)
	}

	if !isReasoningCompatRequest(baseURL, request, isMimoChannel) {
		request.ExtraBody = nil
	}
	request.Include = nil
}

func applyReasoningCompatTokenBudget(request *model.InternalLLMRequest, baseURL string, isMimoChannel bool) {
	if request == nil || !isReasoningCompatRequest(baseURL, request, isMimoChannel) {
		return
	}

	const defaultReasoningMaxCompletionTokens int64 = 512

	if request.MaxCompletionTokens != nil {
		if *request.MaxCompletionTokens < defaultReasoningMaxCompletionTokens {
			v := defaultReasoningMaxCompletionTokens
			request.MaxCompletionTokens = &v
		}
		return
	}

	if request.MaxTokens != nil {
		if *request.MaxTokens < defaultReasoningMaxCompletionTokens {
			v := defaultReasoningMaxCompletionTokens
			request.MaxCompletionTokens = &v
			request.MaxTokens = nil
		}
		return
	}

	v := defaultReasoningMaxCompletionTokens
	request.MaxCompletionTokens = &v
}

func sanitizeMessageForOpenAICompat(msg *model.Message, preserveDeepSeekReasoning bool) {
	if msg == nil {
		return
	}

	reasoningContent := msg.GetReasoningContent()
	shouldKeepReasoning := shouldKeepDeepSeekReasoningContent(msg, preserveDeepSeekReasoning, reasoningContent)

	msg.ClearHelpFields()

	if shouldKeepReasoning {
		msg.ReasoningContent = &reasoningContent
	}
}

func shouldPreserveDeepSeekReasoning(baseURL string, request *model.InternalLLMRequest, isMimoChannel bool) bool {
	return isReasoningCompatRequest(baseURL, request, isMimoChannel)
}

func attachStandaloneDeepSeekReasoningMessages(request *model.InternalLLMRequest) {
	if request == nil || len(request.Messages) == 0 {
		return
	}

	mergedMessages := make([]model.Message, 0, len(request.Messages))
	pendingReasoning := ""
	lastReasoning := ""

	for _, msg := range request.Messages {
		reasoningContent := msg.GetReasoningContent()
		if isStandaloneDeepSeekReasoningMessage(msg, reasoningContent) {
			pendingReasoning += reasoningContent
			lastReasoning = pendingReasoning
			continue
		}

		if msg.Role == "assistant" && reasoningContent != "" {
			lastReasoning = reasoningContent
		}

		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 && msg.GetReasoningContent() == "" {
			if pendingReasoning != "" {
				attached := pendingReasoning
				msg.ReasoningContent = &attached
				lastReasoning = attached
				pendingReasoning = ""
			} else if lastReasoning != "" {
				attached := lastReasoning
				msg.ReasoningContent = &attached
			}
		}

		mergedMessages = append(mergedMessages, msg)
		if msg.Role != "assistant" {
			pendingReasoning = ""
		}
	}

	request.Messages = mergedMessages
}

func isStandaloneDeepSeekReasoningMessage(msg model.Message, reasoningContent string) bool {
	return msg.Role == "assistant" &&
		reasoningContent != "" &&
		msg.Content.Content == nil &&
		len(msg.Content.MultipleContent) == 0 &&
		len(msg.ToolCalls) == 0 &&
		msg.ToolCallID == nil &&
		msg.Refusal == "" &&
		len(msg.Images) == 0
}

func shouldKeepDeepSeekReasoningContent(msg *model.Message, preserveDeepSeekReasoning bool, reasoningContent string) bool {
	if !preserveDeepSeekReasoning || msg == nil || reasoningContent == "" || msg.Role != "assistant" {
		return false
	}

	// DeepSeek requires reasoning_content to be passed back for assistant
	// messages that contain tool_calls when thinking mode is enabled. Assistant
	// messages without tool calls may include reasoning_content, but DeepSeek
	// ignores it in later non-tool-call turns.
	return true
}

func normalizeMessagesForOpenAICompat(messages []model.Message) {
	for i := range messages {
		normalizeMessageForOpenAICompat(&messages[i])
	}
}

func normalizeMessageForOpenAICompat(msg *model.Message) {
	if len(msg.Content.MultipleContent) > 0 {
		if text, ok := flattenTextOnlyContent(msg.Content.MultipleContent); ok {
			msg.Content = model.MessageContent{
				Content: &text,
			}
		} else if msg.Role == "tool" {
			text := flattenTextContent(msg.Content.MultipleContent)
			msg.Content = model.MessageContent{
				Content: &text,
			}
		}
	}

	if msg.Content.Content == nil && len(msg.Content.MultipleContent) == 0 && msg.GetReasoningContent() == "" {
		empty := ""
		msg.Content.Content = &empty
	}
}

func flattenTextOnlyContent(parts []model.MessageContentPart) (string, bool) {
	if len(parts) == 0 {
		return "", false
	}

	textParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Type != "text" || part.Text == nil {
			return "", false
		}
		textParts = append(textParts, *part.Text)
	}

	return strings.Join(textParts, "\n"), true
}

func flattenTextContent(parts []model.MessageContentPart) string {
	if len(parts) == 0 {
		return ""
	}

	textParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Type == "text" && part.Text != nil {
			textParts = append(textParts, *part.Text)
		}
	}

	return strings.Join(textParts, "\n")
}

func (o *ChatOutbound) TransformResponse(ctx context.Context, response *http.Response) (*model.InternalLLMResponse, error) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var resp model.InternalLLMResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	normalizeOpenAICompatUsage(&resp)
	return &resp, nil
}

func (o *ChatOutbound) TransformStream(ctx context.Context, eventData []byte) (*model.InternalLLMResponse, error) {
	if bytes.HasPrefix(eventData, []byte("[DONE]")) {
		return &model.InternalLLMResponse{
			Object: "[DONE]",
		}, nil
	}

	var errCheck struct {
		Error *model.ErrorDetail `json:"error"`
	}
	if err := json.Unmarshal(eventData, &errCheck); err == nil && errCheck.Error != nil {
		return nil, &model.ResponseError{
			Detail: *errCheck.Error,
		}
	}

	var resp model.InternalLLMResponse
	if err := json.Unmarshal(eventData, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stream chunk: %w", err)
	}
	normalizeOpenAICompatUsage(&resp)
	return &resp, nil
}

func normalizeOpenAICompatUsage(resp *model.InternalLLMResponse) {
	if resp == nil || resp.Usage == nil {
		return
	}

	usage := resp.Usage
	if usage.CompletionTokensDetails == nil {
		return
	}

	reasoningTokens := usage.CompletionTokensDetails.ReasoningTokens
	if reasoningTokens < 0 {
		usage.CompletionTokensDetails.ReasoningTokens = 0
		reasoningTokens = 0
	}

	if usage.CompletionTokens < reasoningTokens {
		usage.CompletionTokens = reasoningTokens
	}

	if usage.TotalTokens > 0 && usage.PromptTokens > 0 {
		minimumTotal := usage.PromptTokens + usage.CompletionTokens
		if usage.TotalTokens < minimumTotal {
			usage.TotalTokens = minimumTotal
		}
	}
}

func buildOpenAIUpstreamURL(baseURL, endpointPath string) (string, error) {
	parsed, err := url.Parse(strings.TrimSuffix(baseURL, "/"))
	if err != nil {
		return "", fmt.Errorf("failed to parse base url: %w", err)
	}

	basePath := strings.TrimSuffix(parsed.Path, "/")
	normalizedPath := endpointPath
	if strings.HasSuffix(basePath, "/v1") && strings.HasPrefix(normalizedPath, "/v1/") {
		normalizedPath = strings.TrimPrefix(normalizedPath, "/v1")
	}
	if looksLikeExplicitEndpoint(basePath, normalizedPath) {
		parsed.Path = basePath
	} else {
		parsed.Path = basePath + normalizedPath
	}

	return parsed.String(), nil
}

func looksLikeExplicitEndpoint(basePath, endpointPath string) bool {
	if basePath == "" {
		return false
	}

	baseSegments := strings.Split(strings.Trim(basePath, "/"), "/")
	endpointSegments := strings.Split(strings.Trim(endpointPath, "/"), "/")
	if len(baseSegments) == 0 || len(endpointSegments) == 0 {
		return false
	}

	return strings.EqualFold(baseSegments[len(baseSegments)-1], endpointSegments[len(endpointSegments)-1])
}
