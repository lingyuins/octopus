package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/lingyuins/octopus/internal/pkg/geminicli"
	"github.com/lingyuins/octopus/internal/transformer"
	"github.com/lingyuins/octopus/internal/transformer/model"
)

// Cloud Code Assist（cloudcode-pa.googleapis.com）是 Gemini CLI 的后端，
// 与官方 Generative Language API 在三处不兼容：
//
//	鉴权   ?key=<api_key>            ->  Authorization: Bearer <access_token>
//	路径   /v1beta/models/{m}:{op}   ->  /v1internal:{op}
//	报文   {contents:[...]}          ->  {model, project, request:{contents:[...]}}
//	          响应同构                ->  {response:{...}} 外层包装
//
// OAuth 账号只能走后者：cloud-platform scope 的 access_token 传给官方端点的
// ?key= 参数会被拒。因此这里按凭据形态分流，OAuth JSON 走 Code Assist，
// 裸 key 仍走官方路径，apikey 渠道行为不变。

// codeAssistRequest 是 Code Assist 的请求包装。
type codeAssistRequest struct {
	Model   string                              `json:"model"`
	Project string                              `json:"project,omitempty"`
	Request *model.GeminiGenerateContentRequest `json:"request"`
}

// codeAssistResponse 是 Code Assist 的响应包装。
// 用 RawMessage 承载内层，剥壳时可直接返回原始字节，
// 不经过结构体往返（避免丢弃未建模字段）。
type codeAssistResponse struct {
	Response json.RawMessage `json:"response"`
}

// transformCodeAssistRequest 构造 Cloud Code Assist 的出站请求。
func transformCodeAssistRequest(
	ctx context.Context,
	request *model.InternalLLMRequest,
	baseUrl string,
	cred geminicli.CodeAssistCredential,
) (*http.Request, error) {
	geminiReq := convertLLMToGeminiRequest(request)

	// Code Assist 在包装层携带模型名，内层不再重复。
	modelName := strings.TrimPrefix(strings.TrimSpace(request.Model), "models/")

	body, err := transformer.Marshal(&codeAssistRequest{
		Model:   modelName,
		Project: cred.ProjectID,
		Request: geminiReq,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal code assist request: %w", err)
	}

	isStream := request.Stream != nil && *request.Stream
	op := "generateContent"
	if isStream {
		op = "streamGenerateContent"
	}

	endpoint := strings.TrimSuffix(strings.TrimSpace(baseUrl), "/")
	if endpoint == "" || isOfficialGeminiEndpoint(endpoint) {
		// 渠道 base_url 指向官方端点（或留空）时改用 Code Assist 端点，
		// 避免把 OAuth 请求发到不认 Bearer 的官方域名。
		endpoint = geminicli.CodeAssistEndpoint
	}

	parsedUrl, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base url: %w", err)
	}
	// Code Assist 的操作挂在版本号上，不带 /models/{model} 段。
	parsedUrl.Path = fmt.Sprintf("%s/%s:%s",
		strings.TrimSuffix(parsedUrl.Path, "/"), geminicli.CodeAssistAPIVersion, op)

	if isStream {
		q := parsedUrl.Query()
		q.Set("alt", "sse")
		parsedUrl.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, parsedUrl.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// isOfficialGeminiEndpoint 判断是否为官方 Generative Language API 域名。
func isOfficialGeminiEndpoint(endpoint string) bool {
	lower := strings.ToLower(endpoint)
	return strings.Contains(lower, "generativelanguage.googleapis.com")
}

// unwrapCodeAssistPayload 剥掉 {"response":{...}} 外层包装。
//
// 未包装的报文原样返回，因此对官方格式同样安全。
func unwrapCodeAssistPayload(payload []byte) []byte {
	trimmed := bytes.TrimSpace(payload)
	if !bytes.HasPrefix(trimmed, []byte(`{"response"`)) && !bytes.Contains(trimmed, []byte(`"response"`)) {
		return payload
	}
	var wrapper codeAssistResponse
	if err := transformer.Unmarshal(trimmed, &wrapper); err != nil {
		return payload
	}
	inner := bytes.TrimSpace(wrapper.Response)
	if len(inner) == 0 || !bytes.HasPrefix(inner, []byte("{")) {
		return payload
	}
	return inner
}
