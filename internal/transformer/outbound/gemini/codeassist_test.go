package gemini

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/pkg/geminicli"
	"github.com/lingyuins/octopus/internal/transformer/model"
)

func codeAssistKey(t *testing.T, accessToken, projectID string) string {
	t.Helper()
	return geminicli.MarshalCodeAssistCredential(accessToken, projectID)
}

func newLLMRequest(modelName string, stream bool) *model.InternalLLMRequest {
	text := "hi"
	return &model.InternalLLMRequest{
		Model:  modelName,
		Stream: &stream,
		Messages: []model.Message{
			{Role: "user", Content: model.MessageContent{Content: &text}},
		},
	}
}

// TransformRequest 拿到 OAuth 凭据时必须切到 Cloud Code Assist：
// Bearer 鉴权、v1internal 路径、请求体带 model/project 包装，且不带 ?key=。
func TestTransformRequest_CodeAssistCredential(t *testing.T) {
	out := &MessagesOutbound{}
	key := codeAssistKey(t, "ya29.token", "proj-123")

	req, err := out.TransformRequest(context.Background(), newLLMRequest("gemini-2.5-pro", false), "", key)
	if err != nil {
		t.Fatalf("TransformRequest: %v", err)
	}

	if got := req.Header.Get("Authorization"); got != "Bearer ya29.token" {
		t.Fatalf("Authorization = %q, want Bearer ya29.token", got)
	}
	if req.URL.Query().Get("key") != "" {
		t.Fatalf("OAuth 请求不应带 ?key=，got %q", req.URL.String())
	}
	if req.URL.Host != "cloudcode-pa.googleapis.com" {
		t.Fatalf("host = %q, want cloudcode-pa.googleapis.com", req.URL.Host)
	}
	if req.URL.Path != "/v1internal:generateContent" {
		t.Fatalf("path = %q, want /v1internal:generateContent", req.URL.Path)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var wrapped struct {
		Model   string          `json:"model"`
		Project string          `json:"project"`
		Request json.RawMessage `json:"request"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if wrapped.Model != "gemini-2.5-pro" {
		t.Fatalf("model = %q, want gemini-2.5-pro", wrapped.Model)
	}
	if wrapped.Project != "proj-123" {
		t.Fatalf("project = %q, want proj-123", wrapped.Project)
	}
	if len(wrapped.Request) == 0 || !strings.Contains(string(wrapped.Request), "contents") {
		t.Fatalf("request 包装缺少 contents: %s", string(wrapped.Request))
	}
}

// 流式请求走 streamGenerateContent 且带 alt=sse。
func TestTransformRequest_CodeAssistStream(t *testing.T) {
	out := &MessagesOutbound{}
	req, err := out.TransformRequest(context.Background(),
		newLLMRequest("gemini-2.5-flash", true), "", codeAssistKey(t, "tok", "p1"))
	if err != nil {
		t.Fatalf("TransformRequest: %v", err)
	}
	if req.URL.Path != "/v1internal:streamGenerateContent" {
		t.Fatalf("path = %q, want /v1internal:streamGenerateContent", req.URL.Path)
	}
	if req.URL.Query().Get("alt") != "sse" {
		t.Fatalf("alt = %q, want sse", req.URL.Query().Get("alt"))
	}
}

// 渠道 base_url 指向官方端点时也必须改用 Code Assist 端点，
// 否则 OAuth token 会被发到不认 Bearer 的官方域名。
func TestTransformRequest_CodeAssistOverridesOfficialBaseURL(t *testing.T) {
	out := &MessagesOutbound{}
	req, err := out.TransformRequest(context.Background(),
		newLLMRequest("gemini-2.5-pro", false),
		"https://generativelanguage.googleapis.com/v1beta",
		codeAssistKey(t, "tok", ""))
	if err != nil {
		t.Fatalf("TransformRequest: %v", err)
	}
	if req.URL.Host != "cloudcode-pa.googleapis.com" {
		t.Fatalf("host = %q, want cloudcode-pa.googleapis.com", req.URL.Host)
	}
}

// 自建代理（非官方域名）应当被保留，只替换路径与鉴权方式。
func TestTransformRequest_CodeAssistKeepsCustomBaseURL(t *testing.T) {
	out := &MessagesOutbound{}
	req, err := out.TransformRequest(context.Background(),
		newLLMRequest("gemini-2.5-pro", false),
		"https://relay.example.com/gemini",
		codeAssistKey(t, "tok", ""))
	if err != nil {
		t.Fatalf("TransformRequest: %v", err)
	}
	if req.URL.Host != "relay.example.com" {
		t.Fatalf("host = %q, want relay.example.com", req.URL.Host)
	}
	if req.URL.Path != "/gemini/v1internal:generateContent" {
		t.Fatalf("path = %q, want /gemini/v1internal:generateContent", req.URL.Path)
	}
}

// 裸 API key（apikey 渠道）行为必须保持不变：?key= + 官方路径 + 无 Authorization。
func TestTransformRequest_PlainAPIKeyUnchanged(t *testing.T) {
	out := &MessagesOutbound{}
	req, err := out.TransformRequest(context.Background(),
		newLLMRequest("gemini-2.5-pro", false),
		"https://generativelanguage.googleapis.com/v1beta",
		"AIzaSyPlainKey")
	if err != nil {
		t.Fatalf("TransformRequest: %v", err)
	}
	if got := req.URL.Query().Get("key"); got != "AIzaSyPlainKey" {
		t.Fatalf("key = %q, want AIzaSyPlainKey", got)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("apikey 渠道不应带 Authorization，got %q", got)
	}
	if req.URL.Path != "/v1beta/models/gemini-2.5-pro:generateContent" {
		t.Fatalf("path = %q", req.URL.Path)
	}
}

func TestUnwrapCodeAssistPayload(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "wrapped",
			in:   `{"response":{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}}`,
			want: `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}`,
		},
		{
			// 官方格式没有 response 包装，必须原样返回。
			name: "official passthrough",
			in:   `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}`,
			want: `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}`,
		},
		{
			// 业务字段恰好叫 response 但不是包装层，也不能误剥。
			name: "non-object response field",
			in:   `{"response":"text"}`,
			want: `{"response":"text"}`,
		},
		{
			name: "invalid json",
			in:   `not-json`,
			want: `not-json`,
		},
		{
			name: "empty",
			in:   ``,
			want: ``,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.TrimSpace(string(unwrapCodeAssistPayload([]byte(tc.in))))
			if got != tc.want {
				t.Fatalf("unwrap = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestIsOfficialGeminiEndpoint(t *testing.T) {
	cases := map[string]bool{
		"https://generativelanguage.googleapis.com/v1beta": true,
		"https://GenerativeLanguage.googleapis.com":        true,
		"https://cloudcode-pa.googleapis.com":              false,
		"https://relay.example.com/gemini":                 false,
	}
	for endpoint, want := range cases {
		if got := isOfficialGeminiEndpoint(endpoint); got != want {
			t.Fatalf("isOfficialGeminiEndpoint(%q) = %v, want %v", endpoint, got, want)
		}
	}
}
