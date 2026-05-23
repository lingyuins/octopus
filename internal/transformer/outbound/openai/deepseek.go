package openai

import (
	"encoding/json"
	"strings"

	"github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/utils/log"
)

func isDeepSeekCompatRequest(baseURL string, request *model.InternalLLMRequest) bool {
	return isProviderReasoningCompatRequest(baseURL, request, false, "deepseek")
}

func isMimoCompatRequest(baseURL string, request *model.InternalLLMRequest) bool {
	return isProviderReasoningCompatRequest(baseURL, request, true, "mimo")
}

func isReasoningCompatRequest(baseURL string, request *model.InternalLLMRequest, isMimoChannel bool) bool {
	return isProviderReasoningCompatRequest(baseURL, request, isMimoChannel, "deepseek") ||
		isProviderReasoningCompatRequest(baseURL, request, isMimoChannel, "mimo")
}

func isProviderReasoningCompatRequest(baseURL string, request *model.InternalLLMRequest, isMimoChannel bool, provider string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return false
	}

	if provider == "mimo" && isMimoChannel {
		return true
	}

	lowerBaseURL := strings.ToLower(strings.TrimSpace(baseURL))
	if lowerBaseURL != "" && strings.Contains(lowerBaseURL, provider) {
		return true
	}
	if request == nil {
		return false
	}

	if strings.EqualFold(
		strings.TrimSpace(request.TransformerMetadata[model.TransformerMetadataGroupEndpointType]),
		provider,
	) {
		return true
	}

	lowerModelName := strings.ToLower(strings.TrimSpace(request.Model))
	if strings.Contains(lowerModelName, provider) {
		return true
	}

	if provider == "mimo" {
		return strings.Contains(lowerModelName, "xiaomi")
	}

	return false
}

func normalizeDeepSeekReasoningCompat(request *model.InternalLLMRequest, baseURL string, isMimoChannel bool) {
	if request == nil || !isReasoningCompatRequest(baseURL, request, isMimoChannel) {
		return
	}

	thinkingType, hasThinkingType := extractDeepSeekThinkingType(request.ExtraBody)
	normalizedEffort := normalizeDeepSeekReasoningEffort(request.ReasoningEffort)
	originalEffort := strings.ToLower(strings.TrimSpace(request.ReasoningEffort))

	if hasThinkingType && thinkingType == "disabled" {
		request.ReasoningEffort = ""
	} else {
		request.ReasoningEffort = normalizedEffort
	}

	switch {
	case hasThinkingType && (thinkingType == "enabled" || thinkingType == "disabled"):
		request.ExtraBody = mergeDeepSeekThinkingExtraBody(request.ExtraBody, thinkingType)
	case originalEffort == "none":
		request.ExtraBody = mergeDeepSeekThinkingExtraBody(request.ExtraBody, "disabled")
	case request.ReasoningEffort != "":
		request.ExtraBody = mergeDeepSeekThinkingExtraBody(request.ExtraBody, "enabled")
	}
}

func normalizeDeepSeekReasoningEffort(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "", "none":
		return ""
	case "low", "medium", "high":
		return "high"
	case "xhigh", "max":
		return "max"
	default:
		return strings.TrimSpace(effort)
	}
}

func extractDeepSeekThinkingType(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", false
	}

	thinkingValue, ok := payload["thinking"]
	if !ok {
		return "", false
	}

	thinking, ok := thinkingValue.(map[string]any)
	if !ok {
		return "", false
	}

	typeValue, ok := thinking["type"]
	if !ok {
		return "", false
	}

	typeString, ok := typeValue.(string)
	if !ok {
		return "", false
	}

	normalized := strings.ToLower(strings.TrimSpace(typeString))
	return normalized, normalized != ""
}

func mergeDeepSeekThinkingExtraBody(raw json.RawMessage, thinkingType string) json.RawMessage {
	payload := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			log.Warnf("failed to unmarshal deepseek extra body: %v", err)
		}
	}

	payload["thinking"] = map[string]any{
		"type": thinkingType,
	}

	merged, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return merged
}
