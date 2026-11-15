package nodetransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// This optional fixture executes an explicitly selected external JS module. The
// ordinary cloud suite does not require a plugin checkout or a JS runtime.
func TestCrossLanguagePlugin(t *testing.T) {
	module := os.Getenv("OCTOPUS_NODE_PLUGIN_TEST_MODULE")
	if module == "" {
		t.Skip("set OCTOPUS_NODE_PLUGIN_TEST_MODULE to explicitly enable the external JS fixture")
	}
	runtime := os.Getenv("OCTOPUS_NODE_PLUGIN_RUNTIME")
	if runtime == "" {
		runtime = "node"
	}

	m := NewManager()
	t.Cleanup(func() { _ = m.Close() })
	var calls atomic.Int32
	cancelSeen := make(chan struct{}, 1)
	disconnectStarted := make(chan struct{}, 1)
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer fixture-local-key" || r.Header.Get("Cookie") != "" || r.Header.Get("X-Unsafe") != "" {
			t.Error("cross-language credential boundary failed")
		}
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"allowed","api_key":"fixture-private"},{"id":"not-allowed"}]}`)
			return
		}
		var body struct {
			Model string `json:"model"`
			Case  string `json:"case"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model != "allowed" {
			t.Errorf("cross-language body: model=%q err=%v", body.Model, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Set-Cookie", "fixture-private")
		w.Header().Set("X-Request-Id", "cross-language")
		switch body.Case {
		case "sse":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: first\n\n")
			w.(http.Flusher).Flush()
			time.Sleep(20 * time.Millisecond)
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
		case "cancel":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: waiting\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			cancelSeen <- struct{}{}
		case "disconnect":
			disconnectStarted <- struct{}{}
			<-r.Context().Done()
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ok":true}`)
		}
	}))
	t.Cleanup(local.Close)
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-node-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = m.Accept(r.Context(), 1, w, r, AcceptOptions{MaxConcurrency: 4, Validate: func() bool { return true }})
	}))
	t.Cleanup(cloud.Close)
	config, err := json.Marshal(map[string]interface{}{
		"cloudURL":  "ws" + strings.TrimPrefix(cloud.URL, "http") + "/node/v1/connect",
		"nodeToken": "fixture-node-token", "magpieURL": local.URL, "magpieKey": "fixture-local-key",
		"models": []string{"allowed"}, "maxConcurrency": 4, "requestTimeoutSeconds": 5, "allowInsecureLoopback": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	start := func() {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		// stdin provides graceful shutdown; cancellation is a bounded fallback.
		script := `(async () => {
const { pathToFileURL } = await import('node:url');
const { resolve } = await import('node:path');
const { connectOnce } = await import(pathToFileURL(resolve(process.env.OCTOPUS_NODE_PLUGIN_TEST_MODULE)).href);
const controller = new AbortController();
process.stdin.once('data', () => controller.abort());
process.stdin.resume();
try { await connectOnce(JSON.parse(process.env.OCTOPUS_NODE_FIXTURE_CONFIG), { signal: controller.signal }); }
finally { process.stdin.destroy(); }
})().catch(error => { console.error(error.message); process.exitCode = 1; });`
		cmd := exec.CommandContext(ctx, runtime, "-e", script)
		cmd.Env = append(os.Environ(), "OCTOPUS_NODE_FIXTURE_CONFIG="+string(config))
		var output bytes.Buffer
		cmd.Stdout, cmd.Stderr = &output, &output
		input, err := cmd.StdinPipe()
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			cancel()
			_ = input.Close()
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		ended := false
		t.Cleanup(func() {
			_, _ = io.WriteString(input, "stop\n")
			_ = input.Close()
			if !ended {
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					cancel()
					<-done
				}
			}
			cancel()
		})
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for {
			if m.Status(1).Online {
				return
			}
			select {
			case err := <-done:
				ended = true
				t.Fatalf("JS fixture exited before registration: %v\n%s", err, output.String())
			case <-deadline.C:
				t.Fatal("JS fixture did not register")
			case <-tick.C:
			}
		}
	}
	start()
	client := m.Client(1, 3*time.Second)
	resp, err := client.Get(BaseURL + "/models")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil || string(body) != `{"object":"list","data":[{"id":"allowed","object":"model"}]}` {
		t.Fatalf("filtered models: %q %v", body, err)
	}
	noDeadlineClient := m.Client(1, 0)
	resp, err = noDeadlineClient.Do(modelRequest(t, context.Background(), "/responses", `{"model":"allowed","case":"json"}`))
	if err != nil {
		t.Fatalf("unbounded cross-language request: %v", err)
	}
	body, err = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil || string(body) != `{"ok":true}` {
		t.Fatalf("unbounded cross-language roundtrip: %q %v", body, err)
	}
	largeBody := `{"model":"allowed","case":"json","padding":"` + strings.Repeat("x", 5<<20) + `"}`
	resp, err = noDeadlineClient.Do(modelRequest(t, context.Background(), "/responses", largeBody))
	if err != nil {
		t.Fatalf("large cross-language request: %v", err)
	}
	body, err = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil || string(body) != `{"ok":true}` {
		t.Fatalf("large cross-language roundtrip: %q %v", body, err)
	}

	for _, item := range []struct{ name, want string }{{"json", `{"ok":true}`}, {"sse", "data: first\n\ndata: [DONE]\n\n"}} {
		req := modelRequest(t, context.Background(), "/responses", fmt.Sprintf(`{"model":"allowed","case":%q}`, item.name))
		req.Header.Set("Authorization", "Bearer fixture-cloud-secret")
		req.Header.Set("Cookie", "fixture=private")
		req.Header.Set("X-Unsafe", "fixture-private")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil || string(body) != item.want {
			t.Fatalf("%s roundtrip %q %v", item.name, body, err)
		}
		if resp.Header.Get("Set-Cookie") != "" || resp.Header.Get("X-Request-Id") != "cross-language" {
			t.Fatal("cross-language response header boundary failed")
		}
	}
	resp, err = client.Do(modelRequest(t, context.Background(), "/responses", `{"model":"allowed","case":"cancel"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	select {
	case <-cancelSeen:
	case <-time.After(3 * time.Second):
		t.Fatal("JS fixture did not propagate cancellation")
	}
	result := make(chan error, 1)
	go func() {
		_, err := client.Do(modelRequest(t, context.Background(), "/responses", `{"model":"allowed","case":"disconnect"}`))
		result <- err
	}()
	select {
	case <-disconnectStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("disconnect request did not reach JS fixture")
	}
	m.Disconnect(1)
	select {
	case err := <-result:
		if !IsExecutionUncertain(err) || !errors.Is(err, ErrNodeOffline) {
			t.Fatalf("cross-language disconnect: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cross-language request hung after disconnect")
	}
	start()
	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 7 {
		t.Fatalf("cross-language request replayed: %d calls", calls.Load())
	}
	resp, err = client.Do(modelRequest(t, context.Background(), "/responses", `{"model":"allowed","case":"json"}`))
	if err != nil {
		t.Fatal(err)
	}
	body, err = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil || string(body) != `{"ok":true}` || calls.Load() != 8 {
		t.Fatalf("new session roundtrip %q %v calls=%d", body, err, calls.Load())
	}
}
