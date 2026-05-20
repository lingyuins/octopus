package op

import (
	"context"
	"testing"

	"github.com/lingyuins/octopus/internal/model"
)

func TestStatsModelUpdatePreservesModelIdentity(t *testing.T) {
	restoreStats := snapshotStatsPersistenceState()
	defer restoreStats()

	if err := StatsModelUpdate(model.StatsModel{
		ID:        101,
		Name:      "gpt-4o",
		ChannelID: 7,
		StatsMetrics: model.StatsMetrics{
			RequestSuccess: 1,
		},
	}); err != nil {
		t.Fatalf("StatsModelUpdate() error = %v", err)
	}

	list := StatsModelList()
	var got *model.StatsModel
	for i := range list {
		if list[i].ID == 101 {
			got = &list[i]
			break
		}
	}
	if got == nil {
		t.Fatal("StatsModelList missing id 101 after update")
	}
	if got.Name != "gpt-4o" {
		t.Fatalf("stats name = %q, want %q", got.Name, "gpt-4o")
	}
	if got.ChannelID != 7 {
		t.Fatalf("stats channel_id = %d, want %d", got.ChannelID, 7)
	}
	if got.RequestSuccess != 1 {
		t.Fatalf("stats request_success = %d, want %d", got.RequestSuccess, 1)
	}
}

func TestLLMListRanksBySuccessRateThenSuccessCount(t *testing.T) {
	restoreLLM := snapshotLLMCacheState()
	defer restoreLLM()
	restoreStats := snapshotStatsPersistenceState()
	defer restoreStats()

	llmModelCache.Clear()
	llmModelCache.Set("gpt-4o", model.LLMPrice{})
	llmModelCache.Set("claude-3-7-sonnet", model.LLMPrice{})
	llmModelCache.Set("gemini-2.5-pro", model.LLMPrice{})
	llmModelCache.Set("o1-mini", model.LLMPrice{})

	if err := StatsModelUpdate(model.StatsModel{
		ID:        1,
		Name:      "gpt-4o",
		ChannelID: 11,
		StatsMetrics: model.StatsMetrics{
			RequestSuccess: 8,
			RequestFailed:  2,
		},
	}); err != nil {
		t.Fatalf("StatsModelUpdate() error = %v", err)
	}
	if err := StatsModelUpdate(model.StatsModel{
		ID:        2,
		Name:      "claude-3-7-sonnet",
		ChannelID: 12,
		StatsMetrics: model.StatsMetrics{
			RequestSuccess: 4,
			RequestFailed:  0,
		},
	}); err != nil {
		t.Fatalf("StatsModelUpdate() error = %v", err)
	}
	if err := StatsModelUpdate(model.StatsModel{
		ID:        3,
		Name:      "gemini-2.5-pro",
		ChannelID: 13,
		StatsMetrics: model.StatsMetrics{
			RequestSuccess: 3,
			RequestFailed:  0,
		},
	}); err != nil {
		t.Fatalf("StatsModelUpdate() error = %v", err)
	}

	got, err := LLMList(context.Background())
	if err != nil {
		t.Fatalf("LLMList() error = %v", err)
	}

	wantOrder := []string{
		"claude-3-7-sonnet",
		"gemini-2.5-pro",
		"gpt-4o",
		"o1-mini",
	}
	if len(got) != len(wantOrder) {
		t.Fatalf("LLMList() len = %d, want %d", len(got), len(wantOrder))
	}
	for i, want := range wantOrder {
		if got[i].Name != want {
			t.Fatalf("LLMList()[%d] = %q, want %q; got=%v", i, got[i].Name, want, got)
		}
	}
}

func snapshotLLMCacheState() func() {
	oldLLMs := llmModelCache.GetAll()
	return func() {
		llmModelCache.Clear()
		for name, price := range oldLLMs {
			llmModelCache.Set(name, price)
		}
	}
}
