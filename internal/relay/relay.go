package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/helper"
	dbmodel "github.com/lingyuins/octopus/internal/model"
	ch "github.com/lingyuins/octopus/internal/op/channel"
	grp "github.com/lingyuins/octopus/internal/op/group"
	"github.com/lingyuins/octopus/internal/op/modelmapping"
	rl "github.com/lingyuins/octopus/internal/op/ratelimitstore"
	st "github.com/lingyuins/octopus/internal/op/stats"
	"github.com/lingyuins/octopus/internal/relay/balancer"
	"github.com/lingyuins/octopus/internal/relay/condition"
	"github.com/lingyuins/octopus/internal/relay/poolscheduler"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/transformer/inbound"
	"github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
	"github.com/lingyuins/octopus/internal/transformer/rewrite"
	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/lingyuins/octopus/internal/utils/semantic_cache"
	"github.com/tmaxmax/go-sse"
)

var errClientDisconnected = errors.New("client disconnected")
var errResponseFilterBlocked = errors.New("response filter blocked by keyword")

func resolveRequestedUpstreamModel(requestModel string) (string, bool) {
	trimmed := strings.TrimSpace(requestModel)
	if trimmed == "" {
		return "", false
	}
	prefix, upstream, ok := strings.Cut(trimmed, "/")
	if !ok {
		return "", false
	}
	if !strings.EqualFold(strings.TrimSpace(prefix), "zen") {
		return "", false
	}
	upstream = strings.TrimSpace(upstream)
	if upstream == "" {
		return "", false
	}
	return upstream, true
}

func detectZenPreferredChannelTypes(requestModel string, isEmbeddingRequest bool) map[outbound.OutboundType]struct{} {
	upstreamModel, ok := resolveRequestedUpstreamModel(requestModel)
	if !ok {
		return nil
	}
	if isEmbeddingRequest {
		return map[outbound.OutboundType]struct{}{
			outbound.OutboundTypeOpenAIEmbedding: {},
		}
	}

	lowerModel := strings.ToLower(strings.TrimSpace(upstreamModel))
	switch {
	case strings.HasPrefix(lowerModel, "claude"):
		return map[outbound.OutboundType]struct{}{
			outbound.OutboundTypeAnthropic: {},
		}
	case strings.HasPrefix(lowerModel, "gemini"), strings.HasPrefix(lowerModel, "models/gemini"), strings.HasPrefix(lowerModel, "gemma"):
		return map[outbound.OutboundType]struct{}{
			outbound.OutboundTypeGemini: {},
		}
	case strings.HasPrefix(lowerModel, "gpt-"), strings.HasPrefix(lowerModel, "o1"), strings.HasPrefix(lowerModel, "o3"), strings.HasPrefix(lowerModel, "o4"), strings.HasPrefix(lowerModel, "text-embedding"), strings.HasPrefix(lowerModel, "text-moderation"):
		return map[outbound.OutboundType]struct{}{
			outbound.OutboundTypeOpenAIChat:     {},
			outbound.OutboundTypeOpenAIResponse: {},
			outbound.OutboundTypeVolcengine:     {},
			outbound.OutboundTypeMimo:           {},
		}
	default:
		return nil
	}
}

func outboundAttemptTypes(channelType outbound.OutboundType, request *model.InternalLLMRequest, outboundFormat string) []outbound.OutboundType {
	return outbound.ResolveAttemptTypes(channelType, request, outboundFormat)
}

func shouldTryAdapterFallback(result attemptResult, adapterIndex, attemptCount int) bool {
	if result.Success || result.Written || adapterIndex >= attemptCount-1 {
		return false
	}
	// 只有路由级失败（换候选）才值得尝试另一种出站 adapter 格式。
	// 客户端错误 ScopeNone（如 context_length_exceeded 的 400）与 Key 级失败
	// ScopeSameChannel 都不该再换 adapter：前者必须立刻把上游错误体回给下游，
	// 后者换 adapter 只会用同一把 Key 多打一次，徒增延迟。
	return result.Decision.Scope == ScopeNextChannel
}
func isZenCandidateChannelAllowed(requestModel string, channelType outbound.OutboundType, isEmbeddingRequest bool) bool {
	preferred := detectZenPreferredChannelTypes(requestModel, isEmbeddingRequest)
	if len(preferred) == 0 {
		return true
	}
	_, ok := preferred[channelType]
	return ok
}

type perModelQuota struct {
	RPM int `json:"rpm"`
	TPM int `json:"tpm"`
}

func resolveAPIRateLimit(modelName string, c *gin.Context) (rpm int, tpm int) {
	rpm = c.GetInt("rate_limit_rpm")
	tpm = c.GetInt("rate_limit_tpm")

	perModelJSON := c.GetString("per_model_quota_json")
	if perModelJSON == "" {
		return
	}

	var quotas map[string]perModelQuota
	if err := jsonAPI.Unmarshal([]byte(perModelJSON), &quotas); err != nil {
		return
	}

	if q, ok := quotas[modelName]; ok {
		if q.RPM > 0 {
			rpm = q.RPM
		}
		if q.TPM > 0 {
			tpm = q.TPM
		}
	}
	return
}

func resolveCandidateModelName(requestModel string, item dbmodel.GroupItem) string {
	if upstreamModel, ok := resolveRequestedUpstreamModel(requestModel); ok {
		if strings.TrimSpace(item.ModelName) == "" || strings.EqualFold(strings.TrimSpace(item.ModelName), "zen") {
			return upstreamModel
		}
	}
	return item.ModelName
}

func apiKeyAllowsGroupCategory(allowedCategories string, groupCategory string) bool {
	allowedCategories = strings.TrimSpace(allowedCategories)
	if allowedCategories == "" {
		return true
	}
	category := strings.TrimSpace(groupCategory)
	if category == "" {
		return false
	}
	for _, allowed := range strings.Split(allowedCategories, ",") {
		if strings.EqualFold(strings.TrimSpace(allowed), category) {
			return true
		}
	}
	return false
}

