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
	"github.com/lingyuins/octopus/internal/helper"
	dbmodel "github.com/lingyuins/octopus/internal/model"
	ch "github.com/lingyuins/octopus/internal/op/channel"
	grp "github.com/lingyuins/octopus/internal/op/group"
	rl "github.com/lingyuins/octopus/internal/op/ratelimitstore"
	stg "github.com/lingyuins/octopus/internal/op/setting"
	st "github.com/lingyuins/octopus/internal/op/stats"
	"github.com/lingyuins/octopus/internal/relay/balancer"
	"github.com/lingyuins/octopus/internal/relay/condition"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/transformer/inbound"
	"github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
	"github.com/lingyuins/octopus/internal/transformer/rewrite"
	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/lingyuins/octopus/internal/utils/semantic_cache"
	"github.com/tmaxmax/go-sse"
)

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
	if err := json.Unmarshal([]byte(perModelJSON), &quotas); err != nil {
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

func initSemanticCacheFromSettings() {
	enabled, _ := stg.GetBool(dbmodel.SettingKeySemanticCacheEnabled)
	if !enabled {
		semantic_cache.Clear()
		return
	}
	ttl, _ := stg.GetInt(dbmodel.SettingKeySemanticCacheTTL)
	thresholdRaw, _ := stg.GetInt(dbmodel.SettingKeySemanticCacheThreshold)
	maxEntries, _ := stg.GetInt(dbmodel.SettingKeySemanticCacheMaxEntries)
	if ttl <= 0 {
		ttl = 3600
	}
	if thresholdRaw < 0 || thresholdRaw > 100 {
		thresholdRaw = 98
	}
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	semantic_cache.Init(maxEntries, float64(thresholdRaw)/100.0, ttl)
}

func resolveCandidateModelName(requestModel string, item dbmodel.GroupItem) string {
	if upstreamModel, ok := resolveRequestedUpstreamModel(requestModel); ok {
		if strings.TrimSpace(item.ModelName) == "" || strings.EqualFold(strings.TrimSpace(item.ModelName), "zen") {
			return upstreamModel
		}
	}
	return item.ModelName
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
	// Initialize semantic cache from settings
	initSemanticCacheFromSettings()

	if shouldUseRelayStreamSession(internalRequest) {
		sessionHash := buildRelayStreamSessionHash(endpointType, int(inboundType), apiKeyID, internalRequest.RawRequest)
		session, created, err := acquireRelayStreamSession(internalRequest.ConversationID, apiKeyID, sessionHash)
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
	iter := balancer.NewIterator(group, apiKeyID, requestModel)
	if iter.Len() == 0 {
		lastErr = errors.New("no available channel")
		resp.Error(c, http.StatusServiceUnavailable, "no available channel")
		return
	}

	// 根据分组端点提供方做请求兼容改写
	internalRequest = rewriteConversationRequestByProvider(group, internalRequest)

	// 初始化 Metrics
	metrics := NewRelayMetrics(apiKeyID, requestModel, endpointType, group.EndpointType, internalRequest)

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
		iter:              iter,
		streamSession:     streamSession,
	}

	if endpointFamily := semanticCacheEndpointFamily(endpointType, inboundType); endpointFamily != "" {
		served, payload, cacheErr := maybeServeSemanticCacheHit(c, req, endpointFamily)
		if cacheErr != nil {
			log.Warnf("semantic cache lookup failed: %v", cacheErr)
		}
		if served {
			if normalizedPayload := semanticCacheHitPayload(payload, internalRequest); len(normalizedPayload) > 0 {
				if internalResponse, parseErr := buildSemanticCacheHitInternalResponse(internalRequest, normalizedPayload); parseErr == nil {
					metrics.SetInternalResponse(internalResponse, internalRequest.Model)
				}
			}
			metrics.Save(true, nil, nil)
			return
		}
	}

	maxKeyRetriesPerRoute := getMaxAttemptsPerCandidate()
	maxRouteRetries := getMaxRouteRetries()
	ratelimitCooldown := getRatelimitCooldown()
	maxTotalAttempts := getMaxTotalAttempts()

	var allAttempts []dbmodel.ChannelAttempt

	for routeRound := 1; routeRound <= maxRouteRetries; routeRound++ {
		// 每轮开始前检查操作上下文
		if err := req.operationCtx.Err(); err != nil {
			lastErr = err
			logRelayErrorfByContext(err, "relay operation ended before request completed: %v", err)
			metrics.Save(false, err, allAttempts)
			return
		}

		// 每轮路由重试重建迭代器（粘性渠道自然在最前）
		routeIter := balancer.NewIterator(group, apiKeyID, requestModel)

		for routeIter.Next() {
			if maxTotalAttempts > 0 && len(allAttempts) >= maxTotalAttempts {
				lastErr = fmt.Errorf("reached relay max total attempts: %d", maxTotalAttempts)
				goto exhausted
			}
			if err := req.operationCtx.Err(); err != nil {
				lastErr = err
				logRelayErrorfByContext(err, "relay operation ended before request completed: %v", err)
				metrics.Save(false, err, allAttempts)
				return
			}

			item := routeIter.Item()

			channel, err := ch.Get(item.ChannelID, req.operationCtx)
			if err != nil {
				log.Warnf("failed to get channel %d: %v", item.ChannelID, err)
				routeIter.Skip(item.ChannelID, 0, fmt.Sprintf("channel_%d", item.ChannelID), fmt.Sprintf("channel not found: %v", err))
				continue
			}
			if !channel.Enabled {
				routeIter.Skip(channel.ID, 0, channel.Name, "channel disabled")
				continue
			}

			resolvedModelName := resolveCandidateModelName(requestModel, item)
			if strings.TrimSpace(resolvedModelName) == "" {
				routeIter.Skip(channel.ID, 0, channel.Name, "resolved upstream model is empty")
				continue
			}

			// 出站适配器 + 类型兼容性（渠道级，一次检查）
			outAdapter := outbound.Get(channel.Type)
			if outAdapter == nil {
				routeIter.Skip(channel.ID, 0, channel.Name, fmt.Sprintf("unsupported channel type: %d", channel.Type))
				continue
			}
			if internalRequest.IsEmbeddingRequest() && !outbound.IsEmbeddingChannelType(channel.Type) {
				routeIter.Skip(channel.ID, 0, channel.Name, "channel type not compatible with embedding request")
				continue
			}
			if internalRequest.IsChatRequest() && !outbound.IsChatChannelType(channel.Type) {
				routeIter.Skip(channel.ID, 0, channel.Name, "channel type not compatible with chat request")
				continue
			}
			if !isZenCandidateChannelAllowed(requestModel, channel.Type, internalRequest.IsEmbeddingRequest()) {
				routeIter.Skip(channel.ID, 0, channel.Name, "channel type not preferred for zen model prefix")
				continue
			}

			internalRequest.Model = resolvedModelName

			// 渠道内 Key 级重试
			var failedKeyIDs []int
			for keyRound := 1; keyRound <= maxKeyRetriesPerRoute; keyRound++ {
				if maxTotalAttempts > 0 && len(allAttempts) >= maxTotalAttempts {
					lastErr = fmt.Errorf("reached relay max total attempts: %d", maxTotalAttempts)
					goto exhausted
				}
				if err := req.operationCtx.Err(); err != nil {
					lastErr = err
					logRelayErrorfByContext(err, "relay operation ended: %v", err)
					metrics.Save(false, err, allAttempts)
					return
				}

				var usedKey dbmodel.ChannelKey
				if keyRound == 1 {
					usedKey = channel.GetChannelKeyWithCooldown(ratelimitCooldown)
				} else {
					usedKey = channel.GetChannelKeyExcludingWithCooldown(failedKeyIDs, ratelimitCooldown)
				}
				if usedKey.ChannelKey == "" {
					break
				}

				// 熔断跳过不消耗 Key 重试配额
				if routeIter.SkipCircuitBreak(channel.ID, usedKey.ID, channel.Name, resolvedModelName) {
					failedKeyIDs = append(failedKeyIDs, usedKey.ID)
					keyRound--
					continue
				}

				log.Infof("request model %s, mode: %d, channel: %s model: %s key_id: %d (route R%d, key %d/%d, sticky=%t)",
					requestModel, group.Mode, channel.Name, resolvedModelName, usedKey.ID,
					routeRound, keyRound, maxKeyRetriesPerRoute, routeIter.IsSticky())

				ra := &relayAttempt{
					relayRequest:         req,
					outAdapter:           outAdapter,
					channel:              channel,
					usedKey:              usedKey,
					firstTokenTimeOutSec: group.FirstTokenTimeOut,
					tryIndex:             keyRound,
					tryTotal:             maxKeyRetriesPerRoute,
				}

				result := ra.attempt()
				// 当前请求的 attempt 记录挂在 relayRequest.iter（即 routeIter）上；
				// 不要从最外层 req.iter 读取，否则成功日志会丢失渠道信息。
				currentAttempts := append(allAttempts, req.iter.Attempts()...)
				if result.Success {
					lastErr = nil
					metrics.Save(true, nil, currentAttempts)
					return
				}

				switch result.Decision.Scope {
				case ScopeNone:
					lastErr = result.Err
					metrics.Save(false, lastErr, currentAttempts)
					resp.BadGateway(c)
					return
				case ScopeAbortAll:
					lastErr = result.Err
					metrics.Save(false, result.Err, currentAttempts)
					return
				case ScopeSameChannel:
					lastErr = result.Err
					failedKeyIDs = append(failedKeyIDs, usedKey.ID)
					// 继续 keyRound 循环，尝试同渠道下一个 Key
				case ScopeNextChannel:
					lastErr = result.Err
					failedKeyIDs = append(failedKeyIDs, usedKey.ID)
					break // 跳出 key 循环，尝试下一个渠道
				default:
					lastErr = result.Err
					metrics.Save(false, lastErr, currentAttempts)
					resp.BadGateway(c)
					return
				}
			}
		}
		allAttempts = append(allAttempts, routeIter.Attempts()...)
	}

exhausted:
}

// attempt 统一管理一次通道尝试的完整生命周期
func (ra *relayAttempt) attempt() attemptResult {
	span := ra.iter.StartAttempt(ra.channel.ID, ra.usedKey.ID, ra.channel.Name, ra.internalRequest.Model)

	// 转发请求
	statusCode, fwdErr := ra.forward()

	// 检查是否已写入流式响应
	written := ra.c.Writer.Written()

	// 使用错误分类驱动决策
	decision := ClassifyRelayError(statusCode, fwdErr, written)

	// 更新 channel key 状态
	ra.usedKey.StatusCode = statusCode
	ra.usedKey.LastUseTimeStamp = time.Now().Unix()

	if decision.Scope == ScopeNone && !decision.IsError {
		// ====== 成功 ======
		ra.collectResponse()
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
		// 会话保持：更新粘性记录
		balancer.SetSticky(ra.apiKeyID, ra.requestModel, ra.channel.ID, ra.usedKey.ID)

		return attemptResult{Success: true, Decision: decision}
	}

	// ====== 失败 ======
	ch.KeyUpdate(ra.usedKey)

	// 构造日志消息
	msg := decision.String()
	if ra.tryTotal > 1 {
		msg = fmt.Sprintf("attempt %d/%d: %s", ra.tryIndex, ra.tryTotal, msg)
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

	// 熔断器和 Auto 策略：只在换候选或停止时记录失败
	// 换 Key 重试不触发熔断计数，避免误熔断
	if decision.Scope == ScopeNextChannel || decision.Scope == ScopeAbortAll {
		balancer.RecordFailure(ra.channel.ID, ra.usedKey.ID, ra.internalRequest.Model)
		balancer.RecordAutoFailure(ra.channel.ID, ra.internalRequest.Model)
	}

	if written {
		ra.collectResponse()
	}

	// 记录决策日志
	if decision.IsError {
		logRelayErrorfByContext(fwdErr, "channel %s failed on attempt %d/%d: %s (decision: %s)",
			ra.channel.Name, ra.tryIndex, ra.tryTotal, fwdErr, decision.Scope.String())
	}

	return attemptResult{
		Success:  false,
		Written:  written,
		Err:      fmt.Errorf("channel %s failed on attempt %d/%d: %v", ra.channel.Name, ra.tryIndex, ra.tryTotal, fwdErr),
		Decision: decision,
	}
}

// parseRequest 解析并验证入站请求
func parseRequest(inboundType inbound.InboundType, c *gin.Context) (*model.InternalLLMRequest, model.Inbound, error) {
	body, err := readLimitedRequestBody(c, maxRelayJSONBodyBytes)
	if err != nil {
		resp.Error(c, relayRequestBodyErrorStatus(err), err.Error())
		return nil, nil, err
	}

	inAdapter := inbound.Get(inboundType)
	internalRequest, err := inAdapter.TransformRequest(c.Request.Context(), body)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return nil, nil, err
	}

	// Pass through the original query parameters
	internalRequest.Query = c.Request.URL.Query()

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

	requestForOutbound, effectiveRewrite, err := prepareInternalRequestForOutbound(ra.channel, ra.internalRequest, ra.groupEndpointType)
	if err != nil {
		log.Warnf("failed to prepare outbound request data: %v", err)
		return 0, fmt.Errorf("failed to prepare outbound request data: %w", err)
	}

	// 构建出站请求
	outboundRequest, err := ra.outAdapter.TransformRequest(
		ctx,
		requestForOutbound,
		ra.channel.GetNormalizedBaseUrl(),
		ra.usedKey.ChannelKey,
	)
	if err != nil {
		log.Warnf("failed to create request: %v", err)
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

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
			return 0, err
		}
		return response.StatusCode, nil
	}
	if err := ra.handleResponse(ctx, response); err != nil {
		return 0, err
	}
	return response.StatusCode, nil
}

