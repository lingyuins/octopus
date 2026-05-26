package relay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/transformer/inbound"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/lingyuins/octopus/internal/utils/semantic_cache"
)

const (
	semanticCacheNamespaceMetadataKey = "semantic_cache_namespace"
	semanticCacheTextMetadataKey      = "semantic_cache_text"
	semanticCacheHitMetadataKey       = "semantic_cache_hit"
)

func semanticCacheEndpointFamily(endpointType string, inboundType inbound.InboundType) string {
	switch {
	case endpointType == dbmodel.EndpointTypeChat && inboundType == inbound.InboundTypeOpenAIChat:
		return "chat"
	case endpointType == dbmodel.EndpointTypeResponses && inboundType == inbound.InboundTypeOpenAIResponse:
		return "responses"
	default:
		return ""
	}
}

func semanticLookupCacheKey(apiKeyID int, endpointFamily string, req *transmodel.InternalLLMRequest) string {
	if req == nil {
		return ""
	}
	return semantic_cache.BuildNamespace(apiKeyID, endpointFamily, req.Model)
}

func buildSemanticCacheLookupInput(apiKeyID int, endpointFamily string, req *transmodel.InternalLLMRequest) (string, string, bool) {
	if req == nil || apiKeyID <= 0 {
		semantic_cache.RecordBypass()
		return "", "", false
	}
	if strings.TrimSpace(endpointFamily) == "" || strings.TrimSpace(req.Model) == "" {
		semantic_cache.RecordBypass()
		return "", "", false
	}
	if req.Stream != nil && *req.Stream {
		semantic_cache.RecordBypass()
		return "", "", false
	}

	text, ok := semantic_cache.ExtractNormalizedText(req)
	if !ok {
		semantic_cache.RecordBypass()
		return "", "", false
	}

	return semantic_cache.BuildNamespace(apiKeyID, endpointFamily, req.Model), text, true
}

func getSemanticCacheLookupInput(req *relayRequest, endpointFamily string) (string, string, bool, bool) {
	if req == nil || req.internalRequest == nil {
		return "", "", false, false
	}
	cacheKey := semanticLookupCacheKey(req.apiKeyID, endpointFamily, req.internalRequest)
	if req.retryCache == nil || cacheKey == "" {
		namespace, text, ok := buildSemanticCacheLookupInput(req.apiKeyID, endpointFamily, req.internalRequest)
		return namespace, text, ok, false
	}
	return req.retryCache.getLookupInput(cacheKey, func() (string, string, bool) {
		return buildSemanticCacheLookupInput(req.apiKeyID, endpointFamily, req.internalRequest)
	})
}

func maybeServeSemanticCacheHit(c *gin.Context, req *relayRequest, endpointFamily string) (bool, []byte, error) {
	if c == nil || req == nil || req.internalRequest == nil {
		return false, nil, nil
	}

	namespace, text, ok, _ := getSemanticCacheLookupInput(req, endpointFamily)
	if !ok {
		return false, nil, nil
	}

	cfg, ok := loadSemanticCacheRuntimeConfig()
	if !ok {
		semantic_cache.RecordBypass()
		return false, nil, nil
	}
	ensureSemanticCacheInitialized(cfg)

	embedding, _, err := lookupSemanticEmbeddingWithCache(req.operationCtx, req, cfg, namespace, text)
	if err != nil {
		semantic_cache.RecordBypass()
		log.Warnf("semantic cache lookup bypassed: %v", err)
		return false, nil, nil
	}

	semantic_cache.RecordEvaluated()
	if payload, found := semantic_cache.Lookup(namespace, embedding); found {
		semantic_cache.RecordHit()
		if req.internalRequest.TransformerMetadata == nil {
			req.internalRequest.TransformerMetadata = make(map[string]string, 1)
		}
		req.internalRequest.TransformerMetadata[semanticCacheHitMetadataKey] = "true"
		c.Data(http.StatusOK, "application/json", payload)
		return true, payload, nil
	}
	semantic_cache.RecordMiss()

	if req.internalRequest.TransformerMetadata == nil {
		req.internalRequest.TransformerMetadata = make(map[string]string, 2)
	}
	req.internalRequest.TransformerMetadata[semanticCacheNamespaceMetadataKey] = namespace
	req.internalRequest.TransformerMetadata[semanticCacheTextMetadataKey] = text

	return false, nil, nil
}

func storeSemanticCacheResponse(ctx context.Context, req *transmodel.InternalLLMRequest, responseJSON []byte) {
	if req == nil || len(responseJSON) == 0 || !json.Valid(responseJSON) {
		return
	}

	namespace, text, ok := semanticCacheStoreMetadata(req)
	if !ok {
		return
	}

	cfg, ok := loadSemanticCacheRuntimeConfig()
	if !ok {
		return
	}
	ensureSemanticCacheInitialized(cfg)

	embedding, err := semantic_cache.NewEmbeddingClient(cfg).CreateEmbedding(ctx, text)
	if err != nil {
		log.Warnf("semantic cache store bypassed: %v", err)
		return
	}

	semantic_cache.Store(namespace, text, responseJSON, embedding)
	semantic_cache.RecordStored()
}

func isSemanticCacheHitRequest(req *transmodel.InternalLLMRequest) bool {
	if req == nil || req.TransformerMetadata == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(req.TransformerMetadata[semanticCacheHitMetadataKey]), "true")
}

