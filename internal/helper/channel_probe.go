package helper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/conf"
	appmodel "github.com/lingyuins/octopus/internal/model"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
	"github.com/lingyuins/octopus/internal/utils/log"
)

type ChannelTestResult struct {
	BaseURL      string `json:"base_url"`
	KeyRemark    string `json:"key_remark,omitempty"`
	KeyMasked    string `json:"key_masked,omitempty"`
	StatusCode   int    `json:"status_code"`
	Passed       bool   `json:"passed"`
	LatencyMS    int64  `json:"latency_ms"`
	Message      string `json:"message,omitempty"`
	ResponseBody string `json:"response_body,omitempty"`
}

type ChannelTestSummary struct {
	Passed  bool                `json:"passed"`
	Results []ChannelTestResult `json:"results"`
}

func TestChannel(ctx context.Context, request appmodel.Channel) (*ChannelTestSummary, error) {
	if conf.IsDevMockSuccess() {
		baseURL := "dev-mock://local"
		if len(request.BaseUrls) > 0 && strings.TrimSpace(request.BaseUrls[0].URL) != "" {
			baseURL = strings.TrimSpace(request.BaseUrls[0].URL)
		}
		keyMasked := "sk-o...0001"
		if len(request.Keys) > 0 && strings.TrimSpace(request.Keys[0].ChannelKey) != "" {
			keyMasked = maskSecret(request.Keys[0].ChannelKey)
		}
		log.Infof("dev mock channel test success: base_url=%s", baseURL)
		return &ChannelTestSummary{
			Passed: true,
			Results: []ChannelTestResult{{
				BaseURL:      baseURL,
				KeyMasked:    keyMasked,
				StatusCode:   http.StatusOK,
				Passed:       true,
				LatencyMS:    1,
				Message:      "ok",
				ResponseBody: devMockText,
			}},
		}, nil
	}

	client, err := ChannelHttpClient(&request)
	if err != nil {
		return nil, err
	}

	baseURLs := make([]probeBaseURL, 0, len(request.BaseUrls))
	for _, item := range request.BaseUrls {
		url := strings.TrimSpace(item.URL)
		if url != "" {
			baseURLs = append(baseURLs, probeBaseURL{
				url:        request.GetNormalizedBaseUrlFor(url),
				suffixMode: item.SuffixMode,
			})
		}
	}
	if len(baseURLs) == 0 {
		return nil, fmt.Errorf("at least one base url is required")
	}

	keys := make([]appmodel.ChannelKey, 0, len(request.Keys))
	for _, key := range request.Keys {
		if strings.TrimSpace(key.ChannelKey) == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("at least one api key is required")
	}

	summary := &ChannelTestSummary{Results: make([]ChannelTestResult, 0, len(baseURLs)*len(keys))}
	for _, base := range baseURLs {
		baseURL := base.url
		for _, key := range keys {
			result := ChannelTestResult{
				BaseURL:   baseURL,
				KeyRemark: strings.TrimSpace(key.Remark),
				KeyMasked: maskSecret(key.ChannelKey),
			}
			startedAt := time.Now()
			statusCode, bodyText, testErr := performChannelTestRequest(ctx, client, request, baseURL, key.ChannelKey)
			result.LatencyMS = time.Since(startedAt).Milliseconds()
			result.StatusCode = statusCode
			result.ResponseBody = bodyText
			result.Passed = statusCode == http.StatusOK || statusCode == http.StatusTooManyRequests
			if testErr != nil {
				result.Message = testErr.Error()
			} else if result.Passed {
				result.Message = "ok"
			}

			// 连通性探测失败时，若渠道配置了可用的模型，回退到一次真实模型调用
			// 做综合判断：部分上游（如某些中转站）的 GET /models 端点行为异常，
			// 即使 key/网络正常也返回非 200，导致健康渠道误报失败（issue 反馈）。
			// 真实调用成功则视为渠道可用。SkipModelTest 渠道（低字节请求会扣费/
			// 封禁，issue #98）不发真实调用，手动测试入口同样要遵守该约定。
			if !result.Passed && !request.SkipModelTest && fallbackModelName(&request) != "" {
				fallbackStatus, fallbackBody, _, fallbackErr := performChannelModelFallback(ctx, &request, base, key.ChannelKey)
				if fallbackErr == nil {
					result.Passed = true
					result.Message = "ok (via model call)"
					result.ResponseBody = fallbackBody
					if fallbackStatus != 0 {
						result.StatusCode = fallbackStatus
					}
				}
			}

			if result.Passed {
				summary.Passed = true
			}
			summary.Results = append(summary.Results, result)
		}
	}

	return summary, nil
}

func performChannelTestRequest(ctx context.Context, client *http.Client, request appmodel.Channel, baseURL, apiKey string) (int, string, error) {
	if request.Type == 3 {
		return performGeminiConnectivityRequest(ctx, client, request, baseURL, apiKey)
	}
	return performOpenAICompatibleConnectivityRequest(ctx, client, request, baseURL, apiKey)
}