// Handler 处理入站请求并转发到上游服务
func Handler(endpointType string, inboundType inbound.InboundType, c *gin.Context) {
	InflightInc()
	defer InflightDec()
	// 解析请求
	internalRequest, inAdapter, err := parseRequest(inboundType, c)
	if err != nil {
		return
	}
	supportedModels := c.GetString("supported_models")
	if supportedModels != "" {
		supportedModelsArray := strings.Split(supportedModels, ",")
		for i := range supportedModelsArray {
			supportedModelsArray[i] = strings.TrimSpace(supportedModelsArray[i])
		}
		if !slices.Contains(supportedModelsArray, internalRequest.Model) {
			resp.Error(c, http.StatusBadRequest, "model not supported")
			return
		}
	}

	requestModel := internalRequest.Model
	apiKeyID := c.GetInt("api_key_id")

	// Rate limiting: check RPM/TPM before forwarding
	if rpm := c.GetInt("rate_limit_rpm"); rpm > 0 || c.GetInt("rate_limit_tpm") > 0 {
		effectiveRPM, effectiveTPM := resolveAPIRateLimit(requestModel, c)
		if effectiveRPM > 0 || effectiveTPM > 0 {
			allowed, remaining, retryAfter := rl.CheckRateLimit(apiKeyID, requestModel, effectiveRPM, effectiveTPM, 0)
			if !allowed {
				c.Header("X-RateLimit-Remaining", "0")
				c.Header("Retry-After", strconv.Itoa(retryAfter))
				resp.Error(c, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			if effectiveRPM > 0 {
				c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
			}
		}
	}
	var streamSession *relayStreamSession
	var streamSessionOwned bool
	var lastErr error

	if conversationID, sessionHash, ok := resolveRelayStreamSessionIdentity(endpointType, int(inboundType), apiKeyID, internalRequest); ok {
		session, created, err := acquireRelayStreamSession(conversationID, apiKeyID, sessionHash)
		if err != nil {
			statusCode := http.StatusConflict
			if !errors.Is(err, errRelayConversationBusy) {
				statusCode = http.StatusInternalServerError
			}
			resp.Error(c, statusCode, err.Error())
			return
		}
		streamSession = session
		streamSessionOwned = created
		if !created {
			req := &relayRequest{
				c:               c,
				clientCtx:       c.Request.Context(),
				internalRequest: internalRequest,
				apiKeyID:        apiKeyID,
				requestModel:    requestModel,
				streamSession:   streamSession,
			}
			serveRelayStreamSession(c, req)
			return
		}
		defer func() {
			if streamSession == nil || !streamSessionOwned || streamSession.IsDone() {
				return
			}
			if lastErr == nil {
				lastErr = errors.New("relay stream ended without a terminal result")
			}
			streamSession.Finish(lastErr)
		}()
	}

	operationCtx, cancel := newRelayOperationContext()
	defer cancel()

	// 获取通道分组
	group, err := grp.GroupGetEnabledMapByEndpoint(endpointType, requestModel, operationCtx)
	if err != nil {
		lastErr = err
		log.Infof("model not found: model=%s endpoint_type=%s reason=%v", requestModel, endpointType, err)
		resp.Error(c, http.StatusNotFound, "model not found")
		return
	}
	if !apiKeyAllowsGroupCategory(c.GetString("allowed_group_categories"), group.Category) {
		lastErr = fmt.Errorf("group category not allowed for api key: category=%s", group.Category)
		resp.Error(c, http.StatusBadRequest, "model not supported")
		return
	}

	// 检查条件路由：条件不匹配则跳过
	if group.Condition != "" {
		condCtx := condition.RequestContext{
			Model:    requestModel,
			APIKeyID: apiKeyID,
			Hour:     time.Now().UTC().Hour(),
		}
		if match, condErr := condition.Evaluate(group.Condition, condCtx); condErr != nil || !match {
			lastErr = fmt.Errorf("condition not met for group %s", group.Name)
			resp.Error(c, http.StatusNotFound, "model not found")
			return
		}
	}

	// 创建迭代器（策略排序 + 粘性优先）
	iter := balancer.NewIterator(group, apiKeyID, requestModel, parseExcludedChannels(c.GetString("excluded_channels")))
	if iter.Len() == 0 {
		lastErr = errors.New("no available channel")
		resp.Error(c, http.StatusServiceUnavailable, "no available channel")
		return
	}

	// 根据分组端点提供方做请求兼容改写
	internalRequest = rewriteConversationRequestByProvider(group, internalRequest)

	// 初始化 Metrics
	clientIP := c.ClientIP()
	metrics := NewRelayMetrics(apiKeyID, requestModel, endpointType, group.EndpointType, clientIP, internalRequest)

	// 请求级上下文
	req := &relayRequest{
		c:                 c,
		clientCtx:         c.Request.Context(),
		operationCtx:      operationCtx,
		inAdapter:         inAdapter,
		internalRequest:   internalRequest,
		metrics:           metrics,
		apiKeyID:          apiKeyID,
		requestModel:      requestModel,
		groupEndpointType: group.EndpointType,
		group:             &group, // 传递分组对象用于策略读取
		iter:              iter,
		streamSession:     streamSession,
		retryCache:        newRetryRequestCache(),
	}

	var inflightKey string
	var inflightEnabled bool
	if endpointFamily := semanticCacheEndpointFamily(endpointType, inboundType); endpointFamily != "" {
		served, payload, cacheErr := maybeServeSemanticCacheHit(c, req, endpointFamily)
		if cacheErr != nil {
			log.Warnf("semantic cache lookup failed: %v", cacheErr)
		}
		if served {
			log.Infof("semantic cache hit: model=%s endpoint=%s", requestModel, endpointFamily)
			if normalizedPayload := semanticCacheHitPayload(payload, internalRequest); len(normalizedPayload) > 0 {
				if internalResponse, parseErr := buildSemanticCacheHitInternalResponse(internalRequest, normalizedPayload); parseErr == nil {
					metrics.SetInternalResponse(internalResponse, internalRequest.Model)
				}
			}
			metrics.Save(true, nil, nil)
			return
		}
		if _, text, ok, _ := getSemanticCacheLookupInput(req, endpointFamily); ok {
			inflightKey, inflightEnabled = requestSingleflightKey(apiKeyID, endpointFamily, internalRequest.Model, text, internalRequest)
		}
	}

	maxKeyRetriesPerRoute := getMaxAttemptsPerCandidate()
	maxRouteRetries := getMaxRouteRetries()
	ratelimitCooldown := getRatelimitCooldown()
	maxTotalAttempts := getMaxTotalAttempts()

	if inflightEnabled {
		result, sfErr, shared := relayInflightGroup.Do(inflightKey, func() (any, error) {
			return executeRelay(req, group, requestModel, maxKeyRetriesPerRoute, maxRouteRetries, ratelimitCooldown, maxTotalAttempts)
		})
		if sfErr == nil {
			if outcome, ok := result.(*inflightRelayResult); ok && outcome != nil {
				if shared {
					if outcome.namespace != "" && outcome.requestText != "" {
						cfg, ok := semanticCacheRuntimeConfig()
						if ok {
							embedding, _, embErr := lookupSemanticEmbeddingWithCache(req.operationCtx, req, cfg, outcome.namespace, outcome.requestText)
							if embErr == nil {
								if payload, found := semantic_cache.Lookup(outcome.namespace, embedding); found {
									normalizedPayload := semanticCacheHitPayload(payload, internalRequest)
									c.Data(http.StatusOK, "application/json", normalizedPayload)
									if internalResponse, parseErr := buildSemanticCacheHitInternalResponse(internalRequest, normalizedPayload); parseErr == nil {
										metrics.SetInternalResponse(internalResponse, outcome.actualModel)
									}
									metrics.Save(true, nil, nil)
									return
								}
							}
						}
					}
					if resp := cloneInternalResponse(outcome.internalResp); resp != nil {
						metrics.SetInternalResponse(resp, outcome.actualModel)
						// Cache miss: the leader already wrote its own response.
						// Transform the internal response to the inbound format and
						// write it to the shared caller's context so the client
						// receives a complete body instead of an empty 200 (4C-01).
						if inResponse, terr := req.inAdapter.TransformResponse(req.clientCtx, resp); terr == nil && len(inResponse) > 0 {
							c.Data(http.StatusOK, "application/json", inResponse)
						} else if terr != nil {
							logRelayErrorfByContext(terr, "shared caller transform response: %v", terr)
						}
					}
					metrics.Save(true, nil, outcome.attempts)
					return
				}
				return
			}
		}
	}

	if _, err := executeRelay(req, group, requestModel, maxKeyRetriesPerRoute, maxRouteRetries, ratelimitCooldown, maxTotalAttempts); err != nil {
		return
	}
	return
}

// attempt 统一管理一次通道尝试的完整生命周期
func (ra *relayAttempt) attempt() attemptResult {
	span := ra.iter.StartAttempt(ra.channel.ID, ra.usedKey.ID, ra.channel.Name, ra.internalRequest.Model)
	span.SetAdapterType(ra.adapterType.String())

	// 转发请求
	statusCode, fwdErr := ra.forward()

	// Client disconnected — do not record failure stats, circuit-breaker
	// counts, or retry hints. The client chose to stop, not the channel.
	if errors.Is(fwdErr, errClientDisconnected) {
		span.End(dbmodel.AttemptFailed, statusCode, "client disconnected")
		return attemptResult{
			Success:  false,
			Written:  ra.c.Writer.Written(),
			Err:      fwdErr,
			Decision: RetryDecision{Scope: ScopeAbortAll, Reason: "client disconnected", Code: statusCode},
		}
	}

	// 输出结果关键词拦截 — 不重试，不记录渠道失败统计
	if errors.Is(fwdErr, errResponseFilterBlocked) {
		span.End(dbmodel.AttemptFailed, statusCode, "response filter blocked")
		return attemptResult{
			Success:  false,
			Written:  ra.c.Writer.Written(),
			Err:      fwdErr,
			Decision: RetryDecision{Scope: ScopeAbortAll, Reason: "response filter blocked by keyword", Code: statusCode},
		}
	}

	// 空输出重试（issue #106）：上游返回 200 但输出为空，换 Key 重试。
	// 不记录渠道失败统计/熔断（这不是渠道故障，只是模型偶尔返回空），
	// 但占用一次 Key 级重试名额，用完后换下一渠道。
	if errors.Is(fwdErr, errEmptyOutput) {
		span.End(dbmodel.AttemptFailed, statusCode, "empty output, retrying")
		return attemptResult{
			Success:  false,
			Written:  false,
			Err:      fwdErr,
			Decision: RetryDecision{Scope: ScopeSameChannel, Reason: "empty output, try another key", Code: statusCode, IsError: true},
		}
	}

	// 检查是否已写入流式响应
	written := ra.c.Writer.Written()

	// 使用错误分类驱动决策
	decision := ClassifyRelayError(statusCode, fwdErr, written)

	// 记录按模型粒度的 key 冷却：某模型触发错误时，仅冷却该 (keyID, model) 组合，
	// 不影响该 key 上其它模型的可用性（见 issue #94）。仅错误响应（≥400）才冷却。
	if statusCode >= 400 {
		balancer.RecordKeyCooldown(ra.channel.ID, ra.usedKey.ID, ra.internalRequest.Model, statusCode)
	}
	// 记录可用度衰减：按错误类型加权（401/403 重扣、429/5xx 轻扣），仅 availability 策略生效。
	balancer.RecordKeyAvailability(ra.channel.ID, ra.usedKey.ID, ra.internalRequest.Model, statusCode, false)

	if decision.Scope == ScopeNone && !decision.IsError {
		// ====== 成功 ======
		ra.collectResponse()
		ra.collectAndStoreStreamResponse()
		ra.usedKey.TotalCost += ra.metrics.Stats.InputCost + ra.metrics.Stats.OutputCost
		ch.KeyUpdate(ra.usedKey)

		span.End(dbmodel.AttemptSuccess, statusCode, "")

		// Channel 维度统计
		updateChannelSuccessStats(ra.channel.ID, span.Duration().Milliseconds(), ra.metrics.Stats)
		st.ModelRecord(ra.channel.ID, ra.internalRequest.Model, dbmodel.StatsMetrics{
			WaitTime:       span.Duration().Milliseconds(),
			InputToken:     ra.metrics.Stats.InputToken,
			OutputToken:    ra.metrics.Stats.OutputToken,
			InputCost:      ra.metrics.Stats.InputCost,
			OutputCost:     ra.metrics.Stats.OutputCost,
			RequestSuccess: 1,
		})

		// 熔断器：记录成功
		balancer.RecordSuccess(ra.channel.ID, ra.usedKey.ID, ra.internalRequest.Model)
		// Auto策略：记录成功
		balancer.RecordAutoSuccess(ra.channel.ID, ra.internalRequest.Model)
		// Auto策略：记录延迟（毫秒）
		balancer.RecordAutoLatency(ra.channel.ID, ra.internalRequest.Model, span.Duration().Milliseconds())
		// 可用度：成功加分（上限 100），仅 availability 策略生效。
		balancer.RecordKeyAvailability(ra.channel.ID, ra.usedKey.ID, ra.internalRequest.Model, statusCode, true)
		// 速度策略：记录 EMA 平滑 TPS（output_tokens / duration_seconds），仅 speed 策略生效。
		balancer.RecordKeySpeed(ra.channel.ID, ra.usedKey.ID, ra.internalRequest.Model, ra.metrics.Stats.OutputToken, span.Duration().Milliseconds())
		// 会话保持：更新粘性记录
		balancer.SetSticky(ra.apiKeyID, ra.requestModel, ra.channel.ID, ra.usedKey.ID)

		return attemptResult{Success: true, Decision: decision}
	}

	// ====== 失败 ======
	ch.KeyUpdate(ra.usedKey)

	// 构造日志消息：决策摘要 + 上游原始错误（issue #93）。
	// fwdErr 形如 "upstream error: 429: {\"error\":...}"，已包含上游真实响应体，
	// 直接附在决策摘要后，便于在 relay log 中区分 429 的不同成因（资源耗尽 / RPM 限制等），
	// 而不是只看到笼统的 "rate limited, try another key"。
	msg := decision.String()
	if upstreamErr := extractUpstreamErrorDetail(fwdErr); upstreamErr != "" {
		msg = buildErrorMessage(msg, upstreamErr)
	}
	if ra.tryTotal > 1 {
		msg = buildAttemptMessage(ra.tryIndex, ra.tryTotal, msg)
	}
	span.End(dbmodel.AttemptFailed, statusCode, msg)

	// Channel 维度统计
	st.ChannelUpdate(ra.channel.ID, dbmodel.StatsMetrics{
		WaitTime:      span.Duration().Milliseconds(),
		RequestFailed: 1,
	})
	st.ModelRecord(ra.channel.ID, ra.internalRequest.Model, dbmodel.StatsMetrics{
		WaitTime:      span.Duration().Milliseconds(),
		RequestFailed: 1,
	})

	// 熔断器和 Auto 策略的记录由调用方（adapter fallback loop）控制，
	// 避免在 adapter 降级场景（如 Responses→Chat）中误触发熔断。

	if written {
		ra.collectResponse()
	}

	// 记录决策日志
	if decision.IsError {
		logRelayErrorfByContext(fwdErr, "[%s] channel %s adapter=%s attempt %d/%d failed: %s (decision: %s)",
			ra.internalRequest.RawAPIFormat, ra.channel.Name, ra.adapterType, ra.tryIndex, ra.tryTotal, fwdErr, decision.Scope.String())
	}

	return attemptResult{
		Success:  false,
		Written:  written,
		Err:      fmt.Errorf("channel %s adapter=%s attempt %d/%d: %v", ra.channel.Name, ra.adapterType, ra.tryIndex, ra.tryTotal, fwdErr),
		Decision: decision,
	}
}

// parseRequest 解析并验证入站请求
func parseRequest(inboundType inbound.InboundType, c *gin.Context) (*model.InternalLLMRequest, model.Inbound, error) {
	body, err := readLimitedRequestBody(c, getMaxRelayJSONBodyBytes())
	if err != nil {
		resp.Error(c, relayRequestBodyErrorStatus(err), err.Error())
		return nil, nil, err
	}

	inAdapter := inbound.Get(inboundType)
	if inAdapter == nil {
		resp.Error(c, http.StatusBadRequest, fmt.Sprintf("unsupported inbound type: %d", inboundType))
		return nil, nil, fmt.Errorf("unsupported inbound type: %d", inboundType)
	}
	internalRequest, err := inAdapter.TransformRequest(c.Request.Context(), body)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return nil, nil, err
	}

	// Pass through the original query parameters and request path
	internalRequest.Query = c.Request.URL.Query()
	internalRequest.RawPath = c.Request.URL.Path

	if err := internalRequest.Validate(); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return nil, nil, err
	}

	populateRelayRequestSessionFields(c, internalRequest, body)

	return internalRequest, inAdapter, nil
}

