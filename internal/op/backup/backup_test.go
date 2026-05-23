package backup

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func loadBackupSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src, err := os.ReadFile(filepath.Clean(filepath.Join(filepath.Dir(file), "backup.go")))
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	return string(src)
}

func TestFullImportDeleteOrderUsesChannelGroupsTable(t *testing.T) {
	text := loadBackupSource(t)
	if strings.Contains(text, `"group_items", "group_channel_items", "groups"`) {
		t.Fatal("delete order still references legacy group_channel_items table")
	}
	if !strings.Contains(text, `"group_items", "channel_groups", "groups"`) {
		t.Fatal("delete order does not include channel_groups between group_items and groups")
	}
}

func TestBackupIncludesCircuitBreakerStates(t *testing.T) {
	text := loadBackupSource(t)
	if !strings.Contains(text, `Find(&d.CircuitBreakerStates)`) {
		t.Fatal("ExportAll does not export circuit_breaker_states")
	}
	if !strings.Contains(text, `"audit_logs", "runtime_states", "circuit_breaker_states"`) {
		t.Fatal("full import delete order does not clear circuit_breaker_states")
	}
	if !strings.Contains(text, `cfg.doNothing("circuit_breaker_states", toAny(dump.CircuitBreakerStates))`) {
		t.Fatal("ImportWithMode does not restore circuit_breaker_states")
	}
}
