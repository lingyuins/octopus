package analytics

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/relaylog"
	"github.com/lingyuins/octopus/internal/op/setting"
)

// TestBuildGroupHealth_SurfacesFailingChannelFromAttempts 验证 buildGroupHealth 把
// "渠道A 失败→重试到B 成功"中的渠道A 失败计入组的 failureCount，并在 FailingChannels
// 下钻列表中暴露（issue #67 核心修复）。
func TestBuildGroupHealth_SurfacesFailingChannelFromAttempts(t *testing.T) {
	groups := []model.Group{
		{
			ID:   1,
			Name: "gpt-4o",
			Mode: model.GroupModeFailover,
			Items: []model.GroupItem{
				{ChannelID: 11, ModelName: "gpt-4o"},
				{ChannelID: 22, ModelName: "gpt-4o"},
			},
		},
	}
	channelByID := map[int]model.Channel{
		11: {ID: 11, Name: "channelA", Enabled: true},
		22: {ID: 22, Name: "channelB", Enabled: true},
	}

	// 渠道A 在该模型上有 3 次失败（来自 attempts 聚合）。
	failures := map[string]*analyticsFailureAggregateRow{
		makeAnalyticsFailureKey(11, "gpt-4o", "gpt-4o"): {
			ChannelID:        11,
			RequestModelName: "gpt-4o",
			ActualModelName:  "gpt-4o",
			FailureCount:     3,
			LastFailureAt:    1700_000_100,
		},
	}

	items := buildGroupHealth(groups, channelByID, failures)
	if len(items) != 1 {
		t.Fatalf("expected 1 group health item, got %d", len(items))
	}
	item := items[0]
	if item.FailureCount != 3 {
		t.Fatalf("FailureCount = %d, want 3", item.FailureCount)
	}
	if item.Status != "degraded" {
		t.Fatalf("Status = %q, want degraded (failureCount>=3)", item.Status)
	}
	if len(item.FailingChannels) != 1 {
		t.Fatalf("FailingChannels len = %d, want 1", len(item.FailingChannels))
	}
	fc := item.FailingChannels[0]
	if fc.ChannelID != 11 || fc.ChannelName != "channelA" {
		t.Fatalf("failing channel = %+v, want channelA(11)", fc)
	}
	if fc.FailureCount != 3 {
		t.Fatalf("failing channel FailureCount = %d, want 3", fc.FailureCount)
	}
}

// TestLoadAnalyticsFailureRows_AggregatesPerAttemptFromCache 验证内存缓存中整体成功
// 但含失败尝试的请求，会把失败计入对应渠道（而非顶层成功渠道）。
func TestLoadAnalyticsFailureRows_AggregatesPerAttemptFromCache(t *testing.T) {
	restoreLogs := relaylog.SetCacheForTest([]model.RelayLog{
		{
			Time: time.Now().Unix(),
			// 顶层渠道B、整体成功
			ChannelId:        22,
			ActualModelName:  "gpt-4o",
			RequestModelName: "gpt-4o",
			Error:            "",
			Attempts: []model.ChannelAttempt{
				{ChannelID: 11, ModelName: "gpt-4o", Status: model.AttemptFailed},
				{ChannelID: 22, ModelName: "gpt-4o", Status: model.AttemptSuccess},
			},
		},
	})
	defer restoreLogs()

	settingCache := setting.GetCache()
	oldSettings := settingCache.GetAll()
	settingCache.Set(model.SettingKeyRelayLogKeepEnabled, "false")
	defer func() {
		settingCache.Clear()
		for k, v := range oldSettings {
			settingCache.Set(k, v)
		}
	}()

	failures, err := loadAnalyticsFailureRows(context.Background(), time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("loadAnalyticsFailureRows error: %v", err)
	}

	// 渠道A(11) 应有 1 次失败；顶层渠道B(22) 整体成功不应被计入。
	rowA, ok := failures[makeAnalyticsFailureKey(11, "gpt-4o", "gpt-4o")]
	if !ok || rowA == nil {
		t.Fatalf("expected failure row for channelA(11), got nil")
	}
	if rowA.FailureCount != 1 {
		t.Fatalf("channelA FailureCount = %d, want 1", rowA.FailureCount)
	}
	if _, ok := failures[makeAnalyticsFailureKey(22, "gpt-4o", "gpt-4o")]; ok {
		t.Fatalf("channelB(22) should NOT appear in failures (overall success)")
	}
}

