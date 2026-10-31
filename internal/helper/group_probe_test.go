package helper

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appmodel "github.com/lingyuins/octopus/internal/model"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

func TestBuildGroupProbeRequest_ConversationEndpointsUseMessages(t *testing.T) {
	for _, endpointType := range []string{
		appmodel.EndpointTypeAll,
		appmodel.EndpointTypeChat,
		appmodel.EndpointTypeDeepSeek,
		appmodel.EndpointTypeResponses,
		appmodel.EndpointTypeMessages,
	} {
		req, err := buildGroupProbeRequest(endpointType, "gpt-4o-mini")
		if err != nil {
			t.Fatalf("buildGroupProbeRequest(%q) error = %v", endpointType, err)
		}
		if req.EmbeddingInput != nil {
			t.Fatalf("buildGroupProbeRequest(%q) embedding_input = %#v, want nil", endpointType, req.EmbeddingInput)
		}
		if req.RawAPIFormat != transmodel.APIFormatOpenAIChatCompletion {
			t.Fatalf("buildGroupProbeRequest(%q) RawAPIFormat = %q, want %q", endpointType, req.RawAPIFormat, transmodel.APIFormatOpenAIChatCompletion)
		}
		if len(req.Messages) != 1 {
			t.Fatalf("buildGroupProbeRequest(%q) messages len = %d, want 1", endpointType, len(req.Messages))
		}
		if req.Messages[0].Content.Content == nil || *req.Messages[0].Content.Content != "hi" {
			t.Fatalf("buildGroupProbeRequest(%q) message content = %#v, want hi", endpointType, req.Messages[0].Content.Content)
		}
		if req.Stream == nil || *req.Stream {
			t.Fatalf("buildGroupProbeRequest(%q) stream = %#v, want false", endpointType, req.Stream)
		}
	}
}

func TestBuildGroupProbeRequest_EmbeddingsUseInput(t *testing.T) {
	req, err := buildGroupProbeRequest(appmodel.EndpointTypeEmbeddings, "text-embedding-3-small")
	if err != nil {
		t.Fatalf("buildGroupProbeRequest() error = %v", err)
	}
	if len(req.Messages) != 0 {
		t.Fatalf("buildGroupProbeRequest() messages len = %d, want 0", len(req.Messages))
	}
	if req.EmbeddingInput == nil || req.EmbeddingInput.Single == nil || *req.EmbeddingInput.Single != "hi" {
		t.Fatalf("buildGroupProbeRequest() embedding_input = %#v, want single hi", req.EmbeddingInput)
	}
}

func TestPrepareGroupProbeRequestReusesChannelAndGroupSettings(t *testing.T) {
	override := `{"max_tokens":32,"temperature":0.2}`
	request, err := buildGroupProbeRequest(appmodel.EndpointTypeChat, "gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	group := &appmodel.Group{EndpointType: appmodel.EndpointTypeChat, ThinkingMode: appmodel.ThinkingModeOn}
	channel := &appmodel.Channel{ParamOverride: &override}
	prepared, err := prepareGroupProbeRequest(channel, group, request)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.MaxTokens == nil || *prepared.MaxTokens != 32 {
		t.Fatalf("max_tokens = %#v, want 32", prepared.MaxTokens)
	}
	if prepared.Temperature == nil || *prepared.Temperature != 0.2 {
		t.Fatalf("temperature = %#v, want 0.2", prepared.Temperature)
	}
	if prepared.ThinkingMode != appmodel.ThinkingModeOn {
		t.Fatalf("thinking mode = %q, want on", prepared.ThinkingMode)
	}
	if prepared.TransformerMetadata[transmodel.TransformerMetadataGroupEndpointType] != appmodel.EndpointTypeChat {
		t.Fatalf("probe endpoint metadata = %#v", prepared.TransformerMetadata)
	}
}

func TestBuildGroupProbeRequest_RejectsUnsupportedEndpoint(t *testing.T) {
	if _, err := buildGroupProbeRequest(appmodel.EndpointTypeImageGeneration, "gpt-image-1"); err == nil {
		t.Fatal("buildGroupProbeRequest() error = nil, want unsupported endpoint error")
	}
}

func TestValidateGroupProbeChannelType_RejectsMismatchedEmbeddingChannel(t *testing.T) {
	err := validateGroupProbeChannelType(appmodel.EndpointTypeEmbeddings, outbound.OutboundTypeOpenAIChat)
	if err == nil {
		t.Fatal("validateGroupProbeChannelType() error = nil, want mismatch error")
	}
}

func TestValidateGroupProbeChannelType_AllAcceptsChatChannel(t *testing.T) {
	err := validateGroupProbeChannelType(appmodel.EndpointTypeAll, outbound.OutboundTypeOpenAIChat)
	if err != nil {
		t.Fatalf("validateGroupProbeChannelType() error = %v, want nil", err)
	}
}

func TestSendGroupProbeRequest_EmbeddingsUseEmbeddingPayload(t *testing.T) {
	type outboundRequest struct {
		Model string `json:"model"`
		Input string `json:"input"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("request path = %q, want /v1/embeddings", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("Authorization = %q, want Bearer sk-test", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		var got outboundRequest
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		if got.Input != "hi" {
			t.Fatalf("input = %q, want hi", got.Input)
		}
		if got.Model != "text-embedding-3-small" {
			t.Fatalf("model = %q, want text-embedding-3-small", got.Model)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"emb-1","object":"list","created":1,"model":"text-embedding-3-small","data":[{"object":"embedding","index":0,"embedding":[0.1]}]}`))
	}))
	defer server.Close()

	channel := &appmodel.Channel{
		Type:     outbound.OutboundTypeOpenAIEmbedding,
		BaseUrls: []appmodel.BaseUrl{{URL: server.URL}},
	}

	statusCode, responseText, internalResp, err := sendGroupProbeRequest(
		context.Background(),
		outbound.Get(outbound.OutboundTypeOpenAIEmbedding),
		channel,
		"sk-test",
		appmodel.EndpointTypeEmbeddings,
		"text-embedding-3-small",
	)
	if err != nil {
		t.Fatalf("sendGroupProbeRequest() error = %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("sendGroupProbeRequest() status = %d, want 200", statusCode)
	}
	if responseText == "" {
		t.Fatal("sendGroupProbeRequest() responseText = empty, want upstream payload")
	}
	if internalResp == nil {
		t.Fatal("sendGroupProbeRequest() internalResp = nil, want parsed response (issue #90)")
	}
}

