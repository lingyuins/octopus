package helper

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	appmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

// setupHelperDB 初始化 SQLite 主库 + 共享日志库并启用日志保留，供 group_probe
// 测试使用（recordTestLog 依赖 relaylog.RelayLogAdd，需 DB）。
func setupHelperDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "helper-groupprobe.db")
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
	if err := setting.SetString(appmodel.SettingKeyRelayLogKeepEnabled, "true"); err != nil {
		t.Fatalf("enable relay log keep failed: %v", err)
	}
}

// TestTestGroupModelItem_SkipsChannelWithSkipModelTest 验证 issue #98：当渠道
// 设置 SkipModelTest=true 时，分组测试跳过该渠道并返回跳过提示，不会发起真实
// 探测请求（无 mock server，若未跳过会因无可用 base url/key 报其它错误）。
func TestTestGroupModelItem_SkipsChannelWithSkipModelTest(t *testing.T) {
	setupHelperDB(t)

	channels := map[int]appmodel.Channel{
		42: {
			ID:            42,
			Name:          "punishing-upstream",
			Enabled:       true,
			SkipModelTest: true,
			// 故意不设 Keys / BaseUrls：若跳过逻辑失效，会在 key 查找阶段返回
			// "no available key"，而非跳过提示，测试据此区分。
		},
	}
	item := appmodel.GroupItem{ID: 1, ChannelID: 42, ModelName: "gpt-4o-mini"}

	result := testGroupModelItem(context.Background(), appmodel.EndpointTypeChat, item, channels)

	if result.Passed {
		t.Fatalf("expected Passed=false for skipped channel, got true")
	}
	if result.Message != "channel skipped model test" {
		t.Fatalf("expected skip message, got %q", result.Message)
	}
}

// TestTestGroupModelItem_DoesNotSkipWhenFlagFalse 验证 SkipModelTest=false 时
// 不触发跳过（继续走正常校验链，这里因无 key 返回 "no available key"）。
func TestTestGroupModelItem_DoesNotSkipWhenFlagFalse(t *testing.T) {
	setupHelperDB(t)

	channels := map[int]appmodel.Channel{
		43: {ID: 43, Name: "normal", Enabled: true, SkipModelTest: false},
	}
	item := appmodel.GroupItem{ID: 2, ChannelID: 43, ModelName: "gpt-4o-mini"}

	result := testGroupModelItem(context.Background(), appmodel.EndpointTypeChat, item, channels)

	if result.Message == "channel skipped model test" {
		t.Fatalf("channel without SkipModelTest should not be skipped")
	}
	// 无 key 时应走到 "no available key" 分支，证明未被跳过逻辑提前返回。
	if result.Message != "no available key" {
		t.Fatalf("expected 'no available key' for keyless channel, got %q", result.Message)
	}
}
