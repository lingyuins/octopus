package nodetransport

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/lingyuins/octopus/internal/nodetransport/testpeer"
)

func eventually(t *testing.T, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition did not become true")
}

type fixture struct{ manager *Manager }

func setup(t *testing.T, handler http.HandlerFunc) *fixture {
	t.Helper()
	m := NewManager()
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer node-test" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = m.Accept(r.Context(), 1, w, r, AcceptOptions{MaxConcurrency: 4, Validate: func() bool { return true }})
	}))
	t.Cleanup(cloud.Close)
	t.Cleanup(func() { _ = m.Close() })
	testpeer.Start(t, "ws"+strings.TrimPrefix(cloud.URL, "http")+"/node/v1/connect", http.Header{"Authorization": []string{"Bearer node-test"}}, 4, handler)
	eventually(t, func() bool { return m.Status(1).Online })
	return &fixture{manager: m}
}

func modelRequest(t *testing.T, ctx context.Context, path, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, BaseURL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestStreamingRoutesAndCredentialBoundary(t *testing.T) {
	var count atomic.Int32
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Api-Key") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Unsafe") != "" {
			t.Error("cloud credentials or untrusted headers reached the peer")
		}
		if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Anthropic-Version") != "2023-06-01" {
			t.Error("allowed request headers lost")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != `{"model":"allowed","stream":true}` {
			t.Errorf("request body %q %v", body, err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Set-Cookie", "must-not-escape")
		w.Header().Set("Authorization", "peer-secret")
		w.Header().Set("X-Request-Id", "test-request")
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(20 * time.Millisecond)
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	})
	for _, path := range []string{"/chat/completions", "/responses", "/messages", "/embeddings"} {
		req := modelRequest(t, context.Background(), path, `{"model":"allowed","stream":true}`)
		req.Header.Set("Authorization", "Bearer cloud-secret")
		req.Header.Set("X-Api-Key", "cloud-secret")
		req.Header.Set("Cookie", "secret=1")
		req.Header.Set("X-Forwarded-For", "8.8.8.8")
		req.Header.Set("X-Unsafe", "x")
		req.Header.Set("Anthropic-Version", "2023-06-01")
		resp, err := f.manager.Client(1, time.Second).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Header.Get("Set-Cookie") != "" || resp.Header.Get("Authorization") != "" {
			t.Error("peer secrets leaked")
		}
		if resp.Header.Get("X-Request-Id") != "test-request" {
			t.Error("allowed response header lost")
		}
		scan := bufio.NewReader(resp.Body)
		line, err := scan.ReadString('\n')
		if err != nil || line != "data: first\n" {
			t.Fatalf("first SSE %q %v", line, err)
		}
		rest, err := io.ReadAll(scan)
		_ = resp.Body.Close()
		if err != nil || string(rest) != "\ndata: [DONE]\n\n" {
			t.Fatalf("completion %q %v", rest, err)
		}
	}
	if count.Load() != 4 {
		t.Fatalf("requests %d", count.Load())
	}
}

func TestTargetAllowlistAndNoDialFallback(t *testing.T) {
	m := NewManager()
	defer m.Close()
	for _, base := range []string{BaseURL, BaseURL + "/"} {
		if ValidateChannelTarget(base) != nil {
			t.Fatal(base)
		}
	}
	for _, base := range []string{"http://node.octopus.invalid/v1", "https://node.octopus.invalid:443/v1", BaseURL + "?x=1", BaseURL + "#x", "https://u@node.octopus.invalid/v1", "https://example.com/v1", BaseURL + "/models"} {
		if ValidateChannelTarget(base) == nil {
			t.Errorf("accepted %s", base)
		}
	}
	for _, target := range []string{BaseURL + "/images/generations", BaseURL + "/audio/speech", BaseURL + "/videos/generations", BaseURL + "/../api/v1/user", BaseURL + "/chat/completions?x=1", BaseURL + "/%63hat/completions", "http://127.0.0.1/v1/chat/completions", "https://example.com/v1/chat/completions"} {
		req, _ := http.NewRequest(http.MethodPost, target, nil)
		_, err := m.Client(1, time.Second).Do(req)
		if !errors.Is(err, ErrInvalidTarget) {
			t.Errorf("%s: %v", target, err)
		}
	}
	req := modelRequest(t, context.Background(), "/chat/completions", `{"model":"allowed"}`)
	_, err := m.Client(1, time.Second).Do(req)
	if !errors.Is(err, ErrNodeOffline) || IsExecutionUncertain(err) {
		t.Fatalf("offline %v", err)
	}
}

