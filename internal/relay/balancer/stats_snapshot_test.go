package balancer

import (
	"fmt"
	"testing"
	"time"
)

// TestGetAutoStatsSnapshot_ReturnsAllAndFiltered 验证快照读取全部条目，
// 并能按 channelIDs 过滤。
func TestGetAutoStatsSnapshot_ReturnsAllAndFiltered(t *testing.T) {
	clearAutoStatsForTest()

	modelA := fmt.Sprintf("snap-a-%d", time.Now().UnixNano())
	modelB := fmt.Sprintf("snap-b-%d", time.Now().UnixNano())

	// 渠道1: 4 成功 1 失败；渠道2: 2 成功；渠道3: 1 失败
	recordOutcome(1, modelA, true, 4)
	recordOutcome(1, modelA, false, 1)
	recordOutcome(2, modelA, true, 2)
	recordOutcome(3, modelB, false, 1)

	t.Cleanup(clearAutoStatsForTest)

	// 全量：应返回 3 条（渠道1+modelA、渠道2+modelA、渠道3+modelB）
	all := GetAutoStatsSnapshot(nil)
	if len(all) != 3 {
		t.Fatalf("full snapshot len = %d, want 3", len(all))
	}

	byKey := make(map[string]AutoStatsSnapshotItem, len(all))
	for _, s := range all {
		byKey[fmt.Sprintf("%d:%s", s.ChannelID, s.ModelName)] = s
	}
	item1, ok := byKey[fmt.Sprintf("1:%s", modelA)]
	if !ok {
		t.Fatalf("missing channel1+modelA in snapshot")
	}
	// 4 成功 / 5 总 = 0.8
	if item1.SuccessRate < 0.79 || item1.SuccessRate > 0.81 {
		t.Fatalf("channel1 success rate = %f, want ~0.8", item1.SuccessRate)
	}
	if item1.SampleCount != 5 {
		t.Fatalf("channel1 sample count = %d, want 5", item1.SampleCount)
	}

	// 过滤：只取渠道2 和 渠道3
	filtered := GetAutoStatsSnapshot([]int{2, 3})
	if len(filtered) != 2 {
		t.Fatalf("filtered snapshot len = %d, want 2", len(filtered))
	}
	for _, s := range filtered {
		if s.ChannelID != 2 && s.ChannelID != 3 {
			t.Fatalf("filtered snapshot leaked channel %d", s.ChannelID)
		}
	}
}

// TestGetAutoStatsSnapshot_EmptyWhenNoStats 验证无数据时返回空切片。
func TestGetAutoStatsSnapshot_EmptyWhenNoStats(t *testing.T) {
	clearAutoStatsForTest()
	t.Cleanup(clearAutoStatsForTest)

	got := GetAutoStatsSnapshot(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty snapshot, got %d", len(got))
	}
}