// forward 转发请求到上游服务
func (ra *relayAttempt) forward() (int, error) {
	ctx := ra.operationCtx

	// 单次转发尝试超时（issue #122）：当分组配置了 attempt_time_out 时，
	// 为本次尝试创建带超时的派生上下文。超时后 context deadline exceeded
	// 被 isTimeoutError() 匹配，产生 ScopeNextChannel 决策。
	if ra.attemptTimeOutSec > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(ra.attemptTimeOutSec)*time.Second)
		defer cancel()
	}

	requestForOutbound := ra.internalRequest
	effectiveRewrite := (*rewrite.EffectiveConfig)(nil)
	// passthrough / raw（原始穿透）都要求原样转发客户端请求体，
	// 跳过 param_override 与改写引擎，避免请求体被二次加工。
	if ra.adapterType != outbound.OutboundTypePassthrough && ra.adapterType != outbound.OutboundTypeRaw {
		var err error
		requestForOutbound, effectiveRewrite, err = prepareInternalRequestForOutbound(ra.channel, ra.internalRequest, ra.groupEndpointType)
		if err != nil {
			log.Warnf("failed to prepare outbound request data: %v", err)
			return 0, fmt.Errorf("failed to prepare outbound request data: %w", err)
		}
	}

	// 构建出站请求
	baseURL := ra.channel.GetNormalizedBaseUrl()
	if ra.poolBaseURL != "" {
		baseURL = ra.poolBaseURL
	}
	outboundRequest, err := ra.outAdapter.TransformRequest(
		ctx,
		requestForOutbound,
		baseURL,
		ra.usedKey.ChannelKey,
	)
	if err != nil {
		log.Warnf("failed to create request: %v", err)
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	// 号池凭据按 platform/type 路由出站鉴权头。
	ra.applyPoolCredentialHeaders(outboundRequest)

	// 复制请求头
	ra.copyHeaders(outboundRequest, effectiveRewrite)

	// 发送请求
	response, err := ra.sendRequest(outboundRequest)
	if err != nil {
		return 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer response.Body.Close()

	// 检查响应状态
	statusCode, err := ra.handleForwardResponse(response)
	if err != nil {
		return statusCode, err
	}

	// 处理响应
	if ra.internalRequest.Stream != nil && *ra.internalRequest.Stream {
		if err := ra.handleStreamResponse(ctx, response); err != nil {
			return response.StatusCode, err
		}
		return response.StatusCode, nil
	}
	if err := ra.handleResponse(ctx, response); err != nil {
		return response.StatusCode, err
	}
	return response.StatusCode, nil
}

// applyPoolCredentialHeaders 按号池账号 platform/type 调整出站鉴权头。
//
// 出站适配器已按 ChannelKey 设置默认 Authorization（Bearer 或 X-API-Key）。
// 这里仅处理需要覆盖的情况：
//   - cookie 类型：删除 Authorization，改为 Cookie header（ChannelKey 即 cookie 值）。
//   - openai-oauth 且非 codex 适配器：ChannelKey 是 OAuth JSON，适配器无法解析，
//     手动设置 Authorization: Bearer {access_token} + chatgpt-account-id: {account_id}。
//     codex 适配器自身解析 OAuth JSON，无需覆盖。
//   - gemini-oauth：ChannelKey 是 code_assist 凭据 JSON，由 gemini 出站适配器
//     自行解析并设置 Bearer（同时切到 cloudcode-pa 端点），这里不覆盖。
//   - 其他 oauth/apikey/upstream：适配器默认 Bearer 行为正确，无需覆盖。
//   - P3 header overrides：符合资格条件时叠加自定义请求头（跳过黑名单与安全头）。
func (ra *relayAttempt) applyPoolCredentialHeaders(req *http.Request) {
	if ra.poolType == "" {
		return
	}
	switch ra.poolType {
	case dbmodel.PoolTypeCookie:
		req.Header.Del("Authorization")
		req.Header.Set("Cookie", ra.usedKey.ChannelKey)
	case dbmodel.PoolTypeOAuth:
		if ra.poolPlatform == dbmodel.PoolPlatformOpenAI && ra.adapterType != outbound.OutboundTypeCodex {
			// ChannelKey 对 openai-oauth 是 OAuth JSON（由 EffectiveKeyWithExtra 构造）。
			var oauth struct {
				AccessToken string `json:"access_token"`
				AccountID   string `json:"account_id"`
			}
			if json.Unmarshal([]byte(ra.usedKey.ChannelKey), &oauth) == nil && oauth.AccessToken != "" {
				req.Header.Set("Authorization", "Bearer "+oauth.AccessToken)
				if oauth.AccountID != "" {
					req.Header.Set("chatgpt-account-id", oauth.AccountID)
				}
			}
		}
	}
	// P3 自定义请求头叠加（不依赖 poolType，仅检查是否为号池账号）。
	ra.applyHeaderOverrides(req)
}

// blockedHeaderOverrideNames 与 sub2api 对齐的黑名单：防止误覆写鉴权/路由关键头。
var blockedHeaderOverrideNames = map[string]struct{}{
	"authorization":            {},
	"x-api-key":                {},
	"cookie":                   {},
	"host":                     {},
	"content-length":           {},
	"chatgpt-account-id":       {},
	"x-claude-code-session-id": {},
	"x-client-request-id":      {},
	"x-grok-conv-id":           {},
}

// applyHeaderOverrides 按账号 extra.header_overrides 应用自定义请求头。
// 资格条件同 sub2api IsHeaderOverrideEligible：
//   - anthropic / openai 且 type==apikey
//   - grok 且 (type==apikey 或 oauth)
//
// 其他平台/类型不叫用（防御性），避免破坏 cookie/setup-token 类型的默认鉴权。
// 黑名单头与空 value 会被跳过。
func (ra *relayAttempt) applyHeaderOverrides(req *http.Request) {
	if ra.poolAccount == nil {
		return
	}
	acct := ra.poolAccount
	eligible := false
	switch acct.Platform {
	case dbmodel.PoolPlatformAnthropic, dbmodel.PoolPlatformOpenAI:
		eligible = acct.Type == dbmodel.PoolTypeAPIKey
	case dbmodel.PoolPlatformGrok:
		eligible = acct.Type == dbmodel.PoolTypeAPIKey || acct.Type == dbmodel.PoolTypeOAuth
	}
	if !eligible {
		return
	}
	extra := acct.GetExtra()
	if !extra.HeaderOverridesEnabled || len(extra.HeaderOverrides) == 0 {
		return
	}
	for k, v := range extra.HeaderOverrides {
		if v == "" {
			continue
		}
		lower := strings.ToLower(k)
		if _, blocked := blockedHeaderOverrideNames[lower]; blocked {
			continue
		}
		// 前缀匹配 x-codex-* 名列黑名单
		if strings.HasPrefix(lower, "x-codex-") {
			continue
		}
		req.Header.Set(k, v)
	}
}

func (ra *relayAttempt) handleForwardResponse(response *http.Response) (int, error) {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return response.StatusCode, nil
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxErrorBodyBytes+1))
	if err != nil {
		return response.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}
	if len(body) > maxErrorBodyBytes {
		return response.StatusCode, fmt.Errorf("upstream error: %d: response body too large", response.StatusCode)
	}
	return response.StatusCode, fmt.Errorf("upstream error: %d: %s", response.StatusCode, string(body))
}

