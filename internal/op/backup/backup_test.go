package backup

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	internaldb "github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
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
	if !strings.Contains(text, `"audit_logs", "auto_strategy_states", "circuit_breaker_states"`) {
		t.Fatal("full import delete order does not clear runtime or circuit_breaker_states")
	}
	if !strings.Contains(text, `doNothing("circuit_breaker_states", &dump.CircuitBreakerStates, len(dump.CircuitBreakerStates))`) {
		t.Fatal("ImportWithMode does not restore circuit_breaker_states")
	}
}

func TestImportWithModeFullClearsExistingRowsUsingActualTableNames(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "backup.db")
	if err := internaldb.InitDB("sqlite", dbPath, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		_ = internaldb.Close()
	})

	dbConn := internaldb.GetDB()
	legacyChannel := model.Channel{ID: 1, Name: "legacy-channel", Type: outbound.OutboundTypeOpenAIChat, BaseUrls: []model.BaseUrl{{URL: "https://legacy.example.com"}}}
	legacyGroup := model.Group{ID: 1, Name: "legacy-group", Mode: model.GroupModeRoundRobin, EndpointType: model.EndpointTypeChat}
	legacyAlert := model.AlertHistory{ID: 1, RuleID: 1, RuleName: "legacy", Message: "legacy", Time: 1}
	legacyRuntime := model.AutoStrategyState{Key: "legacy", ChannelID: 1, ModelName: "gpt-4o", UpdatedAt: 1}
	legacyStats := model.StatsTotal{ID: 1}

	for _, row := range []any{&legacyChannel, &legacyGroup, &legacyAlert, &legacyRuntime, &legacyStats} {
		if err := dbConn.Create(row).Error; err != nil {
			t.Fatalf("seed legacy row: %v", err)
		}
	}

	dump := &model.DBDump{
		Version:       1,
		Channels:      []model.Channel{{ID: 2, Name: "new-channel", Type: outbound.OutboundTypeOpenAIChat, BaseUrls: []model.BaseUrl{{URL: "https://new.example.com"}}}},
		Groups:        []model.Group{{ID: 2, Name: "new-group", Mode: model.GroupModeRandom, EndpointType: model.EndpointTypeChat}},
		AlertHistory:  []model.AlertHistory{{ID: 2, RuleID: 2, RuleName: "new", Message: "new", Time: 2}},
		RuntimeStates: []model.AutoStrategyState{{Key: "new", ChannelID: 2, ModelName: "gpt-4.1", UpdatedAt: 2}},
		IncludeStats:  true,
		StatsTotal:    []model.StatsTotal{{ID: 2}},
	}

	if _, err := ImportWithMode(context.Background(), dump, model.ImportModeFull); err != nil {
		t.Fatalf("full import: %v", err)
	}

	assertCount := func(modelValue any, expected int64, where string, args ...any) {
		t.Helper()
		var count int64
		query := dbConn.Model(modelValue)
		if where != "" {
			query = query.Where(where, args...)
		}
		if err := query.Count(&count).Error; err != nil {
			t.Fatalf("count %T: %v", modelValue, err)
		}
		if count != expected {
			t.Fatalf("count %T = %d, want %d", modelValue, count, expected)
		}
	}

	assertCount(&model.Channel{}, 0, "id = ?", 1)
	assertCount(&model.Channel{}, 1, "id = ?", 2)
	assertCount(&model.Group{}, 0, "id = ?", 1)
	assertCount(&model.Group{}, 1, "id = ?", 2)
	assertCount(&model.AlertHistory{}, 0, "id = ?", 1)
	assertCount(&model.AlertHistory{}, 1, "id = ?", 2)
	assertCount(&model.AutoStrategyState{}, 0, "key = ?", "legacy")
	assertCount(&model.AutoStrategyState{}, 1, "key = ?", "new")
	assertCount(&model.StatsTotal{}, 0, "id = ?", 1)
	assertCount(&model.StatsTotal{}, 1, "id = ?", 2)
}