func TestCancellationAndExplicitBodyClose(t *testing.T) {
	for _, mode := range []string{"context", "body"} {
		t.Run(mode, func(t *testing.T) {
			started := make(chan struct{})
			canceled := make(chan struct{})
			f := setup(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data: hi\n\n")
				w.(http.Flusher).Flush()
				close(started)
				<-r.Context().Done()
				close(canceled)
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			resp, err := f.manager.Client(1, 0).Do(modelRequest(t, ctx, "/responses", `{"model":"allowed"}`))
			if err != nil {
				t.Fatal(err)
			}
			<-started
			if mode == "context" {
				cancel()
				_, err = io.ReadAll(resp.Body)
				if !IsExecutionUncertain(err) || !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation %v", err)
				}
			} else {
				_ = resp.Body.Close()
			}
			select {
			case <-canceled:
			case <-time.After(3 * time.Second):
				t.Fatal("peer did not receive cancellation")
			}
			_ = resp.Body.Close()
			eventually(t, func() bool { return f.manager.Status(1).InFlight == 0 })
		})
	}
}

func TestClientTimeoutCancelsPeerRequest(t *testing.T) {
	canceled := make(chan struct{})
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
		close(canceled)
	})
	_, err := f.manager.Client(1, 100*time.Millisecond).Do(modelRequest(t, context.Background(), "/responses", `{"model":"allowed"}`))
	if !IsExecutionUncertain(err) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout %v", err)
	}
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout did not propagate")
	}
}

func TestDisconnectUncertainAndNeverReplay(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if calls.Add(1) == 1 {
			close(started)
		}
		<-r.Context().Done()
	})
	result := make(chan error, 1)
	go func() {
		_, e := f.manager.Client(1, 0).Do(modelRequest(t, context.Background(), "/responses", `{"model":"allowed"}`))
		result <- e
	}()
	<-started
	f.manager.Disconnect(1)
	select {
	case err := <-result:
		if !IsExecutionUncertain(err) {
			t.Fatalf("disconnect %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("request hung")
	}
	eventually(t, func() bool { return !f.manager.Status(1).Online })
	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 1 {
		t.Fatal("replayed request")
	}
}

func TestParallelRequestsAndLimits(t *testing.T) {
	gate := make(chan struct{})
	var current, max atomic.Int32
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		n := current.Add(1)
		defer current.Add(-1)
		for {
			old := max.Load()
			if n <= old || max.CompareAndSwap(old, n) {
				break
			}
		}
		select {
		case <-gate:
			_, _ = io.WriteString(w, "ok")
		case <-r.Context().Done():
		}
	})
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, e := f.manager.Client(1, 2*time.Second).Do(modelRequest(t, context.Background(), "/responses", `{"model":"allowed"}`))
			if e == nil {
				_, e = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
			errs <- e
		}()
	}
	eventually(t, func() bool { return current.Load() == 4 })
	_, err := f.manager.Client(1, time.Second).Do(modelRequest(t, context.Background(), "/responses", `{"model":"allowed"}`))
	if !errors.Is(err, ErrNodeBusy) || IsExecutionUncertain(err) {
		t.Fatalf("busy %v", err)
	}
	close(gate)
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Error(e)
		}
	}
	if max.Load() != 4 {
		t.Fatalf("max concurrency %d", max.Load())
	}
}

