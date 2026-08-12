package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/credential"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/lingyuins/octopus/internal/utils/xurl"
)

func init() {
	router.NewGroupRouter("/api/v1/verification").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermAPIKeysWrite)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/run", http.MethodPost).
				Handle(runVerification),
		).
		AddRoute(
			router.NewRoute("/run-for/:id", http.MethodPost).
				Handle(runVerificationForProfile),
		).
		AddRoute(
			router.NewRoute("/probes", http.MethodGet).
				Handle(listProbes),
		)
}

var availableProbes = []string{"text_gen", "models_list", "tool_calling", "structured_output"}

// verifyHTTPClient 是验证请求使用的 HTTP 客户端，带 30s 超时；
// 避免使用无超时的 http.DefaultClient 导致请求挂死。重定向逐跳做 SSRF 校验，
// 防止初始校验通过后 302 到内网地址。
var verifyHTTPClient = &http.Client{
	Timeout:       30 * time.Second,
	CheckRedirect: xurl.CheckRedirectSafe(5),
}

func listProbes(c *gin.Context) {
	resp.Success(c, availableProbes)
}

func runVerification(c *gin.Context) {
	var req model.VerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	// BaseURL 直接来自请求体，服务器会据此发起请求，必须做 SSRF 防护。
	if err := xurl.AssertSafeURL(req.BaseURL); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	probes := req.Probes
	if len(probes) == 0 {
		probes = []string{"models_list", "text_gen"}
	}

	results := executeProbes(c.Request.Context(), req.BaseURL, req.APIKey, req.APIType, req.Model, probes)
	resp.Success(c, results)
}

func runVerificationForProfile(c *gin.Context) {
	id, err := parseIntParam(c, "id")
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}

	p, err := credential.GetDecrypted(c.Request.Context(), id)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// 凭据档案的 BaseURL 可能由低权限角色配置，探测前必须做 SSRF 校验
	// （临时 URL 探测入口已校验，此路径此前遗漏）。
	if err := xurl.AssertSafeURL(p.BaseURL); err != nil {
		resp.Error(c, http.StatusBadRequest, "unsafe base_url: "+err.Error())
		return
	}

	var req struct {
		Model  string   `json:"model"`
		Probes []string `json:"probes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}

	probes := req.Probes
	if len(probes) == 0 {
		probes = []string{"models_list", "text_gen"}
	}

	results := executeProbes(c.Request.Context(), p.BaseURL, p.APIKey, p.APIType, req.Model, probes)

	allOK := true
	for _, r := range results {
		if !r.Success {
			allOK = false
			break
		}
	}
	now := time.Now()
	status := model.HealthStatusHealthy
	if !allOK {
		status = model.HealthStatusError
	}
	if err := credential.UpdateHealth(c.Request.Context(), id, status, now); err != nil {
		log.Warnf("failed to update credential %d health status: %v", id, err)
	}

	resp.Success(c, results)
}

func executeProbes(ctx context.Context, baseURL, apiKey, apiType, modelName string, probes []string) []model.VerificationResult {
	baseURL = strings.TrimRight(baseURL, "/")
	if modelName == "" {
		modelName = "gpt-4o-mini"
	}

	var results []model.VerificationResult
	for _, probe := range probes {
		var r model.VerificationResult
		r.Probe = probe

		start := time.Now()
		switch probe {
		case "models_list":
			r = probeModelsList(ctx, baseURL, apiKey)
		case "text_gen":
			r = probeTextGen(ctx, baseURL, apiKey, apiType, modelName)
		case "tool_calling":
			r = probeToolCalling(ctx, baseURL, apiKey, apiType, modelName)
		case "structured_output":
			r = probeStructuredOutput(ctx, baseURL, apiKey, apiType, modelName)
		default:
			r.Error = "unknown probe"
		}
		r.Probe = probe
		r.Latency = time.Since(start).Milliseconds()
		results = append(results, r)
	}
	return results
}

func probeModelsList(ctx context.Context, baseURL, apiKey string) model.VerificationResult {
	body, err := doVerifyRequest(ctx, http.MethodGet, baseURL+"/v1/models", apiKey, nil)
	if err != nil {
		return model.VerificationResult{Success: false, Error: err.Error()}
	}
	return model.VerificationResult{Success: true, Output: truncateOutput(body, 200)}
}

func probeTextGen(ctx context.Context, baseURL, apiKey, apiType, modelName string) model.VerificationResult {
	payload := map[string]interface{}{
		"model":      modelName,
		"max_tokens": 10,
		"messages": []map[string]string{
			{"role": "user", "content": "Say hi"},
		},
	}

	endpoint := baseURL + "/v1/chat/completions"
	if apiType == model.APITypeAnthropic {
		endpoint = baseURL + "/v1/messages"
	}

	body, err := doVerifyRequest(ctx, http.MethodPost, endpoint, apiKey, payload)
	if err != nil {
		return model.VerificationResult{Success: false, Error: err.Error()}
	}
	return model.VerificationResult{Success: true, Output: truncateOutput(body, 300)}
}

func probeToolCalling(ctx context.Context, baseURL, apiKey, apiType, modelName string) model.VerificationResult {
	payload := map[string]interface{}{
		"model":      modelName,
		"max_tokens": 50,
		"messages": []map[string]string{
			{"role": "user", "content": "What is 2+2? Use the calculator tool."},
		},
		"tools": []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "calculator",
					"description": "Evaluate a math expression",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"expression": map[string]string{"type": "string", "description": "math expression"},
						},
						"required": []string{"expression"},
					},
				},
			},
		},
	}

	endpoint := baseURL + "/v1/chat/completions"
	if apiType == model.APITypeAnthropic {
		endpoint = baseURL + "/v1/messages"
	}

	body, err := doVerifyRequest(ctx, http.MethodPost, endpoint, apiKey, payload)
	if err != nil {
		return model.VerificationResult{Success: false, Error: err.Error()}
	}
	return model.VerificationResult{Success: true, Output: truncateOutput(body, 300)}
}

func probeStructuredOutput(ctx context.Context, baseURL, apiKey, apiType, modelName string) model.VerificationResult {
	payload := map[string]interface{}{
		"model":      modelName,
		"max_tokens": 50,
		"messages": []map[string]string{
			{"role": "user", "content": "Return JSON: {\"greeting\": \"hello\"}"},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	endpoint := baseURL + "/v1/chat/completions"
	if apiType == model.APITypeAnthropic {
		endpoint = baseURL + "/v1/messages"
	}

	body, err := doVerifyRequest(ctx, http.MethodPost, endpoint, apiKey, payload)
	if err != nil {
		return model.VerificationResult{Success: false, Error: err.Error()}
	}
	return model.VerificationResult{Success: true, Output: truncateOutput(body, 300)}
}

func doVerifyRequest(ctx context.Context, method, url, apiKey string, payload interface{}) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := verifyHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateOutput(string(respBody), 200))
	}

	return string(respBody), nil
}

func parseIntParam(c *gin.Context, name string) (int, error) {
	return strconv.Atoi(c.Param(name))
}

func truncateOutput(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
