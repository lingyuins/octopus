package relay

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/conf"
	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/relay/balancer"
	"github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

// maxSSEEventSize 定义 SSE 事件的最大大小。
// 对于图像生成模型（如 gemini-3-pro-image-preview），返回的 base64 编码图像数据
// 可能非常大（高分辨率图像可能超过 10MB），因此需要设置足够大的缓冲区。
// 默认 32MB，可通过环境变量 OCTOPUS_RELAY_MAX_SSE_EVENT_SIZE 覆盖。
var maxSSEEventSize = 32 * 1024 * 1024

// maxReasoningBufferBytes 是 buffer 策略下暂存的 reasoning-only chunk 累计字节软上限。
// reasoning-only 流（无可见内容到达）下，chunk 持续追加到 reasoningBuffer 且仅在可见
// 内容到达或响应过滤拦截时 flush，长流会让缓冲无界增长（内存峰值）。达到软上限时
// 直接丢弃当前 buffer 并重置计数（不向客户端 flush，以保持空输出重试安全性）。
// 8 MiB 足够正常 reasoning 输出，与 stream session maxBytes（4 MiB）同量级但留余量。
const maxReasoningBufferBytes = 8 * 1024 * 1024

const (
	defaultMaxRetryPerCandidate = 3
	defaultMaxRouteRetries      = 2
	defaultRatelimitCooldown    = 300
	// defaultMaxTotalAttempts 是所有决策（真实转发 + 冷却跳过 + 熔断跳过）的最大纪录上限。
	// 当 relay_max_total_attempts 未设置或为 0 时回退到此默认值，避免所有渠道都不可用时
	// （key 全冷却 / 熔断）attempts 无上限膨胀导致 relay_logs 单行爆炸（见 issue #192）。
	defaultMaxTotalAttempts = 64
	maxErrorBodyBytes       = 64 << 10 // 64 KiB — upstream error responses should be concise
)

var errEmptyOutput = errors.New("upstream returned empty output (no visible content)")

func init() {
	if raw := strings.TrimSpace(os.Getenv(strings.ToUpper(conf.APP_NAME) + "_RELAY_MAX_SSE_EVENT_SIZE")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			maxSSEEventSize = v
		}
	}
}

func getMaxRetryPerCandidate() int {
	v, err := setting.GetInt(dbmodel.SettingKeyRelayRetryCount)
	// 允许设为 0：此时 maxAttemptsPerCandidate = 1，Key 循环只跑一次，
	// 失败直接跳到下一个渠道。负数或读取失败才回退默认值（见 issue #95）。
	if err != nil || v < 0 {
		return defaultMaxRetryPerCandidate
	}
	return v
}

func getMaxAttemptsPerCandidate() int {
	return getMaxRetryPerCandidate() + 1
}

func getMaxRouteRetries() int {
	v, err := setting.GetInt(dbmodel.SettingKeyRelayRouteRetries)
	if err != nil || v < 1 {
		return defaultMaxRouteRetries
	}
	return v
}

func getRatelimitCooldown() int {
	v, err := setting.GetInt(dbmodel.SettingKeyRatelimitCooldown)
	if err != nil || v < 0 {
		return defaultRatelimitCooldown
	}
	return v
}

func getMaxTotalAttempts() int {
	v, err := setting.GetInt(dbmodel.SettingKeyRelayMaxTotalAttempts)
	// 0 以及读取失败/负数都回退到默认上限（issue #192）：所有渠道不可用时，
	// 若按「0 = 不限制」处理会无限膨胀 attempts，单条 relay_log 可达数百 MB。
	if err != nil || v <= 0 {
		return defaultMaxTotalAttempts
	}
	return v
}

// isRetryEmptyOutputEnabled 返回是否启用空输出重试。
// 当上游返回 200 但输出为空（无可见内容：无文本、无工具调用、无多模态、无音频）时自动重试。
// 适用于流式与非流式。issue #155：推理模型消耗大量 reasoning tokens 但不产生可见内容时也应重试。
func isRetryEmptyOutputEnabled() bool {
	v, err := setting.GetBool(dbmodel.SettingKeyRetryEmptyOutput)
	if err != nil {
		return true // 默认启用
	}
	return v
}