// upstreamErrorDetailPrefix 是 handleForwardResponse 生成的上游错误前缀。
const upstreamErrorDetailPrefix = "upstream error: "

// extractUpstreamErrorDetail 从 forward()/handleForwardResponse 产出的错误信息中
// 抽取上游真实错误（含状态码与响应体），用于 relay log 的 msg 字段，使 429 等错误能
// 原样展示上游响应，而不是只看到笼统的决策摘要（issue #93）。
//
// 错误形如 "upstream error: 429: {\"error\":...}"，这里返回 "429: {\"error\":...}"；
// 若外层再包了一层 channel/attempt 前缀，也会定位到 "upstream error: " 后截取。
// 非 HTTP 错误（如网络错误、transformer 错误）则直接返回错误文本本身。
func extractUpstreamErrorDetail(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if idx := strings.Index(s, upstreamErrorDetailPrefix); idx >= 0 {
		return s[idx+len(upstreamErrorDetailPrefix):]
	}
	return s
}

// clientErrorType 为非 JSON 上游错误体合成 OpenAI 兼容 envelope 时选择 type。
func clientErrorType(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "invalid_request_error"
	case http.StatusUnauthorized, http.StatusForbidden:
		return "authentication_error"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	default:
		return "api_error"
	}
}

// writeClientTerminalError 在 ScopeNone 等「不再重试」的终态下，把上游错误尽量原样
// 回给下游客户端，而不是吞成管理端风格的 502 "upstream service unavailable"。
//
// 这样下游（如 omp/oh-my-pi）才能识别 context_length_exceeded / prompt is too long
// 等溢出信号并自动触发上下文压缩，而不是被网关伪装错误挡住。
//
// 策略：
//  1. statusCode>=400 且错误中带有上游 JSON body → 原样 HTTP 状态 + 原样 body；
//  2. 有上游纯文本 body → 包成 OpenAI 兼容 {"error":{...}}，状态码仍用上游；
//  3. 没有可用 body → 合成最小 OpenAI 兼容错误；
//  4. 非 HTTP 错误 / 无效状态 → 回退 BadGateway。
func writeClientTerminalError(c *gin.Context, statusCode int, err error) {
	if c == nil || c.Writer.Written() {
		return
	}
	if statusCode < 400 {
		resp.BadGateway(c)
		return
	}

	detail := extractUpstreamErrorDetail(err)
	bodyText := strings.TrimSpace(detail)
	if prefix := fmt.Sprintf("%d: ", statusCode); strings.HasPrefix(bodyText, prefix) {
		bodyText = strings.TrimSpace(bodyText[len(prefix):])
	}

	if bodyText != "" && bodyText != "response body too large" {
		body := []byte(bodyText)
		if jsonAPI.Valid(body) {
			c.Data(statusCode, "application/json", body)
			c.Abort()
			return
		}
		if payload, mErr := jsonAPI.Marshal(map[string]any{
			"error": map[string]any{
				"message": bodyText,
				"type":    clientErrorType(statusCode),
				"code":    "",
				"param":   "",
			},
		}); mErr == nil {
			c.Data(statusCode, "application/json", payload)
			c.Abort()
			return
		}
	}

	message := http.StatusText(statusCode)
	if message == "" {
		message = "request failed"
	}
	if payload, mErr := jsonAPI.Marshal(map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    clientErrorType(statusCode),
			"code":    "",
			"param":   "",
		},
	}); mErr == nil {
		c.Data(statusCode, "application/json", payload)
		c.Abort()
		return
	}
	resp.Error(c, statusCode, message)
}

// copyHeaders 复制请求头，过滤 hop-by-hop 头
func (ra *relayAttempt) copyHeaders(outboundRequest *http.Request, effectiveRewrite *rewrite.EffectiveConfig) {
	for key, values := range ra.c.Request.Header {
		if hopByHopHeaders[strings.ToLower(key)] {
			continue
		}
		for _, value := range values {
			outboundRequest.Header.Set(key, value)
		}
	}
	if len(ra.channel.CustomHeader) > 0 {
		for _, header := range ra.channel.CustomHeader {
			outboundRequest.Header.Set(header.HeaderKey, header.HeaderValue)
		}
	}

	if effectiveRewrite != nil && len(effectiveRewrite.ExtraHeaders) > 0 {
		for key, value := range effectiveRewrite.ExtraHeaders {
			outboundRequest.Header.Set(key, value)
		}
	}
}

// sendRequest 发送 HTTP 请求
func (ra *relayAttempt) sendRequest(req *http.Request) (*http.Response, error) {
	var httpClient *http.Client
	var err error
	// 号池账号级代理优先；未配置时回退到渠道级代理。
	if ra.poolProxyConfigID != nil {
		httpClient, err = helper.PoolAccountHttpClient(ra.poolProxyConfigID)
		if err != nil {
			log.Warnf("failed to get pool account http client: %v", err)
			return nil, err
		}
	}
	if httpClient == nil {
		httpClient, err = helper.ChannelHttpClient(ra.channel)
		if err != nil {
			log.Warnf("failed to get http client: %v", err)
			return nil, err
		}
	}

	response, err := httpClient.Do(req)
	if err != nil {
		logRelayErrorfByContext(err, "failed to send request: %v", err)
		return nil, err
	}

	return response, nil
}

