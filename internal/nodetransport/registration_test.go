package nodetransport

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestInvalidHelloNeverRegistersOrValidates(t *testing.T) {
	for i, payload := range []frame{
		{kind: frameRequest},
		{kind: frameHello, id: 1, data: []byte(`{"version":1,"max_concurrency":4}`)},
		{kind: frameHello, data: []byte(`{"version":2,"max_concurrency":4}`)},
		{kind: frameHello, data: []byte(`{"version":1,"max_concurrency":0}`)},
		{kind: frameHello, data: []byte(`{"version":1,"max_concurrency":33}`)},
		{kind: frameHello, data: []byte(`{`)},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			m := NewManager()
			defer m.Close()
			validated := make(chan struct{}, 1)
			cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = m.Accept(r.Context(), 1, w, r, AcceptOptions{Validate: func() bool { validated <- struct{}{}; return true }})
			}))
			defer cloud.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(cloud.URL, "http"), &websocket.DialOptions{Subprotocols: []string{protocol}})
			if err != nil {
				t.Fatal(err)
			}
			defer c.CloseNow()
			if err := c.Write(ctx, websocket.MessageBinary, payload.encode()); err != nil {
				t.Fatal(err)
			}
			if _, _, err := c.Read(ctx); err == nil {
				t.Fatal("invalid hello retained connection")
			}
			if m.Status(1).Online {
				t.Fatal("invalid hello registered")
			}
			select {
			case <-validated:
				t.Fatal("invalid hello reached authorization callback")
			default:
			}
		})
	}
}

func TestConcurrencyNegotiationUsesLowerBound(t *testing.T) {
	for _, item := range []struct{ server, peer, want int }{{2, 4, 2}, {8, 2, 2}, {0, 8, 4}, {100, 32, 32}} {
		m := NewManager()
		cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = m.Accept(r.Context(), 1, w, r, AcceptOptions{MaxConcurrency: item.server, Validate: func() bool { return true }})
		}))
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(cloud.URL, "http"), &websocket.DialOptions{Subprotocols: []string{protocol}})
		if err != nil {
			cancel()
			cloud.Close()
			t.Fatal(err)
		}
		f, err := jsonFrame(frameHello, 0, hello{Version: 1, MaxConcurrency: item.peer})
		if err == nil {
			err = c.Write(ctx, websocket.MessageBinary, f.encode())
		}
		if err != nil {
			_ = c.CloseNow()
			cancel()
			cloud.Close()
			t.Fatal(err)
		}
		eventually(t, func() bool { return m.Status(1).Online })
		if got := m.Status(1).MaxConcurrency; got != item.want {
			t.Errorf("server=%d peer=%d limit=%d want=%d", item.server, item.peer, got, item.want)
		}
		_ = c.CloseNow()
		_ = m.Close()
		cancel()
		cloud.Close()
	}
}

func TestRequestBodyLimitNegotiatesLegacyAndCurrentPeers(t *testing.T) {
	for _, item := range []struct {
		name        string
		peerLimit   int
		oversize    int
		wantSession int
	}{
		{name: "legacy hello retains four MiB ceiling", oversize: legacyMaxRequest + 1, wantSession: legacyMaxRequest},
		{name: "current peer advertises sixteen MiB ceiling", peerLimit: defaultMaxRequest, oversize: legacyMaxRequest + 1, wantSession: defaultMaxRequest},
	} {
		t.Run(item.name, func(t *testing.T) {
			m := NewManager()
			t.Cleanup(func() { _ = m.Close() })
			cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = m.Accept(r.Context(), 1, w, r, AcceptOptions{Validate: func() bool { return true }})
			}))
			t.Cleanup(cloud.Close)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(cloud.URL, "http"), &websocket.DialOptions{Subprotocols: []string{protocol}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = c.CloseNow() })
			f, err := jsonFrame(frameHello, 0, hello{Version: 1, MaxConcurrency: 4, MaxRequestBytes: item.peerLimit})
			if err != nil {
				t.Fatal(err)
			}
			if err := c.Write(ctx, websocket.MessageBinary, f.encode()); err != nil {
				t.Fatal(err)
			}
			eventually(t, func() bool { return m.Status(1).Online })
			m.mu.RLock()
			gotLimit := m.sessions[1].maxRequestBytes
			m.mu.RUnlock()
			if gotLimit != item.wantSession {
				t.Fatalf("session request limit = %d, want %d", gotLimit, item.wantSession)
			}

			request := modelRequest(t, ctx, "/responses", strings.Repeat("x", item.oversize))
			if item.peerLimit > 0 {
				responseDone := make(chan error, 1)
				go func() {
					response, requestErr := m.Client(1, 0).Do(request)
					if requestErr == nil {
						requestErr = response.Body.Close()
					}
					responseDone <- requestErr
				}()
				requestFrame, err := readFrame(ctx, c)
				if err != nil {
					t.Fatalf("read dispatched request: %v", err)
				}
				if requestFrame.kind != frameRequest || requestFrame.id != 1 {
					t.Fatalf("dispatched frame = %+v, want first request", requestFrame)
				}
				responseMetaFrame, err := jsonFrame(frameResponse, requestFrame.id, responseMeta{Status: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}})
				if err != nil {
					t.Fatal(err)
				}
				if err := c.Write(ctx, websocket.MessageBinary, responseMetaFrame.encode()); err != nil {
					t.Fatal(err)
				}
				if err := c.Write(ctx, websocket.MessageBinary, (frame{kind: frameComplete, id: requestFrame.id}).encode()); err != nil {
					t.Fatal(err)
				}
				if err := <-responseDone; err != nil {
					t.Fatalf("current peer request within negotiated limit failed: %v", err)
				}
				return
			}

			_, err = m.Client(1, 0).Do(request)
			if err == nil || !strings.Contains(err.Error(), "node request exceeds limit") {
				t.Fatalf("legacy oversized request error = %v, want request limit rejection", err)
			}
		})
	}
}
