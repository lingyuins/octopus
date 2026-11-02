package nodetransport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type modelsResponseFixture struct {
	manager *Manager
	peer    *websocket.Conn
	ctx     context.Context
}

func setupModelsResponse(t *testing.T) *modelsResponseFixture {
	t.Helper()
	manager := NewManager()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = manager.Accept(r.Context(), 1, w, r, AcceptOptions{MaxConcurrency: 4, Validate: func() bool { return true }})
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { _ = manager.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	peer, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), &websocket.DialOptions{Subprotocols: []string{protocol}})
	if err != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = peer.CloseNow() })
	peer.SetReadLimit(maxChunk + 9)
	f := &modelsResponseFixture{manager: manager, peer: peer, ctx: ctx}
	h, err := jsonFrame(frameHello, 0, hello{Version: 1, MaxConcurrency: 4})
	if err != nil {
		t.Fatal(err)
	}
	f.send(t, h)
	eventually(t, func() bool { return manager.Snapshot(1).Online })
	return f
}

func (f *modelsResponseFixture) send(t *testing.T, message frame) {
	t.Helper()
	if err := f.peer.Write(f.ctx, websocket.MessageBinary, message.encode()); err != nil {
		t.Fatal(err)
	}
}

func (f *modelsResponseFixture) start(t *testing.T, ctx context.Context, method, path string) (*http.Response, uint64) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, method, BaseURL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan struct {
		response *http.Response
		err      error
	}, 1)
	go func() {
		response, err := f.manager.Client(1, 3*time.Second).Do(req)
		result <- struct {
			response *http.Response
			err      error
		}{response, err}
	}()
	var id uint64
	readCtx := f.ctx
	if deadline, ok := f.ctx.Deadline(); ok {
		// 需要多轮同步的测试用更长的读取窗口，避免 fixture 默认 5s ctx 在断言前到期。
		if time.Until(deadline) < 30*time.Second {
			var cancel context.CancelFunc
			readCtx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
			t.Cleanup(cancel)
		}
	}
	for {
		message, err := readFrame(readCtx, f.peer)
		if err != nil {
			t.Fatal(err)
		}
		if message.kind == frameRequest {
			id = message.id
		}
		if message.kind == frameRequestEnd && message.id == id && id != 0 {
			break
		}
	}
	headers, err := jsonFrame(frameResponse, id, responseMeta{Status: http.StatusOK})
	if err != nil {
		t.Fatal(err)
	}
	f.send(t, headers)
	select {
	case got := <-result:
		if got.err != nil {
			t.Fatal(got.err)
		}
		t.Cleanup(func() { _ = got.response.Body.Close() })
		return got.response, id
	case <-f.ctx.Done():
		t.Fatal("response headers did not arrive")
		return nil, 0
	}
}

func TestModelsResponseBuffersByBytesUntilComplete(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	for _, chunkSize := range []int{1, 17, maxChunk} {
		t.Run(strconv.Itoa(chunkSize), func(t *testing.T) {
			f := setupModelsResponse(t)
			response, id := f.start(t, f.ctx, http.MethodGet, "/models")
			size := streamQueueFrames()*chunkSize + 1
			if chunkSize == maxChunk {
				size = MaxModelsResponseBytes
			}
			body := bytes.Repeat([]byte("m"), size)
			for offset := 0; offset < len(body); offset += chunkSize {
				f.send(t, frame{frameResponseData, id, body[offset:min(offset+chunkSize, len(body))]})
			}
			f.send(t, frame{kind: frameComplete, id: id})
			f.manager.mu.Lock()
			session := f.manager.sessions[1]
			f.manager.mu.Unlock()
			eventually(t, func() bool {
				session.mu.Lock()
				stream := session.streams[id]
				session.mu.Unlock()
				if stream == nil {
					t.Fatal("unread models response removed before consumption")
				}
				select {
				case <-stream.done:
					return true
				default:
					return false
				}
			})
			if status := f.manager.Snapshot(1); status.InFlight != 1 {
				t.Fatalf("unread response released capacity: %+v", status)
			}
			got, err := io.ReadAll(response.Body)
			if err != nil || !bytes.Equal(got, body) {
				t.Fatalf("model response bytes=%d want=%d error=%v", len(got), len(body), err)
			}
			eventually(t, func() bool { return f.manager.Snapshot(1).InFlight == 0 })
		})
	}
}