// getReasoningBufferStrategy 返回有效的推理缓冲策略：
// 1. 优先使用分组的 reasoning_buffer_strategy（非空时）
// 2. 回退到全局设置 reasoning_buffer_strategy
// 3. 最终默认 "buffer"（兼容旧行为）
// 返回 "buffer" 或 "immediate"。
func getReasoningBufferStrategy(group *dbmodel.Group) string {
	if group != nil && group.ReasoningBufferStrategy != "" {
		strategy := strings.TrimSpace(group.ReasoningBufferStrategy)
		if strategy == "buffer" || strategy == "immediate" {
			return strategy
		}
	}
	v, err := setting.GetString(dbmodel.SettingKeyReasoningBufferStrategy)
	if err == nil {
		strategy := strings.TrimSpace(v)
		if strategy == "buffer" || strategy == "immediate" {
			return strategy
		}
	}
	return "buffer" // 默认缓冲策略，保持向后兼容
}

// messageHasVisibleContent 检查 Message 是否包含可见内容（文本、多模态、工具调用、音频）。
// reasoning_content / reasoning 不算可见内容（issue #155）。
func messageHasVisibleContent(msg *model.Message) bool {
	if msg == nil {
		return false
	}
	if msg.Content.Content != nil && strings.TrimSpace(*msg.Content.Content) != "" {
		return true
	}
	if len(msg.Content.MultipleContent) > 0 {
		return true
	}
	if len(msg.ToolCalls) > 0 {
		return true
	}
	if msg.Audio != nil {
		return true
	}
	return false
}

// isEmptyOutputResponse 判断非流式响应是否为"空输出"：
// 所有 Choices 的 Message 均无可见内容（无文本、无工具调用、无多模态、无音频）。
// 不依赖 CompletionTokens 判断——推理模型可能消耗大量 reasoning tokens
// 但 CompletionTokens > 0 却不产生任何可见内容（issue #155）。
func isEmptyOutputResponse(resp *model.InternalLLMResponse) bool {
	if resp == nil {
		return false
	}
	// 必须是 chat 响应（有 Choices），embedding 不适用
	if len(resp.Choices) == 0 {
		return false
	}
	for _, choice := range resp.Choices {
		if messageHasVisibleContent(choice.Message) {
			return false
		}
	}
	return true
}

// streamChunkHasVisibleContent 检查流式 chunk（Delta）是否包含可见内容。
// 用于流式空输出检测：当整个流式响应仅包含 reasoning chunks 而无可见内容时触发重试（issue #155）。
func streamChunkHasVisibleContent(resp *model.InternalLLMResponse) bool {
	if resp == nil {
		return false
	}
	for _, choice := range resp.Choices {
		if messageHasVisibleContent(choice.Delta) {
			return true
		}
	}
	return false
}

// hopByHopHeaders 定义不应转发的 HTTP 头
var hopByHopHeaders = map[string]bool{
	"authorization":       true,
	"x-api-key":           true,
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailer":             true,
	"transfer-encoding":   true,
	"upgrade":             true,
	"content-length":      true,
	"host":                true,
	"accept-encoding":     true,
	"x-forwarded-for":     true,
	"x-forwarded-host":    true,
	"x-forwarded-proto":   true,
	"x-forwarded-port":    true,
	"x-real-ip":           true,
	"forwarded":           true,
	"cf-connecting-ip":    true,
	"true-client-ip":      true,
	"x-client-ip":         true,
	"x-cluster-client-ip": true,
	"cookie":              true,
}

type relayRequest struct {
	c                 *gin.Context
	clientCtx         context.Context
	operationCtx      context.Context
	inAdapter         model.Inbound
	internalRequest   *model.InternalLLMRequest
	metrics           *RelayMetrics
	apiKeyID          int
	requestModel      string
	groupEndpointType string
	group             *dbmodel.Group // 新增：用于读取分组级推理缓冲策略
	iter              *balancer.Iterator
	streamSession     *relayStreamSession
	retryCache        *retryRequestCache
}