// handleStreamResponse 处理流式响应
func (ra *relayAttempt) handleStreamResponse(ctx context.Context, response *http.Response) (retErr error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 安全网：确保 stream session 在所有退出路径上都被关闭，
	// 避免外层 defer 产生 "relay stream ended without a terminal result"。
	defer func() {
		if ra.streamSession != nil && !ra.streamSession.IsDone() {
			ra.streamSession.Finish(retErr)
		}
	}()

	if ct := response.Header.Get("Content-Type"); ct != "" && !strings.Contains(strings.ToLower(ct), "text/event-stream") {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 16*1024))
		return fmt.Errorf("upstream returned non-SSE content-type %q for stream request: %s", ct, string(body))
	}

	// 设置 SSE 响应头
	ra.c.Header("Content-Type", "text/event-stream")
	ra.c.Header("Cache-Control", "no-cache")
	ra.c.Header("Connection", "keep-alive")
	ra.c.Header("X-Accel-Buffering", "no")
	if ra.internalRequest.ConversationID != "" {
		ra.c.Header("X-Conversation-ID", ra.internalRequest.ConversationID)
	}

	firstToken := true
	hasVisibleContent := false // 是否已产生可见内容（issue #155 流式空输出检测）
	strategy := getReasoningBufferStrategy(ra.group)
	shouldBuffer := (strategy == "buffer") // buffer=暂存; immediate=立即发送
	var reasoningBuffer [][]byte           // 暂存仅含 reasoning 的 chunk，待可见内容到达后 flush
	var reasoningBufferBytes int           // reasoningBuffer 累计字节数，用于软上限判定（见 maxReasoningBufferBytes）
	clientDone := ra.clientCtx.Done()
	clientDisconnected := false
	clientDisconnectLogged := false
	markClientDisconnected := func() {
		if clientDisconnected {
			return
		}
		clientDisconnected = true
		clientDone = nil
		// 启动断连宽限计时。带 stream session 时循环会继续读上游（以支持断线
		// 重连重放），这段时间里会话既不是 done、也无法被驱逐，其缓冲会一直占
		// 着内存。宽限期到后由下面的 clientGoneTicker 强制收尾（issue #196）。
		ra.streamSession.MarkClientGone()
	}
	logClientDisconnected := func() {
		if !clientDisconnected || clientDisconnectLogged {
			return
		}
		clientDisconnectLogged = true
		log.Warnf(clientDisconnectedLogMessage)
	}

	type sseReadResult struct {
		data string
		err  error
	}
	results := make(chan sseReadResult, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("SSE reader panic recovered: %v", r)
			}
		}()
		defer close(results)
		readCfg := &sse.ReadConfig{MaxEventSize: maxSSEEventSize}
		for ev, err := range sse.Read(response.Body, readCfg) {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err != nil {
				select {
				case results <- sseReadResult{err: err}:
				case <-ctx.Done():
				}
				return
			}
			select {
			case results <- sseReadResult{data: ev.Data}:
			case <-ctx.Done():
				return
			}
		}
	}()

	var firstTokenTimer *time.Timer
	var firstTokenC <-chan time.Time
	if firstToken && ra.firstTokenTimeOutSec > 0 {
		firstTokenTimer = time.NewTimer(time.Duration(ra.firstTokenTimeOutSec) * time.Second)
		firstTokenC = firstTokenTimer.C
		defer func() {
			if firstTokenTimer != nil {
				firstTokenTimer.Stop()
			}
		}()
	}

	// SSE 心跳：当上游在可见内容后出现较长间隔（如 reasoning 阶段）时，定期写入
	// SSE comment（": ping\n\n"）防止反向代理因 proxy_read_timeout 判定后端无响应而
	// 切断连接返回 502。仅在 hasVisibleContent 之后发送——首 token 前的心跳会让
	// c.Writer.Written() 变 true，破坏 buffer 策略下 reasoning 阶段的安全重试语义
	//（见 issue #155），且首 token 超时已由 firstTokenTimer 兜底。
	heartbeatTicker := time.NewTicker(conf.SSEHeartbeatInterval)
	defer heartbeatTicker.Stop()

	// 客户端断连后的宽限期检查。仅在存在 stream session 时有意义：没有 session
	// 的请求在客户端断连时会直接返回。周期远小于宽限期，保证及时收尾。
	var clientGoneTicker *time.Ticker
	var clientGoneC <-chan time.Time
	if ra.streamSession != nil {
		clientGoneTicker = time.NewTicker(relayStreamClientGoneCheckInterval)
		clientGoneC = clientGoneTicker.C
		defer clientGoneTicker.Stop()
	}

	for {
		select {
		case <-clientDone:
			if ra.streamSession == nil {
				log.Infof("client disconnected, stopping stream")
				return errClientDisconnected
			}
			markClientDisconnected()
		case <-clientGoneC:
			// 客户端已断连且上游在宽限期内仍未结束：强制结束会话并停止读取上游，
			// 避免断连会话的 replay 缓冲长期占用内存（issue #196 的 OOM 主因）。
			if !clientDisconnected {
				continue
			}
			// 重连的读取者接入时 Subscribe 会清掉宽限计时；它再次离开后这里负责
			// 重新起表。原请求的 clientCtx 早已 Done，无法再次触发 clientDone 分支，
			// 因此以「当前是否有订阅者」作为客户端是否在读的判据。
			if ra.streamSession.HasSubscribers() {
				continue
			}
			ra.streamSession.MarkClientGone()
			if !ra.streamSession.ClientGoneGraceExceeded() {
				continue
			}
			logClientDisconnected()
			log.Warnf("client gone and upstream still streaming after %s, closing stream session to release buffer",
				relayStreamClientGoneGrace)
			if err := response.Body.Close(); err != nil {
				log.Warnf("failed to close response body on client-gone grace timeout: %v", err)
			}
			ra.streamSession.Finish(errRelayStreamClientGone)
			return errClientDisconnected
		case <-firstTokenC:
			logClientDisconnected()
			log.Warnf("first token timeout (%ds), switching channel", ra.firstTokenTimeOutSec)
			if err := response.Body.Close(); err != nil {
				log.Warnf("failed to close response body on first token timeout: %v", err)
			}
			return fmt.Errorf("first token timeout (%ds)", ra.firstTokenTimeOutSec)
		case <-heartbeatTicker.C:
			if hasVisibleContent {
				if _, err := ra.c.Writer.Write([]byte(": ping\n\n")); err != nil {
					markClientDisconnected()
					logClientDisconnected()
					continue
				}
				ra.c.Writer.Flush()
			}
		case r, ok := <-results:
			if !ok {
				// results channel 被 SSE reader goroutine 关闭。
				// 需要区分正常结束（上游 EOF）和异常中断（ctx 取消/超时）。
				if ctxErr := ctx.Err(); ctxErr != nil {
					if ra.streamSession != nil {
						ra.streamSession.Finish(ctxErr)
					}
					return fmt.Errorf("stream interrupted: %w", ctxErr)
				}
				logClientDisconnected()
				if ra.streamSession != nil {
					ra.streamSession.Finish(nil)
					log.Infof("stream end")
					// 流结束显式释放 reasoningBuffer（虽随函数返回被 GC，但提前释放降低峰值持续时间）。
					reasoningBuffer = nil
					reasoningBufferBytes = 0
					// 空输出检测（issue #106/#155）：整个流式响应没有产生任何可见内容。
					// buffer 策略：reasoning-only chunk 被暂存到 reasoningBuffer，未写入客户端（Written()=false），
					// 可以安全重试。immediate 策略：reasoning 已发送，不可重试（只记录日志）。
					// 仅当启用空输出重试且使用 buffer 策略时触发重试。
					if isRetryEmptyOutputEnabled() && shouldBuffer && !hasVisibleContent {
						log.Infof("channel %s returned empty stream (no visible content), will retry", ra.channel.Name)
						if ra.streamSession != nil {
							ra.streamSession.Finish(nil)
						}
						return errEmptyOutput
					}
					if !shouldBuffer && !hasVisibleContent {
						log.Warnf("channel %s returned empty stream (immediate strategy, no retry)", ra.channel.Name)
					}
					return nil
				}
				return nil
			}
			if r.err != nil {
				logClientDisconnected()
				logRelayErrorfByContext(r.err, "failed to read event: %v", r.err)
				return fmt.Errorf("failed to read stream event: %w", r.err)
			}

			data, chunkHasVisible, err := ra.transformStreamData(ctx, r.data)
			if err != nil {
				if errors.Is(err, errResponseFilterBlocked) {
					// 关键词拦截：发送错误 SSE 事件并终止流
					filterCfg := ra.getResponseFilterConfig()
					// 先 flush 暂存的 reasoning buffer，再发送错误事件
					if len(reasoningBuffer) > 0 {
						writeReasoningBuffer(ra, reasoningBuffer, &clientDisconnected, markClientDisconnected, logClientDisconnected)
						reasoningBuffer = nil
						reasoningBufferBytes = 0
					}
					if ra.streamSession != nil {
						errPayload, _ := jsonAPI.Marshal(map[string]any{
							"error": map[string]any{
								"message": filterCfg.ErrorMessage,
								"type":    "content_filter",
								"code":    "content_blocked",
							},
						})
						ra.streamSession.AddPayload(errPayload)
						ra.streamSession.Finish(nil)
					} else if !clientDisconnected {
						writeSSEErrorEvent(ra.c.Writer, filterCfg.ErrorMessage)
						ra.c.Writer.Flush()
					}
					if closeErr := response.Body.Close(); closeErr != nil {
						log.Warnf("failed to close response body on response filter block: %v", closeErr)
					}
					return fmt.Errorf("response filter blocked streaming output")
				}
				continue
			}
			if len(data) == 0 {
				continue
			}
			if firstToken {
				ra.metrics.SetFirstTokenTime(time.Now())
				firstToken = false
				if firstTokenTimer != nil {
					if !firstTokenTimer.Stop() {
						select {
						case <-firstTokenTimer.C:
						default:
						}
					}
					firstTokenTimer = nil
					firstTokenC = nil
				}
			}

			// issue #155：根据策略决定是否缓冲 reasoning chunks。
			// buffer 策略：暂存到 buffer，待可见内容到达后统一 flush（安全重试但 CF 可能超时）
			// immediate 策略：立即发送所有 chunks（实时体验但空输出不可重试）
			if shouldBuffer && !chunkHasVisible && !hasVisibleContent {
				// 软上限：累计字节超过 maxReasoningBufferBytes 时丢弃当前 buffer，避免
				// 长 reasoning-only 流内存无界增长。丢弃而非 flush 以保持空输出重试安全性
				// （hasVisibleContent 保持 false，仍可重试）。8 MiB reasoning-only 已属异常。
				if reasoningBufferBytes >= maxReasoningBufferBytes {
					reasoningBuffer = nil
					reasoningBufferBytes = 0
				}
				reasoningBuffer = append(reasoningBuffer, data)
				reasoningBufferBytes += len(data)
				continue
			}

			// 可见内容到达，先 flush 暂存的 reasoning buffer
			if len(reasoningBuffer) > 0 {
				writeReasoningBuffer(ra, reasoningBuffer, &clientDisconnected, markClientDisconnected, logClientDisconnected)
				reasoningBuffer = nil
				reasoningBufferBytes = 0
			}
			hasVisibleContent = true

			if ra.streamSession != nil {
				sessionEvents := ra.streamSession.AddPayload(data)
				if clientDisconnected {
					logClientDisconnected()
					continue
				}
				for _, event := range sessionEvents {
					if _, err := ra.c.Writer.Write(formatRelaySSEEvent(event.Sequence, event.Payload)); err != nil {
						markClientDisconnected()
						logClientDisconnected()
						break
					}
					ra.c.Writer.Flush()
				}
				continue
			}

			if clientDisconnected {
				logClientDisconnected()
				continue
			}
			if _, err := ra.c.Writer.Write(data); err != nil {
				markClientDisconnected()
				logClientDisconnected()
				continue
			}
			ra.c.Writer.Flush()
		}
	}
}