// 慢消费在配置了 consumer_wait_timeout 后应获得宽限：
// 消费端只要能在宽限内开始读，就能把完整响应读完而不触发 ErrSlowConsumer。
func TestSlowConsumerRecoversWithinConfiguredWait(t *testing.T) {
	setNodeTransportConfig(t, 2, 5*time.Second)
	f := setupModelsResponse(t)
	response, id := f.start(t, f.ctx, http.MethodPost, "/chat/completions")

	// 塞满并超过缓冲（2 帧），消费端尚未开始读。
	chunk := bytes.Repeat([]byte("x"), 64)
	for i := 0; i < 4; i++ {
		f.send(t, frame{frameResponseData, id, chunk})
	}
	// 宽限内开始消费；先结束流（frameComplete），readLoop 才会把帧放行给 Read。
	f.send(t, frame{kind: frameComplete, id: id})
	body, err := io.ReadAll(response.Body)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("unexpected read error %v", err)
	}
	if len(body) != 4*len(chunk) {
		t.Fatalf("expected %d bytes, got %d", 4*len(chunk), len(body))
	}
}

// 未配置宽限（默认 0）时保持旧行为：缓冲一满立即判慢消费。
func TestSlowConsumerImmediateFailByDefault(t *testing.T) {
	setNodeTransportConfig(t, 2, 0)
	f := setupModelsResponse(t)
	response, id := f.start(t, f.ctx, http.MethodPost, "/chat/completions")
	defer response.Body.Close()

	chunk := bytes.Repeat([]byte("x"), 64)
	for i := 0; i < 8; i++ {
		f.send(t, frame{frameResponseData, id, chunk})
	}
	_, err := io.ReadAll(response.Body)
	if !errors.Is(err, ErrSlowConsumer) || !IsExecutionUncertain(err) {
		t.Fatalf("expected ErrSlowConsumer + ExecutionUncertain, got %v", err)
	}
}

func TestModelsResponseLimitDoesNotBlockOtherStreams(t *testing.T) {
	f := setupModelsResponse(t)
	response, id := f.start(t, f.ctx, http.MethodGet, "/models")
	chunk := bytes.Repeat([]byte("m"), maxChunk)
	for n := 0; n < MaxModelsResponseBytes; n += len(chunk) {
		f.send(t, frame{frameResponseData, id, chunk})
	}
	other, otherID := f.start(t, f.ctx, http.MethodPost, "/responses")
	f.send(t, frame{frameResponseData, otherID, []byte("data: ok\n\n")})
	f.send(t, frame{kind: frameComplete, id: otherID})
	got, err := io.ReadAll(other.Body)
	if err != nil || string(got) != "data: ok\n\n" {
		t.Fatalf("models response blocked another stream: %q %v", got, err)
	}
	f.send(t, frame{frameResponseData, id, []byte("x")})
	_, err = io.ReadAll(response.Body)
	if !errors.Is(err, ErrInvalidModels) || !IsExecutionUncertain(err) {
		t.Fatalf("models limit classification: %v", err)
	}
	for {
		message, err := readFrame(f.ctx, f.peer)
		if err != nil || message.kind != frameCancel || (message.id != id && message.id != otherID) {
			t.Fatalf("oversize response not cancelled: %+v %v", message, err)
		}
		if message.id == id {
			break
		}
	}
	if status := f.manager.Snapshot(1); !status.Online || status.InFlight != 0 {
		t.Fatalf("models overflow affected session/capacity: %+v", status)
	}
}

func TestModelsResponseCancellationAndCloseReleaseCapacity(t *testing.T) {
	for _, mode := range []string{"cancel", "close", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			f := setupModelsResponse(t)
			ctx, cancel := context.WithCancel(f.ctx)
			if mode == "timeout" {
				cancel()
				ctx, cancel = context.WithTimeout(f.ctx, 100*time.Millisecond)
			}
			defer cancel()
			response, id := f.start(t, ctx, http.MethodGet, "/models")
			f.send(t, frame{frameResponseData, id, []byte("unfinished models")})
			switch mode {
			case "cancel":
				cancel()
				_, err := io.ReadAll(response.Body)
				if !errors.Is(err, context.Canceled) || !IsExecutionUncertain(err) {
					t.Fatalf("cancellation classification: %v", err)
				}
			case "timeout":
				_, err := io.ReadAll(response.Body)
				if !errors.Is(err, context.DeadlineExceeded) || !IsExecutionUncertain(err) {
					t.Fatalf("deadline classification: %v", err)
				}
			default:
				_ = response.Body.Close()
				_ = response.Body.Close()
			}
			eventually(t, func() bool { return f.manager.Snapshot(1).InFlight == 0 })
			message, err := readFrame(f.ctx, f.peer)
			if err != nil || message.kind != frameCancel || message.id != id {
				t.Fatalf("unfinished model response not cancelled: %+v %v", message, err)
			}
		})
	}
}