func semanticCacheHitPayload(responseJSON []byte, req *transmodel.InternalLLMRequest) []byte {
	if len(responseJSON) == 0 || !json.Valid(responseJSON) {
		return responseJSON
	}

	var payload map[string]any
	if err := json.Unmarshal(responseJSON, &payload); err != nil {
		return responseJSON
	}

	octopusValue, ok := payload["octopus"].(map[string]any)
	if !ok {
		octopusValue = map[string]any{}
		payload["octopus"] = octopusValue
	}
	octopusValue["semantic_cache"] = map[string]any{
		"hit": true,
	}

	if usageValue, ok := payload["usage"].(map[string]any); ok {
		delete(usageValue, "cached_tokens")
		delete(usageValue, "prompt_cache_hit_tokens")
		if promptDetails, ok := usageValue["input_token_details"].(map[string]any); ok {
			delete(promptDetails, "cached_tokens")
		}
		if promptDetails, ok := usageValue["prompt_tokens_details"].(map[string]any); ok {
			delete(promptDetails, "cached_tokens")
		}
		if inputDetails, ok := usageValue["input_tokens_details"].(map[string]any); ok {
			delete(inputDetails, "cached_tokens")
		}
	}

	normalized, err := json.Marshal(payload)
	if err != nil {
		return responseJSON
	}
	return normalized
}

func buildSemanticCacheHitInternalResponse(req *transmodel.InternalLLMRequest, payload []byte) (*transmodel.InternalLLMResponse, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}

	internalResponse := &transmodel.InternalLLMResponse{}
	switch req.RawAPIFormat {
	case transmodel.APIFormatOpenAIResponse:
		var respPayload struct {
			ID        string `json:"id"`
			Object    string `json:"object"`
			CreatedAt int64  `json:"created_at"`
			Model     string `json:"model"`
			Usage     *struct {
				InputTokens int64 `json:"input_tokens"`
				OutputTokens int64 `json:"output_tokens"`
				TotalTokens int64 `json:"total_tokens"`
				InputTokenDetails *struct {
					CachedTokens int64 `json:"cached_tokens"`
				} `json:"input_token_details"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(payload, &respPayload); err != nil {
			return nil, err
		}
		internalResponse.ID = respPayload.ID
		internalResponse.Object = "chat.completion"
		internalResponse.Created = respPayload.CreatedAt
		internalResponse.Model = respPayload.Model
		if respPayload.Usage != nil {
			internalResponse.Usage = &transmodel.Usage{
				PromptTokens:     respPayload.Usage.InputTokens,
				CompletionTokens: respPayload.Usage.OutputTokens,
				TotalTokens:      respPayload.Usage.TotalTokens,
			}
			if respPayload.Usage.InputTokenDetails != nil && respPayload.Usage.InputTokenDetails.CachedTokens > 0 {
				internalResponse.Usage.PromptTokensDetails = &transmodel.PromptTokensDetails{
					CachedTokens: respPayload.Usage.InputTokenDetails.CachedTokens,
				}
			}
		}
	default:
		if err := json.Unmarshal(payload, internalResponse); err != nil {
			return nil, err
		}
	}
	return internalResponse, nil
}

func semanticCacheStoreMetadata(req *transmodel.InternalLLMRequest) (string, string, bool) {
	if req == nil || req.TransformerMetadata == nil {
		return "", "", false
	}

	namespace := strings.TrimSpace(req.TransformerMetadata[semanticCacheNamespaceMetadataKey])
	text := strings.TrimSpace(req.TransformerMetadata[semanticCacheTextMetadataKey])
	if namespace == "" || text == "" {
		return "", "", false
	}

	return namespace, text, true
}

func loadSemanticCacheRuntimeConfig() (semantic_cache.RuntimeConfig, bool) {
	enabled, err := setting.GetBool(dbmodel.SettingKeySemanticCacheEnabled)
	if err != nil || !enabled {
		return semantic_cache.RuntimeConfig{}, false
	}

	ttl, _ := setting.GetInt(dbmodel.SettingKeySemanticCacheTTL)
	if ttl <= 0 {
		ttl = 3600
	}

	thresholdRaw, _ := setting.GetInt(dbmodel.SettingKeySemanticCacheThreshold)
	if thresholdRaw < 0 || thresholdRaw > 100 {
		thresholdRaw = 98
	}

	maxEntries, _ := setting.GetInt(dbmodel.SettingKeySemanticCacheMaxEntries)
	if maxEntries <= 0 {
		maxEntries = 1000
	}

	embeddingBaseURL, _ := setting.GetString(dbmodel.SettingKeySemanticCacheEmbeddingBaseURL)
	embeddingAPIKey, _ := setting.GetString(dbmodel.SettingKeySemanticCacheEmbeddingAPIKey)
	embeddingModel, _ := setting.GetString(dbmodel.SettingKeySemanticCacheEmbeddingModel)
	timeoutSeconds, _ := setting.GetInt(dbmodel.SettingKeySemanticCacheEmbeddingTimeoutSeconds)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 15
	}

	return semantic_cache.RuntimeConfig{
		Enabled:          true,
		MaxEntries:       maxEntries,
		Threshold:        float64(thresholdRaw) / 100.0,
		TTL:              time.Duration(ttl) * time.Second,
		EmbeddingBaseURL: strings.TrimSpace(embeddingBaseURL),
		EmbeddingAPIKey:  strings.TrimSpace(embeddingAPIKey),
		EmbeddingModel:   strings.TrimSpace(embeddingModel),
		EmbeddingTimeout: time.Duration(timeoutSeconds) * time.Second,
	}, true
}

func ensureSemanticCacheInitialized(cfg semantic_cache.RuntimeConfig) {
	semantic_cache.ApplyRuntimeConfig(cfg)
}