func (ra *relayAttempt) handleForwardResponse(response *http.Response) (int, error) {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return response.StatusCode, nil
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}
	return response.StatusCode, fmt.Errorf("upstream error: %d: %s", response.StatusCode, string(body))
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
	httpClient, err := helper.ChannelHttpClient(ra.channel)
	if err != nil {
		log.Warnf("failed to get http client: %v", err)
		return nil, err
	}

	response, err := httpClient.Do(req)
	if err != nil {
		logRelayErrorfByContext(err, "failed to send request: %v", err)
		return nil, err
	}

	return response, nil
}

// handleStreamResponse 处理流式响应
func (ra *relayAttempt) handleStreamResponse(ctx context.Context, response *http.Response) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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
	clientDone := ra.clientCtx.Done()
	clientDisconnected := false
	clientDisconnectLogged := false
	markClientDisconnected := func() {
		if clientDisconnected {
			return
		}
		clientDisconnected = true
		clientDone = nil
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

	for {
		select {
		case <-clientDone:
			if ra.streamSession == nil {
				log.Infof("client disconnected, stopping stream")
				return nil
			}
			markClientDisconnected()
		case <-firstTokenC:
			logClientDisconnected()
			log.Warnf("first token timeout (%ds), switching channel", ra.firstTokenTimeOutSec)
			if err := response.Body.Close(); err != nil {
				log.Warnf("failed to close response body on first token timeout: %v", err)
			}
			return fmt.Errorf("first token timeout (%ds)", ra.firstTokenTimeOutSec)
		case r, ok := <-results:
			if !ok {
				logClientDisconnected()
				if ra.streamSession != nil {
					ra.streamSession.Finish(nil)
				}
				log.Infof("stream end")
				return nil
			}
			if r.err != nil {
				logClientDisconnected()
				logRelayErrorfByContext(r.err, "failed to read event: %v", r.err)
				return fmt.Errorf("failed to read stream event: %w", r.err)
			}

			data, err := ra.transformStreamData(ctx, r.data)
			if err != nil || len(data) == 0 {
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

// transformStreamData 转换流式数据
func (ra *relayAttempt) transformStreamData(ctx context.Context, data string) ([]byte, error) {
	internalStream, err := ra.outAdapter.TransformStream(ctx, []byte(data))
	if err != nil {
		logRelayErrorfByContext(err, "failed to transform stream: %v", err)
		return nil, err
	}
	if internalStream == nil {
		return nil, nil
	}

	inStream, err := ra.inAdapter.TransformStream(ctx, internalStream)
	if err != nil {
		logRelayErrorfByContext(err, "failed to transform stream: %v", err)
		return nil, err
	}

	return inStream, nil
}

// handleResponse 处理非流式响应
func (ra *relayAttempt) handleResponse(ctx context.Context, response *http.Response) error {
	internalResponse, err := ra.outAdapter.TransformResponse(ctx, response)
	if err != nil {
		logRelayErrorfByContext(err, "failed to transform response: %v", err)
		return fmt.Errorf("failed to transform outbound response: %w", err)
	}
	applyReasoningExhaustedHeader(ra.c, internalResponse)

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
	if provider != "deepseek" && provider != "mimo" {
		return req
	}

	cloned := *req
	if len(req.Messages) > 0 {
		cloned.Messages = make([]model.Message, len(req.Messages))
		for i, msg := range req.Messages {
			cloned.Messages[i] = msg
			if provider == "deepseek" {
				cloned.Messages[i].Reasoning = nil
			} else if provider == "mimo" {
				cloned.Messages[i].ReasoningSignature = nil
			}
		}
	}
	return &cloned
}
