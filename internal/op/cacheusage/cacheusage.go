package cacheusage

import (
	"encoding/json"
	"strings"
)

type ProviderPromptCacheUsageSignals struct {
	PromptTokens             int64
	CachedTokens             int64
	CacheCreationInputTokens int64
	SemanticCacheHit         bool
}

type providerPromptCacheUsagePayload struct {
	Octopus *struct {
		SemanticCache *struct {
			Hit bool `json:"hit"`
		} `json:"semantic_cache"`
	} `json:"octopus"`
	Usage *struct {
		InputTokens        int64 `json:"input_tokens"`
		PromptTokens       int64 `json:"prompt_tokens"`
		CachedTokens       int64 `json:"cached_tokens"`
		PromptCacheHit     int64 `json:"prompt_cache_hit_tokens"`
		InputTokensDetails *struct {
			CachedTokens int64 `json:"cached_tokens"`
		} `json:"input_tokens_details"`
		InputTokenDetails *struct {
			CachedTokens int64 `json:"cached_tokens"`
		} `json:"input_token_details"`
		PromptTokensDetails *struct {
			CachedTokens int64 `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		CacheCreationInputTokens *int64 `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

func ParseProviderPromptCacheUsageSignals(responseContent string) (ProviderPromptCacheUsageSignals, bool) {
	responseContent = strings.TrimSpace(responseContent)
	if responseContent == "" {
		return ProviderPromptCacheUsageSignals{}, false
	}

	var payload providerPromptCacheUsagePayload
	if err := json.Unmarshal([]byte(responseContent), &payload); err != nil || payload.Usage == nil {
		return ProviderPromptCacheUsageSignals{}, false
	}

	usage := ProviderPromptCacheUsageSignals{
		PromptTokens: payload.Usage.InputTokens,
	}
	if usage.PromptTokens <= 0 {
		usage.PromptTokens = payload.Usage.PromptTokens
	}
	if payload.Usage.InputTokensDetails != nil {
		usage.CachedTokens = payload.Usage.InputTokensDetails.CachedTokens
	}
	if usage.CachedTokens <= 0 && payload.Usage.InputTokenDetails != nil {
		usage.CachedTokens = payload.Usage.InputTokenDetails.CachedTokens
	}
	if usage.CachedTokens <= 0 && payload.Usage.PromptTokensDetails != nil {
		usage.CachedTokens = payload.Usage.PromptTokensDetails.CachedTokens
	}
	if usage.CachedTokens <= 0 {
		usage.CachedTokens = payload.Usage.CachedTokens
	}
	if usage.CachedTokens <= 0 {
		usage.CachedTokens = payload.Usage.PromptCacheHit
	}
	if payload.Usage.CacheCreationInputTokens != nil {
		usage.CacheCreationInputTokens = *payload.Usage.CacheCreationInputTokens
	}
	if payload.Octopus != nil && payload.Octopus.SemanticCache != nil {
		usage.SemanticCacheHit = payload.Octopus.SemanticCache.Hit
	}

	if usage.PromptTokens <= 0 && usage.CachedTokens <= 0 && usage.CacheCreationInputTokens <= 0 && !usage.SemanticCacheHit {
		return ProviderPromptCacheUsageSignals{}, false
	}
	return usage, true
}
