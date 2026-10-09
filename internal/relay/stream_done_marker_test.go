package relay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	dbmodel "github.com/lingyuins/octopus/internal/model"
	tmodel "github.com/lingyuins/octopus/internal/transformer/model"
)

// sessionSeq 是会话 key 隔离用的进程内计数器（见 attachStreamSession）。
var sessionSeq atomic.Uint64

// stubOutboundAdapter 最小出站适配器：把上游 SSE data 解析成内部流式 chunk。
// [DONE] 映射为 Object="[DONE]"，与真实 openai/anthropic 出站适配器一致。
type stubOutboundAdapter struct{}

func (stubOutboundAdapter) TransformRequest(context.Context, *tmodel.InternalLLMRequest, string, string) (*http.Request, error) {
	return nil, errors.New("not used")
}

func (stubOutboundAdapter) TransformResponse(context.Context, *http.Response) (*tmodel.InternalLLMResponse, error) {
	return nil, errors.New("not used")
}

func (stubOutboundAdapter) TransformStream(_ context.Context, eventData []byte) (*tmodel.InternalLLMResponse, error) {
	raw := strings.TrimSpace(string(eventData))
	if raw == "[DONE]" {
		return &tmodel.InternalLLMResponse{Object: "[DONE]"}, nil
	}
	// 其余一律视为一个带可见内容的增量 chunk（hasVisibleContent = true）。
	text := raw
	return &tmodel.InternalLLMResponse{
		Choices: []tmodel.Choice{{
			Index: 0,
			Delta: &tmodel.Message{Content: tmodel.MessageContent{Content: &text}},
		}},
	}, nil
}

// stubInboundAdapter 最小入站适配器：把内部 chunk 渲染回客户端 SSE 帧。
// [DONE] 渲染成 `data: [DONE]\n\n`，与 inbound/openai 一致。
type stubInboundAdapter struct{}

func (stubInboundAdapter) TransformRequest(context.Context, []byte) (*tmodel.InternalLLMRequest, error) {
	return nil, errors.New("not used")
}

func (stubInboundAdapter) TransformResponse(context.Context, *tmodel.InternalLLMResponse) ([]byte, error) {
	return nil, errors.New("not used")
}

func (stubInboundAdapter) TransformStream(_ context.Context, stream *tmodel.InternalLLMResponse) ([]byte, error) {
	if stream == nil {
		return nil, nil
	}
	if stream.Object == "[DONE]" {
		return []byte("data: [DONE]\n\n"), nil
	}
	var sb strings.Builder
	for _, choice := range stream.Choices {
		if choice.Delta != nil && choice.Delta.Content.Content != nil {
			sb.WriteString(*choice.Delta.Content.Content)
		}
	}
	return []byte("data: " + sb.String() + "\n\n"), nil
}

func (stubInboundAdapter) GetInternalResponse(context.Context) (*tmodel.InternalLLMResponse, error) {
	return nil, errors.New("not used")
}

// newStreamTestAttempt 构造一个可直接调用 handleStreamResponse 的最小 relayAttempt。
// streamSession 为 nil：走无会话的直写路径，避免触碰全局 session store。
func newStreamTestAttempt(t *testing.T, clientCtx context.Context) (*relayAttempt, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(clientCtx)

	stream := true
	internalReq := &tmodel.InternalLLMRequest{Stream: &stream, Model: "stub-model"}

	ra := &relayAttempt{
		relayRequest: &relayRequest{
			c:               c,
			clientCtx:       clientCtx,
			operationCtx:    context.Background(),
			inAdapter:       stubInboundAdapter{},
			internalRequest: internalReq,
			metrics:         NewRelayMetrics(1, "stub-model", dbmodel.EndpointTypeChat, dbmodel.EndpointTypeChat, "127.0.0.1", internalReq),
		},
		outAdapter: stubOutboundAdapter{},
		channel:    &dbmodel.Channel{ID: 1, Name: "stub-channel"},
		usedKey:    dbmodel.ChannelKey{ID: 1},
	}
	return ra, recorder
}

