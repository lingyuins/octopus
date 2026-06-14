package relaylog

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

// TestRelayLogAttemptsAddPersistsFailedAttempt 验证 RelayLogAttemptsAdd 把失败尝试写入
// relay_log_attempts，使"渠道A 失败→重试到B 成功"中的渠道A 失败可被检索（issue #67）。
func TestRelayLogAttemptsAddPersistsFailedAttempt(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "relaylog-attempts.db")
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

	// 渠道A 失败 + 渠道B 成功。
	attempts := []model.ChannelAttempt{
		{ChannelID: 11, ChannelName: "channelA", ModelName: "gpt-4o", Status: model.AttemptFailed, Duration: 120},
		{ChannelID: 22, ChannelName: "channelB", ModelName: "gpt-4o", Status: model.AttemptSuccess, Duration: 340},
	}
	if err := RelayLogAttemptsAdd(context.Background(), 999, attempts, 1700_000_000); err != nil {
		t.Fatalf("RelayLogAttemptsAdd error: %v", err)
	}

	conn := db.GetLogDB()
	if conn == nil {
		t.Fatalf("GetLogDB returned nil")
	}
	var rows []model.RelayLogAttempt
	if err := conn.Where("relay_log_id = ?", 999).Find(&rows).Error; err != nil {
		t.Fatalf("query attempts failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("attempts count = %d, want 2", len(rows))
	}

	statusByChannel := make(map[int]string, len(rows))
	for _, r := range rows {
		statusByChannel[r.ChannelID] = r.Status
	}
	if statusByChannel[11] != string(model.AttemptFailed) {
		t.Fatalf("channel 11 status = %q, want %q", statusByChannel[11], model.AttemptFailed)
	}
	if statusByChannel[22] != string(model.AttemptSuccess) {
		t.Fatalf("channel 22 status = %q, want %q", statusByChannel[22], model.AttemptSuccess)
	}
}

// TestRelayLogListIncludeAttemptsMatchesFailedChannel 验证 IncludeAttempts=true 时，
// 按渠道A 筛选能命中"在A 失败、B 成功"的请求（内存缓存路径）。
func TestRelayLogListIncludeAttemptsMatchesFailedChannel(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "relaylog-include.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := setting.RefreshCache(context.Background()); err != nil {
		t.Fatalf("RefreshCache failed: %v", err)
	}
	if err := setting.SetString(model.SettingKeyRelayLogKeepEnabled, "false"); err != nil {
		t.Fatalf("disable relay log keep failed: %v", err)
	}

	// 请求在渠道A 失败、渠道B 成功——顶层 channel=B、error=""（最终成功）。
	restore := SetCacheForTest([]model.RelayLog{
		{
			ID: 500, Time: 100, RequestModelName: "gpt-4o", ActualModelName: "gpt-4o",
			ChannelId: 22, ChannelName: "channelB", Error: "",
			Attempts: []model.ChannelAttempt{
				{ChannelID: 11, ChannelName: "channelA", ModelName: "gpt-4o", Status: model.AttemptFailed, Duration: 50},
				{ChannelID: 22, ChannelName: "channelB", ModelName: "gpt-4o", Status: model.AttemptSuccess, Duration: 200},
			},
			TotalAttempts: 2,
		},
	})
	t.Cleanup(restore)

	ids := func(logs []model.RelayLogListItem) []int64 {
		out := make([]int64, 0, len(logs))
		for _, l := range logs {
			out = append(out, l.ID)
		}
		return out
	}

	// 不开 IncludeAttempts：按渠道A 筛选（顶层是B）应命中 0 条。
	logs, err := RelayLogList(context.Background(), LogFilter{ChannelID: intPtr(11)}, 1, 50)
	if err != nil {
		t.Fatalf("RelayLogList error: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("without IncludeAttempts, channelA filter got %v, want empty", ids(logs))
	}

	// 开 IncludeAttempts：按渠道A 筛选应命中这条请求。
	logs, err = RelayLogList(context.Background(), LogFilter{ChannelID: intPtr(11), IncludeAttempts: true}, 1, 50)
	if err != nil {
		t.Fatalf("RelayLogList error: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != 500 {
		t.Fatalf("with IncludeAttempts, channelA filter got %v, want [500]", ids(logs))
	}

	// 开 IncludeAttempts + HasError=true：也应命中（该请求含失败尝试）。
	tt := true
	logs, err = RelayLogList(context.Background(), LogFilter{HasError: &tt, IncludeAttempts: true}, 1, 50)
	if err != nil {
		t.Fatalf("RelayLogList error: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != 500 {
		t.Fatalf("with IncludeAttempts + HasError=true, got %v, want [500]", ids(logs))
	}

	// 开 IncludeAttempts + HasError=false：该请求含失败尝试，应被排除。
	ff := false
	logs, err = RelayLogList(context.Background(), LogFilter{HasError: &ff, IncludeAttempts: true}, 1, 50)
	if err != nil {
		t.Fatalf("RelayLogList error: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("with IncludeAttempts + HasError=false, got %v, want empty (contains failed attempt)", ids(logs))
	}
}

// TestRelayLogListIncludeAttemptsDBSubquery 验证 DB 路径下子查询同样命中失败渠道。
func TestRelayLogListIncludeAttemptsDBSubquery(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "relaylog-include-db.db")
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

	// 顶层渠道=B、整体成功；attempts 表含渠道A 的失败。
	relayLog := model.RelayLog{
		ID: 700, Time: 200, RequestModelName: "gpt-4o", ActualModelName: "gpt-4o",
		ChannelId: 22, ChannelName: "channelB", Error: "", TotalAttempts: 2,
		Attempts: []model.ChannelAttempt{
			{ChannelID: 11, ChannelName: "channelA", ModelName: "gpt-4o", Status: model.AttemptFailed},
			{ChannelID: 22, ChannelName: "channelB", ModelName: "gpt-4o", Status: model.AttemptSuccess},
		},
	}
	if err := db.GetLogDB().Create(&relayLog).Error; err != nil {
		t.Fatalf("seed relay log failed: %v", err)
	}
	if err := RelayLogAttemptsAdd(context.Background(), 700, relayLog.Attempts, 200); err != nil {
		t.Fatalf("RelayLogAttemptsAdd error: %v", err)
	}

	// 清空缓存避免内存路径干扰。
	restore := SetCacheForTest(nil)
	t.Cleanup(restore)

	// 不开 IncludeAttempts：按渠道A（顶层是B）应命中 0 条。
	logs, err := RelayLogList(context.Background(), LogFilter{ChannelID: intPtr(11)}, 1, 50)
	if err != nil {
		t.Fatalf("RelayLogList error: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("without IncludeAttempts, channelA DB filter got %d logs, want 0", len(logs))
	}

	// 开 IncludeAttempts：按渠道A 应命中 1 条。
	logs, err = RelayLogList(context.Background(), LogFilter{ChannelID: intPtr(11), IncludeAttempts: true}, 1, 50)
	if err != nil {
		t.Fatalf("RelayLogList error: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != 700 {
		t.Fatalf("with IncludeAttempts, channelA DB filter got %d logs, want id=700", len(logs))
	}
}