// writeReasoningBuffer flushes buffered reasoning-only chunks to the client.
// Used when visible content finally arrives (or on response-filter block) to
// release the reasoning that was buffered for retry-safety (issue #155).
func writeReasoningBuffer(ra *relayAttempt, buffer [][]byte, clientDisconnected *bool,
	markClientDisconnected func(), logClientDisconnected func()) {
	for _, data := range buffer {
		if *clientDisconnected {
			logClientDisconnected()
			return
		}
		if ra.streamSession != nil {
			sessionEvents := ra.streamSession.AddPayload(data)
			for _, event := range sessionEvents {
				if _, err := ra.c.Writer.Write(formatRelaySSEEvent(event.Sequence, event.Payload)); err != nil {
					markClientDisconnected()
					logClientDisconnected()
					return
				}
				ra.c.Writer.Flush()
			}
			continue
		}
		if _, err := ra.c.Writer.Write(data); err != nil {
			markClientDisconnected()
			logClientDisconnected()
			return
		}
		ra.c.Writer.Flush()
	}
}

// transformStreamData 转换流式数据，返回转换后的 SSE 字节、该 chunk 是否包含可见内容、以及错误。
// hasVisibleContent 用于流式空输出检测：仅含 reasoning 的 chunk 不算可见内容（issue #155）。
func (ra *relayAttempt) transformStreamData(ctx context.Context, data string) ([]byte, bool, error) {
	internalStream, err := ra.outAdapter.TransformStream(ctx, []byte(data))
	if err != nil {
		logRelayErrorfByContext(err, "failed to transform stream: %v", err)
		return nil, false, err
	}
	if internalStream == nil {
		return nil, false, nil
	}

	hasVisible := streamChunkHasVisibleContent(internalStream)

	// 输出结果关键词拦截（流式）
	filterCfg := ra.getResponseFilterConfig()
	if blocked, keyword := applyResponseFilter(internalStream, filterCfg); blocked {
		log.Infof("response filter blocked streaming chunk with keyword %q", keyword)
		return nil, false, errResponseFilterBlocked
	}

	inStream, err := ra.inAdapter.TransformStream(ctx, internalStream)
	if err != nil {
		logRelayErrorfByContext(err, "failed to transform stream: %v", err)
		return nil, false, err
	}

	return inStream, hasVisible, nil
}

// handleResponse 处理非流式响应
func (ra *relayAttempt) handleResponse(ctx context.Context, response *http.Response) error {
	internalResponse, err := ra.outAdapter.TransformResponse(ctx, response)
	if err != nil {
		logRelayErrorfByContext(err, "failed to transform response: %v", err)
		return fmt.Errorf("failed to transform outbound response: %w", err)
	}

	// 输出结果关键词拦截
	filterCfg := ra.getResponseFilterConfig()
	if blocked, keyword := applyResponseFilter(internalResponse, filterCfg); blocked {
		log.Infof("response filter blocked keyword %q", keyword)
		errMsg := filterCfg.ErrorMessage
		errorResp := map[string]any{
			"error": map[string]any{
				"message": errMsg,
				"type":    "content_filter",
				"code":    "content_blocked",
			},
		}
		data, _ := jsonAPI.Marshal(errorResp)
		ra.c.Data(http.StatusOK, "application/json", data)
		return nil
	}

	applyReasoningExhaustedHeader(ra.c, internalResponse)

	// 空输出检测（issue #106/#155）：上游返回 200 但无可见内容。
	// 不依赖 CompletionTokens 判断——推理模型可能 CompletionTokens > 0 但无可见内容。
	if isRetryEmptyOutputEnabled() && isEmptyOutputResponse(internalResponse) {
		log.Infof("channel %s returned empty output (no visible content), will retry", ra.channel.Name)
		return errEmptyOutput
	}

	inResponse, err := ra.inAdapter.TransformResponse(ctx, internalResponse)
	if err != nil {
		logRelayErrorfByContext(err, "failed to transform response: %v", err)
		return fmt.Errorf("failed to transform inbound response: %w", err)
	}

	storeSemanticCacheResponse(ctx, ra.internalRequest, inResponse)

	ra.c.Data(http.StatusOK, "application/json", inResponse)
	return nil
}

func applyReasoningExhaustedHeader(c *gin.Context, resp *model.InternalLLMResponse) {
	if c == nil || !isReasoningExhaustedResponse(resp) {
		return
	}
	c.Header("X-Reasoning-Exhausted", "true")
}

func isReasoningExhaustedResponse(resp *model.InternalLLMResponse) bool {
	if resp == nil || resp.Usage == nil || len(resp.Choices) == 0 {
		return false
	}
	if resp.Usage.CompletionTokensDetails == nil || resp.Usage.CompletionTokensDetails.ReasoningTokens <= 0 {
		return false
	}
	for _, choice := range resp.Choices {
		if choice.Message == nil {
			continue
		}
		if choice.Message.Content.Content != nil && strings.TrimSpace(*choice.Message.Content.Content) != "" {
			return false
		}
		if len(choice.Message.Content.MultipleContent) > 0 || len(choice.Message.ToolCalls) > 0 {
			return false
		}
	}
	return true
}

// collectResponse 收集响应信息
func (ra *relayAttempt) collectResponse() {
	internalResponse, err := ra.inAdapter.GetInternalResponse(ra.operationCtx)
	if err != nil || internalResponse == nil {
		return
	}

	ra.metrics.SetInternalResponse(internalResponse, ra.internalRequest.Model)
}

// collectAndStoreStreamResponse stores the already-aggregated stream response
// in the semantic cache (success path only). It reuses the InternalResponse
// previously collected by collectResponse() to avoid a second call to
// GetInternalResponse(), which would return nil after stream chunks are consumed.
func (ra *relayAttempt) collectAndStoreStreamResponse() {
	if ra.internalRequest.Stream == nil || !*ra.internalRequest.Stream {
		return
	}
	internalResponse := ra.metrics.InternalResponse
	if internalResponse == nil {
		return
	}
	if responseJSON, err := jsonAPI.Marshal(internalResponse); err == nil {
		storeSemanticCacheResponse(ra.operationCtx, ra.internalRequest, responseJSON)
	}
}

func rewriteConversationRequestByProvider(group dbmodel.Group, req *model.InternalLLMRequest) *model.InternalLLMRequest {
	if req == nil {
		return req
	}
	endpointType := dbmodel.NormalizeEndpointType(group.EndpointType)
	provider := strings.ToLower(strings.TrimSpace(group.EndpointProvider))
	if provider == "" || provider == "auto" {
		return req
	}
	if endpointType == dbmodel.EndpointTypeAll {
		return req
	}
	if endpointType != dbmodel.EndpointTypeChat {
		return req
	}

	// Provider rewrite config: which non-standard message fields to strip.
	// Some providers (e.g. standard OpenAI) reject reasoning_content / reasoning_signature fields.
	type providerRewrite struct {
		stripReasoning          bool
		stripReasoningSignature bool
	}
	providers := map[string]providerRewrite{
		"openai":   {stripReasoning: true, stripReasoningSignature: true},
		"deepseek": {stripReasoning: true, stripReasoningSignature: false},
		"mimo":     {stripReasoning: false, stripReasoningSignature: true},
	}
	cfg, ok := providers[provider]
	if !ok {
		return req
	}

	cloned := *req
	if len(req.Messages) > 0 {
		cloned.Messages = make([]model.Message, len(req.Messages))
		for i, msg := range req.Messages {
			cloned.Messages[i] = msg
			if cfg.stripReasoning {
				cloned.Messages[i].Reasoning = nil
			}
			if cfg.stripReasoningSignature {
				cloned.Messages[i].ReasoningSignature = nil
			}
		}
	}
	return &cloned
}

// isClientDisconnected reports whether the client has disconnected.
func isClientDisconnected(clientCtx context.Context) bool {
	select {
	case <-clientCtx.Done():
		return true
	default:
		return false
	}
}

// handleClientDisconnect is a shared handler for client-disconnect checks
// inside the executeRelay retry loops. It saves metrics and returns the error.
func handleClientDisconnect(req *relayRequest, allAttempts []dbmodel.ChannelAttempt) error {
	log.Infof("client disconnected, stopping relay retry loop")
	req.metrics.Save(false, errClientDisconnected, allAttempts)
	return errClientDisconnected
}

