package relay

import (
	"testing"

	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/relay/balancer"
)

// issue #192 回归：渠道内 key 循环的终止性依赖「被跳过（失败提示/熔断）的 key
// 进入 failedKeyIDs 后，重选必须排除它们，全部排除后返回空并 break」。
// 修复前 keyRound == 1 的重选不排除 failedKeyIDs，确定性选 key 策略会选回
// 刚被跳过的同一个 key，造成无限自旋，attempts 记录无限增长
// （实测单条 relay log 440 万条记录 / 805MB）。
func TestPrepareCandidateForRetry_AllKeysExcludedReturnsEmpty(t *testing.T) {
	channel := &dbmodel.Channel{
		ID: 9201,
		Keys: []dbmodel.ChannelKey{
			{ID: 1, ChannelKey: "sk-a", Enabled: true},
			{ID: 2, ChannelKey: "sk-b", Enabled: true},
		},
	}
	iter := balancer.NewIterator(dbmodel.Group{
		Items: []dbmodel.GroupItem{{ChannelID: 9201, ModelName: "gpt-4o"}},
	}, 0, "gpt-4o", nil)
	if !iter.Next() {
		t.Fatal("iterator has no candidates")
	}

	// 部分排除：仍能选出未排除的 key。
	usedKey, _ := PrepareCandidateForRetry(channel, []int{1}, iter, 300, "gpt-4o")
	if usedKey.ID != 2 {
		t.Fatalf("排除 key 1 后应选 key 2，got %d", usedKey.ID)
	}

	// 全部排除：返回空 key，key 循环据此 break，保证不会自旋。
	usedKey, reason := PrepareCandidateForRetry(channel, []int{1, 2}, iter, 300, "gpt-4o")
	if usedKey.ChannelKey != "" {
		t.Fatalf("全部 key 被排除后应返回空，got key ID %d", usedKey.ID)
	}
	if reason != "no more keys to retry" {
		t.Fatalf("reason = %q, want 'no more keys to retry'", reason)
	}
}
