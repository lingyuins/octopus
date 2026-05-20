package op

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
)

func TestStatsRefreshCache_LoadsStatsModels(t *testing.T) {
	restoreStats := snapshotStatsPersistenceState()
	defer restoreStats()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name()))
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	modelStat := model.StatsModel{
		ID:        101,
		Name:      "gpt-4o",
		ChannelID: 9,
		StatsMetrics: model.StatsMetrics{
			RequestSuccess: 7,
			RequestFailed:  2,
		},
	}
	if err := db.GetDB().Create(&modelStat).Error; err != nil {
		t.Fatalf("seed stats model: %v", err)
	}

	if err := statsRefreshCache(context.Background()); err != nil {
		t.Fatalf("statsRefreshCache() error = %v", err)
	}

	got := StatsModelList()
	if len(got) != 1 {
		t.Fatalf("StatsModelList() len = %d, want 1", len(got))
	}
	if got[0].ID != 101 || got[0].Name != "gpt-4o" || got[0].ChannelID != 9 {
		t.Fatalf("unexpected stats model loaded: %+v", got[0])
	}
	if got[0].RequestSuccess != 7 || got[0].RequestFailed != 2 {
		t.Fatalf("unexpected stats model metrics loaded: %+v", got[0])
	}
}