func TestSlowConsumerAbortsBoundedStream(t *testing.T) {
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 1000; i++ {
			if _, err := w.Write(bytes.Repeat([]byte("x"), maxChunk)); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				return
			default:
			}
		}
	})
	resp, err := f.manager.Client(1, 0).Do(modelRequest(t, context.Background(), "/responses", `{"model":"allowed"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	eventually(t, func() bool {
		f.manager.mu.Lock()
		s := f.manager.sessions[1]
		f.manager.mu.Unlock()
		s.mu.Lock()
		n := len(s.streams)
		s.mu.Unlock()
		return n == 0
	})
	if f.manager.Status(1).InFlight != 1 {
		t.Fatal("unread failed response released its capacity")
	}
	b, err := io.ReadAll(resp.Body)
	if !errors.Is(err, ErrSlowConsumer) || !IsExecutionUncertain(err) {
		t.Fatalf("slow consumer %d %v", len(b), err)
	}
	if len(b) > streamQueueFrames()*maxChunk {
		t.Fatalf("unbounded buffer %d", len(b))
	}
}

func TestRequestAndHeaderBounds(t *testing.T) {
	f := setup(t, func(w http.ResponseWriter, r *http.Request) { t.Error("oversize request was dispatched") })
	m := f.manager
	req := modelRequest(t, context.Background(), "/responses", strings.Repeat("x", maxRequest+1))
	_, err := m.Client(1, 0).Do(req)
	if err == nil || IsExecutionUncertain(err) {
		t.Fatal(err)
	}
	req = modelRequest(t, context.Background(), "/responses", `{}`)
	req.Header.Set("Anthropic-Beta", strings.Repeat("x", maxHeader+1))
	_, err = m.Client(1, 0).Do(req)
	if err == nil || IsExecutionUncertain(err) {
		t.Fatal(err)
	}
}

func dialRaw(t *testing.T, cloudURL string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(cloudURL, "http")+"/node/v1/connect", &websocket.DialOptions{Subprotocols: []string{protocol}, HTTPHeader: http.Header{"Authorization": []string{"Bearer node-test"}}})
	if err != nil {
		t.Fatal(err)
	}
	f, _ := jsonFrame(frameHello, 0, hello{Version: 1, MaxConcurrency: 4})
	if err = c.Write(ctx, websocket.MessageBinary, f.encode()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

func TestRevocationAndSessionReplacement(t *testing.T) {
	m := NewManager()
	defer m.Close()
	var valid atomic.Bool
	valid.Store(true)
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = m.Accept(r.Context(), 1, w, r, AcceptOptions{Validate: valid.Load})
	}))
	defer cloud.Close()
	first := dialRaw(t, cloud.URL)
	eventually(t, func() bool { return m.Status(1).Online })
	second := dialRaw(t, cloud.URL)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, _, err := first.Read(ctx); err == nil {
		t.Fatal("old session retained")
	}
	eventually(t, func() bool { return m.Status(1).Online })
	_ = first.CloseNow()
	time.Sleep(20 * time.Millisecond)
	if !m.Status(1).Online {
		t.Fatal("old cleanup removed new session")
	}
	valid.Store(false)
	m.Disconnect(1)
	_ = second.CloseNow()
	dialRaw(t, cloud.URL)
	time.Sleep(20 * time.Millisecond)
	if m.Status(1).Online {
		t.Fatal("revoked late handshake registered")
	}
}

func TestMalformedFramesCloseSession(t *testing.T) {
	for _, name := range []string{"oversize", "short", "unexpected", "bad-response"} {
		t.Run(name, func(t *testing.T) {
			m := NewManager()
			defer m.Close()
			cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = m.Accept(r.Context(), 1, w, r, AcceptOptions{Validate: func() bool { return true }})
			}))
			defer cloud.Close()
			c := dialRaw(t, cloud.URL)
			eventually(t, func() bool { return m.Status(1).Online })
			b := []byte{1}
			switch name {
			case "oversize":
				b = make([]byte, maxChunk+10)
			case "unexpected":
				b = frame{kind: frameRequest, id: 1}.encode()
			case "bad-response":
				b = frame{kind: frameResponse, id: 1, data: []byte(`{}`)}.encode()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				go func() { _, _ = m.Client(1, time.Second).Do(modelRequest(t, ctx, "/responses", `{"model":"allowed"}`)) }()
				readCtx, stop := context.WithTimeout(context.Background(), time.Second)
				defer stop()
				_, _ = readFrame(readCtx, c)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = c.Write(ctx, websocket.MessageBinary, b)
			eventually(t, func() bool { return !m.Status(1).Online })
		})
	}
}

func TestCompletionDistinctFromDisconnect(t *testing.T) {
	f := setup(t, func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "complete") })
	for i := 0; i < 50; i++ {
		resp, err := f.manager.Client(1, time.Second).Do(modelRequest(t, context.Background(), "/responses", `{"model":"allowed"}`))
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		b, e := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if e != nil || string(b) != "complete" {
			t.Fatalf("completion %q %v", b, e)
		}
	}
}

func TestNoFullResponseBase64Envelope(t *testing.T) {
	f := frame{frameResponseData, 99, []byte("data: hello\n\n")}
	encoded := f.encode()
	if !bytes.Equal(encoded[9:], f.data) {
		t.Fatal("chunk payload not raw binary")
	}
}