// TestSendGroupProbeRequest_ChatResponseUsageParsed 验证 issue #90 的修复：
// chat 探测成功后，上游返回的 usage 必须被解析到 InternalLLMResponse.Usage，
// 使 recordTestLog 能回填 InputTokens/OutputTokens/Cost（不再显示为「未知」）。
func TestSendGroupProbeRequest_ChatResponseUsageParsed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// OpenAI chat completion 响应，含 usage。
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`))
	}))
	defer server.Close()

	channel := &appmodel.Channel{
		Type:     outbound.OutboundTypeOpenAIChat,
		BaseUrls: []appmodel.BaseUrl{{URL: server.URL}},
	}

	statusCode, _, internalResp, err := sendGroupProbeRequest(
		context.Background(),
		outbound.Get(outbound.OutboundTypeOpenAIChat),
		channel,
		"sk-test",
		appmodel.EndpointTypeChat,
		"gpt-4o-mini",
	)
	if err != nil {
		t.Fatalf("sendGroupProbeRequest() error = %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("sendGroupProbeRequest() status = %d, want 200", statusCode)
	}
	if internalResp == nil || internalResp.Usage == nil {
		t.Fatal("sendGroupProbeRequest() usage not parsed, want non-nil Usage (issue #90)")
	}
	if internalResp.Usage.PromptTokens != 5 {
		t.Fatalf("prompt_tokens = %d, want 5", internalResp.Usage.PromptTokens)
	}
	if internalResp.Usage.CompletionTokens != 2 {
		t.Fatalf("completion_tokens = %d, want 2", internalResp.Usage.CompletionTokens)
	}
}

func TestGetGroupModelTestProgress_RemovesExpiredEntry(t *testing.T) {
	restoreTTL := groupProbeProgressTTL
	groupProbeProgressTTL = time.Minute
	defer func() {
		groupProbeProgressTTL = restoreTTL
		clearGroupProbeProgressStore()
	}()
	clearGroupProbeProgressStore()

	storeGroupModelProgressAt(&GroupModelTestProgress{ID: "expired"}, time.Now().Add(-2*time.Minute))

	got, ok := GetGroupModelTestProgress("expired")
	if ok || got != nil {
		t.Fatalf("GetGroupModelTestProgress() = (%#v, %t), want (nil, false)", got, ok)
	}
	if _, stillExists := groupProbeProgress.Load("expired"); stillExists {
		t.Fatal("expired progress entry still exists after lookup cleanup")
	}
}

func TestStoreGroupModelProgress_CleansExpiredEntries(t *testing.T) {
	restoreTTL := groupProbeProgressTTL
	groupProbeProgressTTL = time.Minute
	defer func() {
		groupProbeProgressTTL = restoreTTL
		clearGroupProbeProgressStore()
	}()
	clearGroupProbeProgressStore()

	storeGroupModelProgressAt(&GroupModelTestProgress{ID: "expired"}, time.Now().Add(-2*time.Minute))
	storeGroupModelProgressAt(&GroupModelTestProgress{ID: "active", Total: 1}, time.Now())

	if _, stillExists := groupProbeProgress.Load("expired"); stillExists {
		t.Fatal("expired progress entry still exists after store cleanup")
	}
	got, ok := GetGroupModelTestProgress("active")
	if !ok || got == nil || got.ID != "active" {
		t.Fatalf("GetGroupModelTestProgress(active) = (%#v, %t), want active entry", got, ok)
	}
}

func clearGroupProbeProgressStore() {
	groupProbeProgress.Range(func(key, _ any) bool {
		groupProbeProgress.Delete(key)
		return true
	})
}
