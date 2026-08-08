package helper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	appmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

// TestFallbackModelName_PrefersModelOverCustomModel 验证回退模型名选择：
// 优先 Model，其次 CustomModel，并按逗号拆分取第一个非空项。
func TestFallbackModelName_PrefersModelOverCustomModel(t *testing.T) {
	cases := []struct {
		name    string
		channel *appmodel.Channel
		want    string
	}{
		{"nil channel", nil, ""},
		{"no models", &appmodel.Channel{}, ""},
		{"model single", &appmodel.Channel{Model: "gpt-4o"}, "gpt-4o"},
		{"custom single", &appmodel.Channel{CustomModel: "deepseek-v4"}, "deepseek-v4"},
		{"model wins over custom", &appmodel.Channel{Model: "a,b", CustomModel: "c"}, "a"},
		{"custom fallback", &appmodel.Channel{Model: "  ", CustomModel: " x,y "}, "x"},
		{"whitespace only", &appmodel.Channel{Model: " , "}, ""},
	}
	for _, tc := range cases {
		got := fallbackModelName(tc.channel)
		if got != tc.want {
			t.Fatalf("fallbackModelName(%s) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestTestChannel_FallsBackToModelCallOnConnectivityFailure 验证连通性探测
// （GET /models 返回 400 等非 200）失败时，回退到真实模型调用后判定为通过，
// 避免上游 /models 端点异常导致健康渠道误报失败。
func TestTestChannel_FallsBackToModelCallOnConnectivityFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			// 模拟上游 /models 端点行为异常：把该请求当成模型调用解析并返回 400。
			http.Error(w, `{"error":{"message":"Unsupported model: 'deepseek-v4-flash-0731'."}}`, http.StatusBadRequest)
		case "/v1/chat/completions":
			// 真实模型调用正常。
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"deepseek-v4-flash-0731","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	summary, err := TestChannel(context.Background(), appmodel.Channel{
		Type:        outbound.OutboundTypeOpenAIChat,
		BaseUrls:    []appmodel.BaseUrl{{URL: server.URL}},
		Keys:        []appmodel.ChannelKey{{ChannelKey: "sk-test"}},
		Model:       "deepseek-v4-flash-0731",
		CustomModel: "",
	})
	if err != nil {
		t.Fatalf("TestChannel() error = %v", err)
	}
	if !summary.Passed {
		t.Fatalf("TestChannel() passed = false, want true (连通性失败但模型调用成功应回退为通过)")
	}
	if len(summary.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(summary.Results))
	}
	r := summary.Results[0]
	if !r.Passed {
		t.Fatalf("result passed = false, want true; message=%q", r.Message)
	}
	if r.Message != "ok (via model call)" {
		t.Fatalf("result message = %q, want 'ok (via model call)'", r.Message)
	}
}

// TestTestChannel_NoModelNoFallbackStaysFailed 验证渠道未配置模型时，
// 连通性探测失败保持失败（不回退，避免误判）。
func TestTestChannel_NoModelNoFallbackStaysFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadRequest)
	}))
	defer server.Close()

	summary, err := TestChannel(context.Background(), appmodel.Channel{
		Type:     outbound.OutboundTypeOpenAIChat,
		BaseUrls: []appmodel.BaseUrl{{URL: server.URL}},
		Keys:     []appmodel.ChannelKey{{ChannelKey: "sk-test"}},
	})
	if err != nil {
		t.Fatalf("TestChannel() error = %v", err)
	}
	if summary.Passed {
		t.Fatal("TestChannel() passed = true, want false (no model fallback should stay failed)")
	}
	if len(summary.Results) != 1 || summary.Results[0].Passed {
		t.Fatalf("results = %#v, want single failed result", summary.Results)
	}
}

// TestTestChannel_ConnectivitySuccessNoFallback 验证连通性探测成功时
// 不触发模型回退（保持原有 passed 语义）。
func TestTestChannel_ConnectivitySuccessNoFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer server.Close()

	summary, err := TestChannel(context.Background(), appmodel.Channel{
		Type:     outbound.OutboundTypeOpenAIChat,
		BaseUrls: []appmodel.BaseUrl{{URL: server.URL}},
		Keys:     []appmodel.ChannelKey{{ChannelKey: "sk-test"}},
		Model:    "gpt-4o",
	})
	if err != nil {
		t.Fatalf("TestChannel() error = %v", err)
	}
	if !summary.Passed {
		t.Fatal("TestChannel() passed = false, want true")
	}
	if len(summary.Results) != 1 || !summary.Results[0].Passed {
		t.Fatalf("results = %#v, want single passed result", summary.Results)
	}
	if summary.Results[0].Message != "ok" {
		t.Fatalf("result message = %q, want 'ok'", summary.Results[0].Message)
	}
}

// TestPerformChannelModelFallback_OpenAIResponseFallsBackToChat 验证 issue #187：
// channel type=OpenAIResponse(1) 但上游只支持 /v1/chat/completions 时，
// performChannelModelFallback 通过 ResolveAttemptTypes 回退到 OpenAIChat adapter 成功。
func TestPerformChannelModelFallback_OpenAIResponseFallsBackToChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
		case "/v1/responses":
			http.Error(w, `{"error":{"message":"not found"}}`, http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	channel := &appmodel.Channel{
		Type:     outbound.OutboundTypeOpenAIResponse,
		BaseUrls: []appmodel.BaseUrl{{URL: server.URL}},
		Keys:     []appmodel.ChannelKey{{ChannelKey: "sk-test"}},
		Model:    "gpt-4o-mini",
	}

	statusCode, _, _, err := performChannelModelFallback(context.Background(), channel, probeBaseURL{url: server.URL}, "sk-test")
	if err != nil {
		t.Fatalf("performChannelModelFallback() error = %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("performChannelModelFallback() status = %d, want 200", statusCode)
	}
}