// TestLoadAnalyticsChannelModelRows_CountsPerAttemptFailures 验证渠道×模型聚合把
// 重试场景中失败渠道的失败计入其成功率。
func TestLoadAnalyticsChannelModelRows_CountsPerAttemptFailures(t *testing.T) {
	restoreLogs := relaylog.SetCacheForTest([]model.RelayLog{
		{
			Time: time.Now().Unix(),
			// 顶层渠道B、整体成功；含 token/cost
			ChannelId:        22,
			ActualModelName:  "gpt-4o",
			RequestModelName: "gpt-4o",
			Error:            "",
			InputTokens:      100,
			OutputTokens:     50,
			Cost:             0.2,
			Attempts: []model.ChannelAttempt{
				{ChannelID: 11, ModelName: "gpt-4o", Status: model.AttemptFailed},
				{ChannelID: 22, ModelName: "gpt-4o", Status: model.AttemptSuccess},
			},
		},
	})
	defer restoreLogs()

	settingCache := setting.GetCache()
	oldSettings := settingCache.GetAll()
	settingCache.Set(model.SettingKeyRelayLogKeepEnabled, "false")
	defer func() {
		settingCache.Clear()
		for k, v := range oldSettings {
			settingCache.Set(k, v)
		}
	}()

	rows, err := loadAnalyticsChannelModelRows(context.Background(), model.AnalyticsRange7D, nil)
	if err != nil {
		t.Fatalf("loadAnalyticsChannelModelRows error: %v", err)
	}

	rowA, ok := rows["11\x00gpt-4o"]
	if !ok || rowA == nil {
		t.Fatalf("expected row for channelA(11)+gpt-4o")
	}
	// 渠道A：1 次失败、0 次成功；成功率 0%
	if rowA.RequestFailed != 1 || rowA.RequestSuccess != 0 {
		t.Fatalf("channelA got success=%d failed=%d, want success=0 failed=1", rowA.RequestSuccess, rowA.RequestFailed)
	}

	rowB, ok := rows["22\x00gpt-4o"]
	if !ok || rowB == nil {
		t.Fatalf("expected row for channelB(22)+gpt-4o")
	}
	// 渠道B：1 次成功；token/cost 计入（整体成功）
	if rowB.RequestSuccess != 1 || rowB.RequestFailed != 0 {
		t.Fatalf("channelB got success=%d failed=%d, want success=1 failed=0", rowB.RequestSuccess, rowB.RequestFailed)
	}
	if rowB.InputTokens != 100 || rowB.OutputTokens != 50 {
		t.Fatalf("channelB tokens in=%d out=%d, want 100/50", rowB.InputTokens, rowB.OutputTokens)
	}
}

// TestAnalyticsChannelModelBreakdownGet_DBScope 验证 DB 路径下 attempts 表聚合 +
// 分组 scope 过滤。
func TestAnalyticsChannelModelBreakdownGet_DBScope(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "analytics-cm.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := db.InitLogDB("", "", false); err != nil {
		t.Fatalf("InitLogDB(shared) failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := setting.RefreshCache(context.Background()); err != nil {
		t.Fatalf("RefreshCache failed: %v", err)
	}
	if err := setting.SetString(model.SettingKeyRelayLogKeepEnabled, "true"); err != nil {
		t.Fatalf("enable relay log keep failed: %v", err)
	}

	// 顶层渠道B、整体成功；attempts 表含渠道A 失败 + 渠道B 成功。
	relayLog := model.RelayLog{
		ID: 1, Time: time.Now().Unix(),
		RequestModelName: "gpt-4o", ActualModelName: "gpt-4o",
		ChannelId: 22, ChannelName: "channelB", Error: "",
		InputTokens: 100, OutputTokens: 50, Cost: 0.2, TotalAttempts: 2,
		Attempts: []model.ChannelAttempt{
			{ChannelID: 11, ChannelName: "channelA", ModelName: "gpt-4o", Status: model.AttemptFailed},
			{ChannelID: 22, ChannelName: "channelB", ModelName: "gpt-4o", Status: model.AttemptSuccess},
		},
	}
	if err := db.GetLogDB().Create(&relayLog).Error; err != nil {
		t.Fatalf("seed relay log failed: %v", err)
	}
	if err := relaylog.RelayLogAttemptsAdd(context.Background(), 1, relayLog.Attempts, relayLog.Time); err != nil {
		t.Fatalf("RelayLogAttemptsAdd error: %v", err)
	}

	restore := relaylog.SetCacheForTest(nil)
	t.Cleanup(restore)

	items, err := AnalyticsChannelModelBreakdownGet(context.Background(), model.AnalyticsRange7D, nil)
	if err != nil {
		t.Fatalf("AnalyticsChannelModelBreakdownGet error: %v", err)
	}

	// 找到渠道A 的行：应记 1 次失败、0 次成功。
	var foundA, foundB bool
	for _, it := range items {
		if it.ChannelID == 11 && it.ModelName == "gpt-4o" {
			foundA = true
			if it.RequestCount != 1 || it.SuccessRate != 0 {
				t.Fatalf("channelA request_count=%d success_rate=%f, want 1/0", it.RequestCount, it.SuccessRate)
			}
		}
		if it.ChannelID == 22 && it.ModelName == "gpt-4o" {
			foundB = true
			if it.RequestCount != 1 || it.SuccessRate < 99.9 || it.SuccessRate > 100.1 {
				t.Fatalf("channelB request_count=%d success_rate=%f, want 1/100", it.RequestCount, it.SuccessRate)
			}
		}
	}
	if !foundA {
		t.Fatalf("channelA row missing from channel-model breakdown")
	}
	if !foundB {
		t.Fatalf("channelB row missing from channel-model breakdown")
	}
}
