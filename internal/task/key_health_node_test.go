package task

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/helper"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/nodetransport"
	nodeop "github.com/lingyuins/octopus/internal/op/node"
)

func TestNodeKeyHealthFailureDetailRetainsProbeDiagnostic(t *testing.T) {
	if err := db.InitDB("sqlite", filepath.Join(t.TempDir(), "node-health.db"), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	created, err := nodeop.Create(context.Background(), "health-test", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	manager := nodetransport.NewManager()
	previous := nodetransport.DefaultManager
	nodetransport.DefaultManager = manager
	t.Cleanup(func() { _ = manager.Close(); nodetransport.DefaultManager = previous })
	summary, err := helper.TestChannel(context.Background(), model.Channel{NodeID: created.Node.ID})
	if err != nil || summary == nil || len(summary.Results) != 1 || summary.Results[0].DiagnosticCode != "offline" {
		t.Fatalf("offline probe: %+v %v", summary, err)
	}
	if got := buildKeyHealthFailureDetail(summary); got != "node: offline" {
		t.Fatalf("node diagnostic lost: %q", got)
	}
}

func TestNodeKeyHealthFailureDetailUsesOnlySafeDiagnosticCodes(t *testing.T) {
	for _, code := range []string{
		"disabled", "offline", "busy", "timeout", "cancelled", "local_unavailable",
		"local_authentication", "local_http", "invalid_models", "no_models", "upstream_failed",
	} {
		t.Run(code, func(t *testing.T) {
			summary := &helper.ChannelTestSummary{Results: []helper.ChannelTestResult{{
				DiagnosticCode: code,
				Message:        "untrusted private error",
				KeyRemark:      "private remark",
				KeyMasked:      "private credential",
			}}}
			if got := buildKeyHealthFailureDetail(summary); got != "node: "+code {
				t.Fatalf("unsafe or missing diagnostic: %q", got)
			}
		})
	}
	for _, code := range []string{"ok", "https://example.invalid/private?key=not-a-credential", "offline\nprivate"} {
		summary := &helper.ChannelTestSummary{Results: []helper.ChannelTestResult{{DiagnosticCode: code}}}
		if got := buildKeyHealthFailureDetail(summary); got != "node: upstream_failed" {
			t.Fatalf("invalid failure diagnostic reflected: %q", got)
		}
	}
	summary := &helper.ChannelTestSummary{Results: []helper.ChannelTestResult{
		{Passed: true, DiagnosticCode: "ok"},
		{DiagnosticCode: "offline"},
		{KeyRemark: "ordinary", Message: "HTTP 401"},
	}}
	if got := buildKeyHealthFailureDetail(summary); got != "node: offline; ordinary: HTTP 401" || strings.Contains(got, "ok") {
		t.Fatalf("mixed results: %q", got)
	}
}