// relayAttempt 尝试级上下文
type relayAttempt struct {
	*relayRequest // 嵌入请求级上下文

	outAdapter           model.Outbound
	adapterType          outbound.OutboundType
	channel              *dbmodel.Channel
	usedKey              dbmodel.ChannelKey
	firstTokenTimeOutSec int
	attemptTimeOutSec    int
	tryIndex             int
	tryTotal             int
	// poolCredType 号池凭据类型（"bearer" 或 "cookie"）。空字符串表示非号池模式。
	poolCredType string
	// poolProxyConfigID 号池账号级代理配置 ID。nil 表示使用渠道级代理。
	poolProxyConfigID *int
	// poolBaseURL 号池账号 base_url 覆盖。非空时覆盖渠道 base_url。
	poolBaseURL string
	// poolPlatform / poolType 号池账号平台与凭据类型，用于出站凭据路由。
	poolPlatform string
	poolType     string
	// poolAccountID 号池账号 ID，用于 ReportResult。
	poolAccountID int
	// poolAccount 号池账号指针（号池模式时使用），供 applyPoolCredentialHeaders 读取 extra。
	poolAccount *dbmodel.PoolAccount

	// filterCfg 缓存本次尝试的响应关键词过滤配置，避免在流式响应的每个
	// chunk 上重复读取 setting 并解析关键词 JSON。通过 getResponseFilterConfig
	// 懒加载，仅在首次需要时计算一次。
	filterCfg *responseFilterConfig
}

// getResponseFilterConfig 返回本次尝试的响应过滤配置，仅加载一次并缓存。
func (ra *relayAttempt) getResponseFilterConfig() responseFilterConfig {
	if ra.filterCfg == nil {
		cfg := loadResponseFilterConfig()
		ra.filterCfg = &cfg
	}
	return *ra.filterCfg
}

// attemptResult 封装单次尝试的结果
type attemptResult struct {
	Success  bool          // 是否成功
	Written  bool          // 流式响应是否已开始写入（不可重试）
	Err      error         // 失败时的错误
	Decision RetryDecision // 重试决策（新增）
}

// RetryScope 重试范围，明确四种行为边界
type RetryScope int

const (
	ScopeNone        RetryScope = iota // 不重试，请求结束（成功或直接失败）
	ScopeSameChannel                   // 同候选换 Key 重试
	ScopeNextChannel                   // 换下一个候选重试
	ScopeAbortAll                      // 停止所有重试（已写入流式响应）
)

func (s RetryScope) String() string {
	switch s {
	case ScopeNone:
		return "none"
	case ScopeSameChannel:
		return "same_channel"
	case ScopeNextChannel:
		return "next_channel"
	case ScopeAbortAll:
		return "abort_all"
	default:
		return "unknown"
	}
}

// RetryDecision 重试决策，包含决策范围、原因和状态码
type RetryDecision struct {
	Scope   RetryScope // 决策范围
	Reason  string     // 决策原因（日志用）
	Code    int        // HTTP 状态码（0 表示非 HTTP 错误）
	IsError bool       // 是否是错误（非成功）
}

// String 返回决策的描述字符串，用于日志
func (d RetryDecision) String() string {
	if d.Reason == "" {
		return d.Scope.String()
	}
	return fmt.Sprintf("%s (%s)", d.Scope.String(), d.Reason)
}

// ClassifyRelayError 根据状态码和错误类型返回重试决策
// 这是错误分类驱动的核心函数，明确区分：
//   - 直接失败（400 类客户端错误）
//   - 换 Key 重试（401/403/429、网络错误）
//   - 换候选重试（404、5xx、超时、上游业务错误）
//   - 停止所有重试（流式响应已部分写出且本次尝试发生错误）
//
// 参数：
//   - statusCode: HTTP 状态码（0 表示非 HTTP 错误）
//   - err: 错误对象（nil 表示成功）
//   - written: 是否已开始向客户端写出响应；仅表示后续不能安全重试，不等同于失败
//
// 返回：RetryDecision 决策结果
func ClassifyRelayError(statusCode int, err error, written bool) RetryDecision {
	// 成功：无需重试。流式成功完成即使已经写出响应，也不应视为失败。
	if err == nil && statusCode >= 200 && statusCode < 300 {
		return RetryDecision{
			Scope:   ScopeNone,
			Reason:  "success",
			Code:    statusCode,
			IsError: false,
		}
	}

	// 流式响应已部分写出且本次尝试发生错误：停止所有重试，避免再次写回客户端。
	if written {
		return RetryDecision{
			Scope:   ScopeAbortAll,
			Reason:  "stream response already written before failure",
			Code:    statusCode,
			IsError: true,
		}
	}

	// HTTP 错误：根据状态码分类
	if statusCode > 0 {
		return classifyHTTPError(statusCode, err)
	}

	// 非 HTTP 错误（网络错误、超时等）
	return classifyNonHTTPError(err)
}