// attachStreamSession 给 attempt 挂上一个 stream session。
//
// 空输出重试（issue #106/#155）在 handleStreamResponse 的收尾分支里嵌在
// `if ra.streamSession != nil` 内部：无会话的直写路径本来就不做空输出重试。
// 这是本次修改前就存在的既有行为，finalizeStream 原样保留了它，因此验证
// [DONE] 不破坏 #155 语义必须走带会话的路径。
//
// 会话挂在**本测试私有的** store 上，不进包级全局 relayStreamSessions。
//
// 为何不用 acquireRelayStreamSession：它会把会话注册到全局 store，而
// session.store 存的就是包级 relayStreamSessions 的地址。本包的
// stream_session_test.go 会整体重写这个全局变量（把其中的 sync.RWMutex 归零），
// 而 Finish / enforceSessionLimitLocked 的异步驱逐 goroutine 又会去锁同一个
// s.store.mu；两者重叠时会出现 "sync: Unlock of unlocked RWMutex" 致命错误。
// 该竞态是仓库既有问题（把本文件的测试全部 -run 过滤掉仍可复现），但本测试
// 不该往全局 store 里多加会话去放大它，所以改用私有 store 完全隔离。
//
// 构造方式与 acquireRelayStreamSession 的会话初始化保持一致，以保证
// handleStreamResponse 里用到的 AddPayload / Finish / HasSubscribers 语义不变。
func attachStreamSession(t *testing.T, ra *relayAttempt) *relayStreamSession {
	t.Helper()

	store := &relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}

	seq := sessionSeq.Add(1)
	conversationID := fmt.Sprintf("done-marker-%s-%d", t.Name(), seq)
	key := fmt.Sprintf("%s:%d", conversationID, seq)
	now := time.Now()

	session := &relayStreamSession{
		store:             store,
		key:               key,
		conversationID:    conversationID,
		conversationScope: conversationID,
		requestHash:       seq,
		createdAt:         now,
		updatedAt:         now,
		subscribers:       make(map[chan struct{}]struct{}),
	}
	store.byKey[key] = session
	store.activeByConversation[conversationID] = key

	// Finish 幂等（内部有 done 判定），且 handleStreamResponse 的安全网 defer
	// 可能已经 Finish 过；这里确保会话不以活跃态残留。
	t.Cleanup(func() { session.Finish(nil) })

	ra.streamSession = session
	return session
}

// runStreamWithUpstream 用 io.Pipe 驱动 handleStreamResponse：upstream 由 write 函数
// 写入 SSE 帧。write 返回后 pipe 保持打开（不发 EOF），精确复刻「上游发完 [DONE]
// 但不关闭连接」的现场。测试结束由 t.Cleanup 关闭 writer 释放 reader goroutine。
func runStreamWithUpstream(t *testing.T, ra *relayAttempt, write func(w *io.PipeWriter)) <-chan error {
	t.Helper()
	reader, writer := io.Pipe()
	t.Cleanup(func() { _ = writer.Close() })

	go write(writer)

	done := make(chan error, 1)
	go func() {
		done <- ra.handleStreamResponse(context.Background(), &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       reader,
		})
	}()
	return done
}

