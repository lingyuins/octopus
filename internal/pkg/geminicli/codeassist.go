package geminicli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Cloud Code Assist（Gemini CLI 使用的后端）与官方 Generative Language API
// 是两套不同的接入方式：
//
//	generativelanguage.googleapis.com  ->  API key，?key= query 参数
//	cloudcode-pa.googleapis.com        ->  OAuth access_token，Authorization: Bearer
//
// OAuth 流程申请的 cloud-platform scope 只对后者有效，把 access_token 当作
// ?key= 发给官方端点会直接 401/403。
const (
	// CodeAssistEndpoint 是 Cloud Code Assist 的默认接入点。
	CodeAssistEndpoint = "https://cloudcode-pa.googleapis.com"

	// CodeAssistAPIVersion 与 Gemini CLI 保持一致。
	CodeAssistAPIVersion = "v1internal"

	// codeAssistHTTPTimeout 用于 project 发现等元数据请求。
	// 转发请求本身不走这里，由 relay 的客户端控制超时。
	codeAssistHTTPTimeout = 30 * time.Second
)

// OAuthTypeCodeAssist 标记该 OAuth 账号走 Cloud Code Assist 而非官方 API。
// 取值须与 model.OAuthTypeCodeAssist 一致（此处不 import model 以免依赖倒置）。
const OAuthTypeCodeAssist = "code_assist"

// CodeAssistCredential 是 gemini oauth 账号在出站时携带的凭据结构。
//
// 与 openai/codex 的做法一致：把出站需要的字段序列化成 JSON 作为 ChannelKey，
// 供出站适配器解析。裸 access_token 无法区分「官方 API key」与「OAuth token」，
// 也带不了 project_id。
type CodeAssistCredential struct {
	AccessToken string `json:"access_token"`
	ProjectID   string `json:"project_id,omitempty"`
	OAuthType   string `json:"oauth_type,omitempty"`
}

// IsCodeAssist 判断该凭据是否应走 Cloud Code Assist。
func (c CodeAssistCredential) IsCodeAssist() bool {
	return strings.TrimSpace(c.AccessToken) != "" && c.OAuthType == OAuthTypeCodeAssist
}

// MarshalCodeAssistCredential 构造出站凭据 JSON。
func MarshalCodeAssistCredential(accessToken, projectID string) string {
	b, _ := json.Marshal(CodeAssistCredential{
		AccessToken: strings.TrimSpace(accessToken),
		ProjectID:   strings.TrimSpace(projectID),
		OAuthType:   OAuthTypeCodeAssist,
	})
	return string(b)
}

// ParseCodeAssistCredential 解析出站凭据。
//
// 非 JSON（裸 token 或官方 API key）返回 ok=false，调用方应回退到官方 API 行为，
// 保证既有 apikey 渠道不受影响。
func ParseCodeAssistCredential(raw string) (CodeAssistCredential, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "{") {
		return CodeAssistCredential{}, false
	}
	var cred CodeAssistCredential
	if err := json.Unmarshal([]byte(raw), &cred); err != nil {
		return CodeAssistCredential{}, false
	}
	if !cred.IsCodeAssist() {
		return CodeAssistCredential{}, false
	}
	return cred, true
}

// loadCodeAssistResponse 是 :loadCodeAssist 的响应子集。
type loadCodeAssistResponse struct {
	CurrentTier *struct {
		ID string `json:"id"`
	} `json:"currentTier"`
	AllowedTiers []struct {
		ID         string `json:"id"`
		IsDefault  bool   `json:"isDefault"`
		UserDefine bool   `json:"userDefinedCloudaicompanionProject"`
	} `json:"allowedTiers"`
	CloudaicompanionProject string `json:"cloudaicompanionProject"`
}

// onboardUserResponse 是 :onboardUser 长操作的响应子集。
type onboardUserResponse struct {
	Done     bool `json:"done"`
	Response *struct {
		CloudaicompanionProject *struct {
			ID string `json:"id"`
		} `json:"cloudaicompanionProject"`
	} `json:"response"`
}

// DiscoverProject 解析该账号可用的 Cloud Code Assist project id。
//
// 流程与 Gemini CLI 一致：先 :loadCodeAssist 拿现有 project；没有则按默认 tier
// 调 :onboardUser 完成开通。免费 tier 由服务端分配 project，无需用户提供。
//
// 返回空字符串且 error 为 nil 表示服务端未分配 project（部分 tier 允许省略
// project 字段），此时出站请求不带 project 也能工作。
func DiscoverProject(ctx context.Context, httpClient *http.Client, accessToken, endpoint string) (string, error) {
	if strings.TrimSpace(accessToken) == "" {
		return "", fmt.Errorf("geminicli: access token is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: codeAssistHTTPTimeout}
	}
	base := strings.TrimSuffix(strings.TrimSpace(endpoint), "/")
	if base == "" {
		base = CodeAssistEndpoint
	}

	load, err := callCodeAssist[loadCodeAssistResponse](ctx, httpClient, accessToken, base, "loadCodeAssist", map[string]any{
		"metadata": map[string]string{
			"ideType":     "IDE_UNSPECIFIED",
			"platform":    "PLATFORM_UNSPECIFIED",
			"pluginType":  "GEMINI",
			"duetProject": "",
		},
	})
	if err != nil {
		return "", err
	}
	if project := strings.TrimSpace(load.CloudaicompanionProject); project != "" {
		return project, nil
	}

	// 未开通：按默认 tier 走 onboarding。
	tierID := ""
	for _, tier := range load.AllowedTiers {
		if tier.IsDefault {
			tierID = tier.ID
			break
		}
	}
	if tierID == "" && load.CurrentTier != nil {
		tierID = load.CurrentTier.ID
	}
	if tierID == "" {
		// 无可用 tier 信息时不阻断：让出站请求以无 project 方式尝试。
		return "", nil
	}

	onboard, err := callCodeAssist[onboardUserResponse](ctx, httpClient, accessToken, base, "onboardUser", map[string]any{
		"tierId":                  tierID,
		"cloudaicompanionProject": nil,
		"metadata": map[string]string{
			"ideType":    "IDE_UNSPECIFIED",
			"platform":   "PLATFORM_UNSPECIFIED",
			"pluginType": "GEMINI",
		},
	})
	if err != nil {
		return "", err
	}
	if onboard.Response != nil && onboard.Response.CloudaicompanionProject != nil {
		return strings.TrimSpace(onboard.Response.CloudaicompanionProject.ID), nil
	}
	return "", nil
}

// callCodeAssist 调用 Cloud Code Assist 的 :method 接口并解析 JSON 响应。
func callCodeAssist[T any](ctx context.Context, client *http.Client, accessToken, base, method string, payload map[string]any) (*T, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("geminicli: marshal %s payload: %w", method, err)
	}

	url := fmt.Sprintf("%s/%s:%s", base, CodeAssistAPIVersion, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("geminicli: build %s request: %w", method, err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geminicli: %s request failed: %w", method, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("geminicli: read %s response: %w", method, err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("geminicli: %s returned %d: %s", method, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var out T
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("geminicli: decode %s response: %w", method, err)
	}
	return &out, nil
}