// classifyHTTPError 根据 HTTP 状态码返回重试决策
func classifyHTTPError(statusCode int, err error) RetryDecision {
	switch {
	// 400 Bad Request: 客户端错误，换 channel 也无效
	case statusCode == 400:
		return RetryDecision{
			Scope:   ScopeNone,
			Reason:  "bad request, client error",
			Code:    statusCode,
			IsError: true,
		}

	// 401 Unauthorized: Key 无效，换 key 可能解决
	case statusCode == 401:
		return RetryDecision{
			Scope:   ScopeSameChannel,
			Reason:  "unauthorized, key invalid",
			Code:    statusCode,
			IsError: true,
		}

	// 403 Forbidden: Key 权限问题，换 key
	case statusCode == 403:
		return RetryDecision{
			Scope:   ScopeSameChannel,
			Reason:  "forbidden, key permission issue",
			Code:    statusCode,
			IsError: true,
		}

	// 404 Not Found: 模型/资源不存在，可能其他 channel 有
	case statusCode == 404:
		return RetryDecision{
			Scope:   ScopeNextChannel,
			Reason:  "not found, try next channel",
			Code:    statusCode,
			IsError: true,
		}

	// 429 Rate Limited: 该 key 被限，换 key（cooldown 机制也会生效）
	case statusCode == 429:
		return RetryDecision{
			Scope:   ScopeSameChannel,
			Reason:  "rate limited, try another key",
			Code:    statusCode,
			IsError: true,
		}

	// 500 Internal Server Error: 上游服务问题，换候选
	case statusCode == 500:
		return RetryDecision{
			Scope:   ScopeNextChannel,
			Reason:  "internal server error",
			Code:    statusCode,
			IsError: true,
		}

	// 502 Bad Gateway / 503 Service Unavailable / 504 Gateway Timeout: 上游网关问题，换候选
	case statusCode == 502 || statusCode == 503 || statusCode == 504:
		return RetryDecision{
			Scope:   ScopeNextChannel,
			Reason:  fmt.Sprintf("gateway error %d", statusCode),
			Code:    statusCode,
			IsError: true,
		}

	// 其他 2xx 状态码但有错误（如 transformer 错误）：换候选
	case statusCode >= 200 && statusCode < 300 && err != nil:
		return RetryDecision{
			Scope:   ScopeNextChannel,
			Reason:  "transformer error on success response",
			Code:    statusCode,
			IsError: true,
		}

	// 其他状态码：保守策略，换候选
	default:
		return RetryDecision{
			Scope:   ScopeNextChannel,
			Reason:  fmt.Sprintf("unexpected status code %d", statusCode),
			Code:    statusCode,
			IsError: true,
		}
	}
}

// classifyNonHTTPError 根据非 HTTP 错误类型返回重试决策
func classifyNonHTTPError(err error) RetryDecision {
	if err == nil {
		// 无错误但状态码无效，保守处理
		return RetryDecision{
			Scope:   ScopeNextChannel,
			Reason:  "unknown error",
			Code:    0,
			IsError: true,
		}
	}

	// 超时错误：上游响应慢，换候选
	if isTimeoutError(err) {
		return RetryDecision{
			Scope:   ScopeNextChannel,
			Reason:  "timeout",
			Code:    0,
			IsError: true,
		}
	}

	// 网络连接错误：连接失败。同渠道所有 Key 指向同一 host，换 Key 无意义，直接换渠道。
	if isNetworkError(err) {
		return RetryDecision{
			Scope:   ScopeNextChannel,
			Reason:  "network error",
			Code:    0,
			IsError: true,
		}
	}

	// 其他错误：保守策略，换候选
	return RetryDecision{
		Scope:   ScopeNextChannel,
		Reason:  err.Error(),
		Code:    0,
		IsError: true,
	}
}

// isTimeoutError 判断是否为超时错误
func isTimeoutError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	// 常见超时错误字符串匹配
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "first token timeout")
}

// isNetworkError 判断是否为网络连接错误
func isNetworkError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		// 超时错误已在 isTimeoutError 中处理，这里排除
		if netErr.Timeout() {
			return false
		}
		return true
	}
	// 常见网络错误字符串匹配
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "DNS") ||
		strings.Contains(errStr, "failed to send request")
}