// 缺陷5 主回归：上游发完 `data: [DONE]` 后保持连接不关闭。
//
// 未修复时 handleStreamResponse 的 SSE reader 只在上游 EOF/关闭时结束循环，
// 于是会一直阻塞等 EOF，直到客户端或中间层先超时断开 → 记 client disconnected
// → 计入熔断器（叠加缺陷1）→ 健康渠道被误熔断。本测试在未修复代码上会阻塞到超时。
func TestHandleStreamResponseReturnsAfterDoneMarkerWithoutEOF(t *testing.T) {
	clientCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ra, recorder := newStreamTestAttempt(t, clientCtx)

	done := runStreamWithUpstream(t, ra, func(w *io.PipeWriter) {
		_, _ = w.Write([]byte("data: hello\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		// 故意不关闭 writer：上游保持连接，永不产生 EOF。
		<-time.After(30 * time.Second)
	})

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handleStreamResponse() = %v, want nil after [DONE]", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("handleStreamResponse() blocked waiting for EOF after [DONE]; " +
			"upstreams that keep the connection open would hang until the client times out")
	}

	// [DONE] 必须已经送达客户端：入站适配器渲染的终止帧不能被早退吞掉。
	body := recorder.Body.String()
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("client did not receive the [DONE] terminator frame; body = %q", body)
	}
	if !strings.Contains(body, "hello") {
		t.Fatalf("visible content was dropped; body = %q", body)
	}
}

// [DONE] 早退不得破坏 issue #155 的空输出重试语义：
// 上游只发 reasoning（无可见内容）就以 [DONE] 终止时，仍必须返回 errEmptyOutput，
// 而不是因为「收到 [DONE]」就无条件记成功。
func TestHandleStreamResponseDoneMarkerStillRetriesEmptyOutput(t *testing.T) {
	clientCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ra, _ := newStreamTestAttempt(t, clientCtx)
	ra.group = &dbmodel.Group{ReasoningBufferStrategy: "buffer"}
	// 空输出重试分支嵌在 `if ra.streamSession != nil` 内部，必须带会话才能命中。
	attachStreamSession(t, ra)

	// 出站适配器改写为「只产出 reasoning、无可见内容」的 chunk。
	ra.outAdapter = reasoningOnlyOutbound{}

	if !isRetryEmptyOutputEnabled() {
		t.Skip("retry_empty_output 未启用（setting 缓存不可用时默认为启用）；空输出重试路径不适用")
	}

	done := runStreamWithUpstream(t, ra, func(w *io.PipeWriter) {
		_, _ = w.Write([]byte("data: thinking\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		<-time.After(30 * time.Second)
	})

	select {
	case err := <-done:
		if !errors.Is(err, errEmptyOutput) {
			t.Fatalf("handleStreamResponse() = %v, want errEmptyOutput; "+
				"[DONE] must not turn an empty stream into a success (issue #155)", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("handleStreamResponse() blocked after [DONE] on a reasoning-only stream")
	}
}

// 上游正常关闭（EOF）的既有路径必须继续可用——finalizeStream 被 EOF 与 [DONE]
// 共用，这里锁住 EOF 侧不被重构破坏。
func TestHandleStreamResponseStillFinalizesOnEOF(t *testing.T) {
	clientCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ra, recorder := newStreamTestAttempt(t, clientCtx)

	done := runStreamWithUpstream(t, ra, func(w *io.PipeWriter) {
		_, _ = w.Write([]byte("data: hello\n\n"))
		_ = w.Close() // 正常 EOF
	})

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handleStreamResponse() = %v, want nil on clean EOF", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("handleStreamResponse() blocked on a clean EOF")
	}
	if !strings.Contains(recorder.Body.String(), "hello") {
		t.Fatalf("visible content was dropped on EOF path; body = %q", recorder.Body.String())
	}
}

// reasoningOnlyOutbound 产出只含 reasoning_content 的 chunk（无可见内容）。
type reasoningOnlyOutbound struct{ stubOutboundAdapter }

func (reasoningOnlyOutbound) TransformStream(_ context.Context, eventData []byte) (*tmodel.InternalLLMResponse, error) {
	raw := strings.TrimSpace(string(eventData))
	if raw == "[DONE]" {
		return &tmodel.InternalLLMResponse{Object: "[DONE]"}, nil
	}
	reasoning := raw
	return &tmodel.InternalLLMResponse{
		Choices: []tmodel.Choice{{
			Index: 0,
			Delta: &tmodel.Message{ReasoningContent: &reasoning},
		}},
	}, nil
}
