package helper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/nodetransport"
	"github.com/lingyuins/octopus/internal/nodetransport/testpeer"
	nodeop "github.com/lingyuins/octopus/internal/op/node"
)

func TestNodeProbeErrorCodesNeverReflectRawErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{context.DeadlineExceeded, "timeout"}, {context.Canceled, "cancelled"},
		{nodetransport.ErrNodeBusy, "busy"}, {nodetransport.ErrNodeOffline, "offline"},
		{nodetransport.ErrLocalUnavailable, "local_unavailable"}, {nodetransport.ErrLocalAuthentication, "local_authentication"},
		{nodetransport.ErrLocalHTTP, "local_http"}, {nodetransport.ErrInvalidModels, "invalid_models"},
		{fmt.Errorf("private-key-in-error"), "upstream_failed"},
	} {
		if got := nodeProbeErrorCode(fmt.Errorf("wrapped: %w", tc.err)); got != tc.want {
			t.Fatalf("classification: got %s want %s", got, tc.want)
		}
	}
}

func TestNodeChannelProbeNeverFallsBackToInference(t *testing.T) {
	if err := db.InitDB("sqlite", filepath.Join(t.TempDir(), "node-probe.db"), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	created, err := nodeop.Create(context.Background(), "probe", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	manager := nodetransport.NewManager()
	old := nodetransport.DefaultManager
	nodetransport.DefaultManager = manager
	t.Cleanup(func() { _ = manager.Close(); nodetransport.DefaultManager = old })
	summary, err := TestChannel(context.Background(), model.Channel{NodeID: created.Node.ID, Model: "billable-model"})
	if err != nil || summary.Passed || len(summary.Results) != 1 || summary.Results[0].DiagnosticCode != "offline" {
		t.Fatalf("node probe: %+v %v", summary, err)
	}
	result := summary.Results[0]
	if result.BaseURL != "" || result.KeyMasked != "" || result.Message != "" || result.ResponseBody != "" {
		t.Fatalf("internal node transport exposed: %+v", result)
	}
}

func TestNodeProbeReportsOfflineAndDisabled(t *testing.T) {
	manager := nodetransport.NewManager()
	t.Cleanup(func() { _ = manager.Close() })
	for _, enabled := range []bool{true, false} {
		want := "disabled"
		if enabled {
			want = "offline"
		}
		result := probeNode(context.Background(), 1, enabled, manager)
		if result.Passed || result.Code != want {
			t.Fatalf("enabled=%v: %+v", enabled, result)
		}
	}
}

func TestNodeProbeUsesOnlyModelsAndValidatesResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		body   string
		want   string
		status int
	}{
		{"success", `{"object":"list","data":[{"id":"allowed","object":"model"}]}`, "ok", 200},
		{"empty", `{"object":"list","data":[]}`, "no_models", 200},
		{"invalid JSON", `{"data":`, "invalid_models", 200},
		{"missing list", `{}`, "invalid_models", 200},
		{"invalid item", `{"data":[{"id":1}]}`, "invalid_models", 200},
		{"oversized", strings.Repeat("x", (4<<20)+1), "invalid_models", 200},
		{"wrong key", `private upstream text`, "local_authentication", 401},
		{"forbidden key", `private upstream text`, "local_authentication", 403},
		{"wrong port", `private upstream text`, "local_http", 404},
		{"redirect", `private upstream text`, "local_http", 302},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager := nodetransport.NewManager()
			t.Cleanup(func() { _ = manager.Close() })
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = manager.Accept(r.Context(), 1, w, r, nodetransport.AcceptOptions{Validate: func() bool { return true }})
			}))
			t.Cleanup(server.Close)
			testpeer.Start(t, "ws"+strings.TrimPrefix(server.URL, "http"), nil, 4, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
					t.Errorf("unexpected probe: %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			deadline := time.Now().Add(5 * time.Second)
			for !manager.Snapshot(1).Online && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			result := probeNode(context.Background(), 1, true, manager)
			if result.Code != tc.want || result.Passed != (tc.want == "ok") {
				t.Fatalf("probe: %+v", result)
			}
			if tc.want == "ok" && result.ModelCount != 1 {
				t.Fatalf("model count: %+v", result)
			}
		})
	}
}