func performOpenAICompatibleConnectivityRequest(ctx context.Context, client *http.Client, request appmodel.Channel, baseURL, apiKey string) (int, string, error) {
	url := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	for _, header := range request.CustomHeader {
		if strings.TrimSpace(header.HeaderKey) != "" {
			req.Header.Set(header.HeaderKey, header.HeaderValue)
		}
	}
	return doChannelProbeRequest(client, req)
}

func performGeminiConnectivityRequest(ctx context.Context, client *http.Client, request appmodel.Channel, baseURL, apiKey string) (int, string, error) {
	url := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("X-Goog-Api-Key", apiKey)
	for _, header := range request.CustomHeader {
		if strings.TrimSpace(header.HeaderKey) != "" {
			req.Header.Set(header.HeaderKey, header.HeaderValue)
		}
	}
	return doChannelProbeRequest(client, req)
}

func doChannelProbeRequest(client *http.Client, req *http.Request) (int, string, error) {
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return resp.StatusCode, "", fmt.Errorf("read response body: %w", err)
	}
	bodyText := strings.TrimSpace(string(body))
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusTooManyRequests {
		return resp.StatusCode, bodyText, nil
	}
	if bodyText == "" {
		bodyText = resp.Status
	}
	return resp.StatusCode, bodyText, fmt.Errorf("upstream error: %d", resp.StatusCode)
}

// fallbackModelName 返回渠道配置中可用的一个模型名，用于连通性探测失败后的
// 真实模型调用回退。优先使用 Model，其次 CustomModel（二者可能以逗号分隔多个）。
func fallbackModelName(channel *appmodel.Channel) string {
	if channel == nil {
		return ""
	}
	for _, candidate := range []string{channel.Model, channel.CustomModel} {
		for _, part := range strings.Split(candidate, ",") {
			name := strings.TrimSpace(part)
			if name != "" {
				return name
			}
		}
	}
	return ""
}

// probeBaseURL 携带归一化后的探测地址与其原始 SuffixMode：回退调用会再次
// 归一化 base URL，丢失 SuffixMode 会让 custom/volcengine 等非默认模式的
// 地址被按默认规则二次拼接（如 …/api/custom → …/api/custom/v1）。
// 同一 SuffixMode 下归一化幂等，携带原模式可保证二次归一化为 no-op。
type probeBaseURL struct {
	url        string
	suffixMode string
}

// performChannelModelFallback 用渠道配置的模型发起一次真实 chat 探测请求，
// 用于连通性探测（GET /models）失败时的综合判断。经 outbound adapter 转发，
// 与分组/模型测试同源。返回 status、响应体与错误。
func performChannelModelFallback(ctx context.Context, channel *appmodel.Channel, base probeBaseURL, apiKey string) (int, string, *transmodel.InternalLLMResponse, error) {
	if channel == nil {
		return 0, "", nil, fmt.Errorf("channel is nil")
	}
	modelName := fallbackModelName(channel)
	if modelName == "" {
		return 0, "", nil, fmt.Errorf("no model configured for channel")
	}

	if outbound.Get(channel.Type) == nil {
		return 0, "", nil, fmt.Errorf("unsupported channel type: %d", channel.Type)
	}

	// 用渠道类型校验是否支持 chat 探测；不支持则跳过回退。
	if err := validateGroupProbeChannelType(appmodel.EndpointTypeAll, channel.Type); err != nil {
		return 0, "", nil, err
	}

	// 临时把 baseURL 替换为当前探测的 baseURL，保证回退请求打到同一地址。
	cloned := *channel
	cloned.BaseUrls = []appmodel.BaseUrl{{URL: base.url, SuffixMode: base.suffixMode}}
	cloned.Keys = []appmodel.ChannelKey{{ChannelKey: apiKey}}

	// 与 group probe 一致，对 OpenAI 类型渠道走 adapter 回退（issue #187）：
	// 先尝试 Chat Completions，失败再回退 Responses API。
	probeReqForResolve, _ := buildGroupProbeRequest(appmodel.EndpointTypeAll, modelName)
	adapterTypes := outbound.ResolveAttemptTypes(channel.Type, probeReqForResolve, "")
	var lastErr error
	for _, adapterType := range adapterTypes {
		adapter := outbound.Get(adapterType)
		if adapter == nil {
			continue
		}
		statusCode, responseText, internalResp, err := sendGroupProbeRequest(ctx, adapter, &cloned, apiKey, appmodel.EndpointTypeAll, modelName)
		if err == nil {
			return statusCode, responseText, internalResp, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return 0, "", nil, lastErr
	}
	return 0, "", nil, fmt.Errorf("no adapter available for channel type: %d", channel.Type)
}

func maskSecret(secret string) string {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return trimmed
	}
	return trimmed[:4] + "..." + trimmed[len(trimmed)-4:]
}