func executeRelay(req *relayRequest, group dbmodel.Group, requestModel string, maxKeyRetriesPerRoute int, maxRouteRetries int, ratelimitCooldown int, maxTotalAttempts int) (*inflightRelayResult, error) {
	var allAttempts []dbmodel.ChannelAttempt
	var lastErr error
	rateLimitHoldCfg := getRateLimitHoldConfig()

	// roundDecisions 返回到当前为止累计的决策纪录条数（真实转发 + 冷却跳过 + 熔断跳过）。
	// 与之前只统计 ForwardedAttempts 不同：当所有渠道都处于冷却/熔断时，真实转发恒为 0，
	// 只统计转发会让最大总尝试上限形同虚设，attempts 仍会无上限膨胀（见 issue #192）。
	// 因此改用「决策纪录总数」作为上限，同时约束转发工作量与 relay_logs 体积。
	roundDecisions := func(iter *balancer.Iterator) int {
		return len(allAttempts) + len(iter.Attempts())
	}
	// reachedMaxTotalAttempts 判断累计决策纪录是否已达到「最大总尝试次数」上限。
	// 传入当前路由迭代器以统计本轮的决策记录；0/负数已在调用方回退为默认上限。
	reachedMaxTotalAttempts := func(iter *balancer.Iterator) bool {
		if maxTotalAttempts <= 0 {
			return false
		}
		return roundDecisions(iter) >= maxTotalAttempts
	}
	// roundAppended 标记当前路由轮次迭代器的决策记录是否已并入 allAttempts。
	// goto exhausted 可能在轮次中途触发（达到最大总尝试次数），此时需要补齐当前迭代器
	// 的记录以保证 relay log 完整；而正常轮次结束已通过累加语句并入，避免重复。
	roundAppended := false
	// lastRoundForwarded 记录上一轮是否有真实转发。若某一轮所有决策都是跳过/熔断（无真实
	// 转发），说明渠道全部不可用，继续下一轮只会重复同样的失败并膨胀 attempts，应快速失败。
	lastRoundForwarded := 0

	// 应用全局模型映射表（Phase 7）
	requestModel = modelmapping.Resolve(req.operationCtx, requestModel, group.ID)

	for routeRound := 1; routeRound <= maxRouteRetries; routeRound++ {
		roundAppended = false
		if isClientDisconnected(req.clientCtx) {
			return nil, handleClientDisconnect(req, allAttempts)
		}
		if err := req.operationCtx.Err(); err != nil {
			lastErr = err
			logRelayErrorfByContext(err, "relay operation ended before request completed: %v", err)
			req.metrics.Save(false, err, allAttempts)
			return nil, err
		}

		// 快速失败防护（issue #192）：上一轮完全没有真实转发（全部是冷却/熔断跳过），
		// 说明当前所有渠道都不可用。继续下一轮只会以相同方式失败并重复膨胀 attempts，
		// 这里直接落到 exhausted 返回 502，而不是继续死循环。
		if roundAppended && lastRoundForwarded == 0 && routeRound > 1 {
			log.Warnf("[%s] all channels unavailable (no real progress in round R%d), failing fast: model=%s, decisions=%d, last_error=%v",
				req.internalRequest.RawAPIFormat, routeRound-1, requestModel, len(allAttempts), lastErr)
			goto exhausted
		}

		routeIter := balancer.NewIterator(group, req.apiKeyID, requestModel, parseExcludedChannels(req.c.GetString("excluded_channels")))
		req.iter = routeIter

		for routeIter.Next() {
			if reachedMaxTotalAttempts(routeIter) {
				lastErr = fmt.Errorf("reached relay max total attempts: %d", maxTotalAttempts)
				goto exhausted
			}
			if isClientDisconnected(req.clientCtx) {
				return nil, handleClientDisconnect(req, allAttempts)
			}
			if err := req.operationCtx.Err(); err != nil {
				lastErr = err
				logRelayErrorfByContext(err, "relay operation ended before request completed: %v", err)
				req.metrics.Save(false, err, allAttempts)
				return nil, err
			}

			item := routeIter.Item()
			channel, err := ch.Get(item.ChannelID, req.operationCtx)
			if err != nil {
				log.Warnf("failed to get channel %d: %v", item.ChannelID, err)
				routeIter.Skip(item.ChannelID, 0, buildChannelName(item.ChannelID), fmt.Sprintf("channel not found: %v", err))
				continue
			}
			if !channel.Enabled {
				routeIter.Skip(channel.ID, 0, channel.Name, "channel disabled")
				continue
			}

			// Apply global model mapping before resolving to upstream model
			mappedModel := modelmapping.Resolve(req.operationCtx, requestModel, group.ID)

			resolvedModelName := resolveCandidateModelName(mappedModel, item)
			if strings.TrimSpace(resolvedModelName) == "" {
				routeIter.Skip(channel.ID, 0, channel.Name, "resolved upstream model is empty")
				continue
			}

			attemptTypes := outboundAttemptTypes(channel.Type, req.internalRequest, group.OutboundFormat)
			if len(attemptTypes) == 0 || outbound.Get(attemptTypes[0]) == nil {
				routeIter.Skip(channel.ID, 0, channel.Name, fmt.Sprintf("unsupported channel type: %d", channel.Type))
				continue
			}
			if req.internalRequest.IsEmbeddingRequest() && !outbound.IsEmbeddingChannelType(channel.Type) {
				routeIter.Skip(channel.ID, 0, channel.Name, "channel type not compatible with embedding request")
				continue
			}
			if req.internalRequest.IsChatRequest() && !outbound.IsChatChannelType(channel.Type) {
				routeIter.Skip(channel.ID, 0, channel.Name, "channel type not compatible with chat request")
				continue
			}
			if !isZenCandidateChannelAllowed(requestModel, channel.Type, req.internalRequest.IsEmbeddingRequest()) {
				routeIter.Skip(channel.ID, 0, channel.Name, "channel type not preferred for zen model prefix")
				continue
			}

			req.internalRequest.Model = resolvedModelName
			var failedKeyIDs []int
			rateLimitHoldWaited := time.Duration(0)
			for keyRound := 1; keyRound <= maxKeyRetriesPerRoute; keyRound++ {
				if reachedMaxTotalAttempts(routeIter) {
					lastErr = fmt.Errorf("reached relay max total attempts: %d", maxTotalAttempts)
					goto exhausted
				}
				if isClientDisconnected(req.clientCtx) {
					return nil, handleClientDisconnect(req, allAttempts)
				}
				if err := req.operationCtx.Err(); err != nil {
					lastErr = err
					logRelayErrorfByContext(err, "relay operation ended: %v", err)
					req.metrics.Save(false, err, allAttempts)
					return nil, err
				}

				var usedKey dbmodel.ChannelKey
				var poolAccount *dbmodel.PoolAccount
				var poolCredType string
				var poolProxyConfigID *int
				var poolBaseURL, poolPlatform, poolType string
				var poolAccountID int
				if channel.PoolID > 0 {
					// 号池模式：从池调度器选账号。
					sessionHash := strconv.Itoa(req.apiKeyID) + ":" + requestModel
					var excludeAccountIDs []int
					for _, kid := range failedKeyIDs {
						excludeAccountIDs = append(excludeAccountIDs, kid)
					}
					acct, selErr := poolscheduler.SelectAccount(channel.PoolID, sessionHash, excludeAccountIDs, 1, resolvedModelName)
					if selErr != nil || acct == nil {
						routeIter.Skip(channel.ID, 0, channel.Name, "no available pool account")
						lastErr = fmt.Errorf("channel %s: no available pool account", channel.Name)
						break
					}
					poolAccount = acct
					cred := dbmodel.ParsePoolCredential(acct.Credentials)
					poolCredType = cred.Type
					poolProxyConfigID = acct.ProxyConfigID
					poolBaseURL = acct.BaseURL
					poolPlatform = acct.Platform
					poolType = cred.Type
					poolAccountID = acct.ID
					usedKey = dbmodel.ChannelKey{
						ID:         acct.ID,
						ChannelID:  channel.ID,
						Enabled:    true,
						ChannelKey: cred.EffectiveKeyWithExtra(acct.Platform, acct.GetExtra()),
						Priority:   acct.Priority,
					}
				} else {
					// keyRound == 1 但 failedKeyIDs 非空 = 失败提示/熔断跳过后的重选
					// （跳过分支 keyRound-- 不消耗配额）。此时必须走排除路径：
					// 无排除的选 key 是确定性的，会选回刚被跳过的同一个 key，
					// 造成「跳过 → 重选同 key → 再跳过」的死循环，attempts 记录
					// 无限增长（issue #192，实测单条 relay log 440 万条记录）。
					if keyRound == 1 && len(failedKeyIDs) == 0 {
						usedKey = channel.GetChannelKeyWithCooldown(resolvedModelName, ratelimitCooldown)
					} else {
						usedKey, _ = PrepareCandidateForRetry(channel, failedKeyIDs, routeIter, ratelimitCooldown, resolvedModelName)
					}
				}
				if usedKey.ChannelKey == "" {
					// When the key loop exits via break without forwarding
					// (e.g. all keys in rate-limit cooldown), record a skip so the
					// relay log captures the channel info and reason. Without this,
					// single-channel groups return 502 with empty channel name.
					if keyRound == 1 {
						routeIter.Skip(channel.ID, usedKey.ID, channel.Name, "no available key (all keys in cooldown or disabled)")
						lastErr = fmt.Errorf("channel %s: no available key (all keys in cooldown or disabled)", channel.Name)
					}
					break
				}
				// 号池模式跳过熔断器和失败提示（池调度器有自己的健康管理）。
				if channel.PoolID == 0 {
					if hint, ok := globalFailureHintCache.get(channel.ID, usedKey.ID, resolvedModelName); ok {
						failedKeyIDs = append(failedKeyIDs, usedKey.ID)
						routeIter.Skip(channel.ID, usedKey.ID, channel.Name, failureHintSkipReason(hint))
						keyRound--
						continue
					}
					if routeIter.SkipCircuitBreak(channel.ID, usedKey.ID, channel.Name, resolvedModelName) {
						failedKeyIDs = append(failedKeyIDs, usedKey.ID)
						keyRound--
						continue
					}
				}

				log.Infof("request model %s, mode: %d, channel: %s (%s) model: %s key_id: %d (route R%d, key %d/%d, sticky=%t)",
					requestModel, group.Mode, channel.Name, channel.Type, resolvedModelName, usedKey.ID,
					routeRound, keyRound, maxKeyRetriesPerRoute, routeIter.IsSticky())

				var result attemptResult
				for adapterIndex, attemptType := range attemptTypes {
					outAdapter := outbound.Get(attemptType)
					if outAdapter == nil {
						continue
					}
					ra := &relayAttempt{
						relayRequest:         req,
						outAdapter:           outAdapter,
						adapterType:          attemptType,
						channel:              channel,
						usedKey:              usedKey,
						firstTokenTimeOutSec: group.FirstTokenTimeOut,
						attemptTimeOutSec:    group.AttemptTimeOut,
						tryIndex:             keyRound,
						tryTotal:             maxKeyRetriesPerRoute,
						poolCredType:         poolCredType,
						poolProxyConfigID:    poolProxyConfigID,
						poolBaseURL:          poolBaseURL,
						poolPlatform:         poolPlatform,
						poolType:             poolType,
						poolAccountID:        poolAccountID,
						poolAccount:          poolAccount,
					}

					result = ra.attempt()
					if result.Success {
						if adapterIndex > 0 {
							log.Infof("[%s] adapter fallback succeeded on channel %s: %s → %s",
								req.internalRequest.RawAPIFormat, channel.Name, attemptTypes[0], attemptType)
						}
						break
					}
					if !shouldTryAdapterFallback(result, adapterIndex, len(attemptTypes)) {
						break
					}
					log.Infof("[%s] %s adapter failed on channel %s, falling back to %s: %v",
						req.internalRequest.RawAPIFormat, attemptType, channel.Name, attemptTypes[adapterIndex+1], result.Err)
				}
				currentAttempts := append(allAttempts, req.iter.Attempts()...)
				if result.Success {
					if poolAccount != nil {
						poolscheduler.ReportResult(channel.PoolID, poolAccount.ID, true, 0, 0)
						poolscheduler.ReleaseSlot(channel.PoolID, poolAccount.ID)
					}
					namespace, requestText, _ := semanticCacheStoreMetadata(req.internalRequest)
					req.metrics.Save(true, nil, currentAttempts)
					return newInflightRelayResult(cloneInternalResponse(req.metrics.InternalResponse), req.internalRequest.Model, currentAttempts, namespace, requestText), nil
				}

				// 号池模式：上报失败 + 释放槽位 + 设置冷却。
				if poolAccount != nil {
					poolscheduler.ReportResult(channel.PoolID, poolAccount.ID, false, 0, 0)
					poolscheduler.ReleaseSlot(channel.PoolID, poolAccount.ID)
					if result.Decision.Code == http.StatusTooManyRequests {
						poolscheduler.SetRateLimitCooldown(channel.PoolID, poolAccount.ID, time.Now().Add(5*time.Minute))
					} else if result.Decision.Code >= 500 {
						poolscheduler.SetOverload(channel.PoolID, poolAccount.ID, time.Now().Add(60*time.Second))
					}
					// P0 调度健壮性：OpenAI 403 阈值禁用 / OAuth 401 临时禁用（对齐 sub2api ratelimit_service）。
					handlePoolAuthError(poolAccount, poolCredType, result.Decision.Code)
				}

				// 熔断器和 Auto 策略：在所有 adapter 类型（如 Responses→Chat）均失败后才记录，
				// 避免 Response adapter 降级到 Chat 的过程中误触发熔断。
				if channel.PoolID == 0 && (result.Decision.Scope == ScopeNextChannel || result.Decision.Scope == ScopeAbortAll) {
					balancer.RecordFailure(channel.ID, usedKey.ID, resolvedModelName)
					balancer.RecordAutoFailure(channel.ID, resolvedModelName)
				}

				// Client disconnected — stop all retries immediately without
				// recording failure hints or attempting further channels.
				if errors.Is(result.Err, errClientDisconnected) {
					req.metrics.Save(false, result.Err, currentAttempts)
					return nil, result.Err
				}

				// hold 路径：本轮 429 会立刻再试同一渠道，不写 failure hint / 不记 failedKey，
				// 并清掉 attempt() 里刚写入的 key 冷却，避免间隔到期后仍被挡住。
				holdingRateLimit := shouldHoldOnRateLimit(rateLimitHoldCfg, result.Decision) &&
					canContinueRateLimitHold(rateLimitHoldCfg, rateLimitHoldWaited)
				if channel.PoolID == 0 && !holdingRateLimit {
					recordFailureHint(channel.ID, usedKey.ID, resolvedModelName, result.Decision, result.Err, ratelimitCooldown)
				}
				switch result.Decision.Scope {
				case ScopeNone:
					lastErr = result.Err
					req.metrics.Save(false, lastErr, currentAttempts)
					// 400 类客户端错误不再重试，把上游错误体原样回给下游，
					// 避免吞成 502 导致客户端无法识别 context_length_exceeded。
					writeClientTerminalError(req.c, result.Decision.Code, result.Err)
					return nil, result.Err
				case ScopeAbortAll:
					lastErr = result.Err
					req.metrics.Save(false, result.Err, currentAttempts)
					return nil, result.Err
				case ScopeSameChannel:
					lastErr = result.Err
					// 可选：429 时在当前渠道内延时重试，而不是立刻换 Key/渠道。
					// 默认关闭，保持历史「马上 failover」行为。
					if shouldHoldOnRateLimit(rateLimitHoldCfg, result.Decision) {
						if canContinueRateLimitHold(rateLimitHoldCfg, rateLimitHoldWaited) {
							if channel.PoolID == 0 {
								balancer.ClearKeyCooldown(channel.ID, usedKey.ID, resolvedModelName)
							}
							holdCtx := req.clientCtx
							if holdCtx == nil {
								holdCtx = req.operationCtx
							}
							if !waitRateLimitHold(holdCtx, rateLimitHoldCfg, channel.Name, rateLimitHoldWaited) {
								return nil, handleClientDisconnect(req, currentAttempts)
							}
							rateLimitHoldWaited += rateLimitHoldCfg.Interval
							if rateLimitHoldWaited > rateLimitHoldCfg.MaxWait {
								rateLimitHoldWaited = rateLimitHoldCfg.MaxWait
							}
							// 不消耗 keyRound 配额：这是时间维度的坚持，不是换 Key。
							keyRound--
							continue
						}
						// 等待预算耗尽：结束本渠道，转下一渠道。
						failedKeyIDs = append(failedKeyIDs, usedKey.ID)
						break
					}
					failedKeyIDs = append(failedKeyIDs, usedKey.ID)
				case ScopeNextChannel:
					lastErr = result.Err
					failedKeyIDs = append(failedKeyIDs, usedKey.ID)
					break
				default:
					lastErr = result.Err
					req.metrics.Save(false, lastErr, currentAttempts)
					resp.BadGateway(req.c)
					return nil, result.Err
				}
			}
		}
		// 轮次正常结束：把本轮迭代器的全部决策记录累加到 allAttempts（用于日志/metrics），
		// 并把本轮真实转发次数累加到 forwardedBase，供下一轮的最大总尝试次数检查。
		allAttempts = append(allAttempts, routeIter.Attempts()...)
		lastRoundForwarded = routeIter.ForwardedAttempts()
		roundAppended = true
	}

exhausted:
	// 若 exhausted 由轮次中途触发（达到最大总尝试次数），当前迭代器的决策记录尚未累加，
	// 这里补齐最新的记录，保证 relay log 能完整展示冷却/熔断跳过与真实失败的链路
	// （见 issue #95 改动2/6）。正常结束的轮次已通过上方累加语句并入，不重复追加。
	if !roundAppended && req.iter != nil {
		allAttempts = append(allAttempts, req.iter.Attempts()...)
	}
	req.metrics.Save(false, lastErr, allAttempts)
	log.Warnf("[%s] all channels exhausted: model=%s, attempts=%d, last_error=%v",
		req.internalRequest.RawAPIFormat, requestModel, len(allAttempts), lastErr)
	// 对外返回通用文案，避免把上游内部错误体（可能含上游实现细节）回显给客户端；
	// 完整错误已写入上方日志与 relay_log。
	resp.Error(req.c, http.StatusBadGateway, "all channels failed")
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("all channels failed")
}
