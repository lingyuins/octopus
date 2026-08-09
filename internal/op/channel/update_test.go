package channel

import (
	"context"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
)

// advancedFields 保存 Update() 高级设置字段的期望值，用于回归断言。
type advancedFields struct {
	autoSync             bool
	skipModelTest        bool
	disposable           bool
	expireAt             time.Time
	notifChannelID       int
	keySelectionStrategy string
	autoGroup            model.AutoGroupType
	customHeader         []model.CustomHeader
	poolID               int
}

// advancedUpdateRequest 构造携带全部 9 个高级设置字段的更新请求（回归 #182）。
func advancedUpdateRequest(id int) (*model.ChannelUpdateRequest, advancedFields) {
	f := advancedFields{
		autoSync:             true,
		skipModelTest:        true,
		disposable:           true,
		expireAt:             time.Now().Add(24 * time.Hour).Truncate(time.Second),
		notifChannelID:       7,
		keySelectionStrategy: "random",
		autoGroup:            model.AutoGroupTypeExact,
		customHeader: []model.CustomHeader{
			{HeaderKey: "X-Example-Header", HeaderValue: "example-value"},
			{HeaderKey: "X-Second", HeaderValue: "second"},
		},
		poolID: 42,
	}
	req := &model.ChannelUpdateRequest{
		ID:                   id,
		AutoSync:             &f.autoSync,
		SkipModelTest:        &f.skipModelTest,
		Disposable:           &f.disposable,
		ExpireAt:             &f.expireAt,
		NotifChannelID:       &f.notifChannelID,
		KeySelectionStrategy: &f.keySelectionStrategy,
		AutoGroup:            &f.autoGroup,
		CustomHeader:         &f.customHeader,
		PoolID:               &f.poolID,
	}
	return req, f
}

// assertAdvancedFields 断言渠道上的 9 个高级设置字段与期望值一致。
func assertAdvancedFields(t *testing.T, ch *model.Channel, want advancedFields) {
	t.Helper()
	if !ch.AutoSync || !ch.SkipModelTest || !ch.Disposable {
		t.Fatalf("bool fields not persisted: auto_sync=%v skip_model_test=%v disposable=%v",
			ch.AutoSync, ch.SkipModelTest, ch.Disposable)
	}
	if ch.ExpireAt == nil || ch.ExpireAt.Unix() != want.expireAt.Unix() {
		t.Fatalf("ExpireAt = %v, want %v", ch.ExpireAt, want.expireAt)
	}
	if ch.NotifChannelID == nil || *ch.NotifChannelID != want.notifChannelID {
		t.Fatalf("NotifChannelID = %v, want %d", ch.NotifChannelID, want.notifChannelID)
	}
	if ch.KeySelectionStrategy != want.keySelectionStrategy {
		t.Fatalf("KeySelectionStrategy = %q, want %q", ch.KeySelectionStrategy, want.keySelectionStrategy)
	}
	if ch.AutoGroup != want.autoGroup {
		t.Fatalf("AutoGroup = %d, want %d", ch.AutoGroup, want.autoGroup)
	}
	if len(ch.CustomHeader) != len(want.customHeader) {
		t.Fatalf("CustomHeader = %+v, want %+v", ch.CustomHeader, want.customHeader)
	}
	for i := range want.customHeader {
		if ch.CustomHeader[i] != want.customHeader[i] {
			t.Fatalf("CustomHeader[%d] = %+v, want %+v", i, ch.CustomHeader[i], want.customHeader[i])
		}
	}
	if ch.PoolID != want.poolID {
		t.Fatalf("PoolID = %d, want %d", ch.PoolID, want.poolID)
	}
}

// TestUpdatePersistsAllAdvancedFields 回归测试 #182：
// POST /api/v1/channel/update 提交 9 个高级设置字段必须真正落库，
// 且刷新后的缓存（Update 返回值）与 DB 读回值一致。
func TestUpdatePersistsAllAdvancedFields(t *testing.T) {
	setupBatchGroupTest(t)
	seedChannel(t, 1, 1)

	req, want := advancedUpdateRequest(1)
	updated, err := Update(req, context.Background())
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	assertAdvancedFields(t, updated, want)

	var got model.Channel
	if err := db.GetDB().First(&got, 1).Error; err != nil {
		t.Fatalf("reload from DB failed: %v", err)
	}
	assertAdvancedFields(t, &got, want)
}

// TestUpdatePatchSemanticsLeavesOtherFieldsUntouched 验证白名单补丁语义：
// 仅更新单个字段时，其余 9 个高级字段保持不变。
func TestUpdatePatchSemanticsLeavesOtherFieldsUntouched(t *testing.T) {
	setupBatchGroupTest(t)
	seedChannel(t, 1, 1)

	req, want := advancedUpdateRequest(1)
	if _, err := Update(req, context.Background()); err != nil {
		t.Fatalf("initial Update returned error: %v", err)
	}

	name := "renamed"
	if _, err := Update(&model.ChannelUpdateRequest{ID: 1, Name: &name}, context.Background()); err != nil {
		t.Fatalf("name-only Update returned error: %v", err)
	}

	var got model.Channel
	if err := db.GetDB().First(&got, 1).Error; err != nil {
		t.Fatalf("reload from DB failed: %v", err)
	}
	if got.Name != name {
		t.Fatalf("Name = %q, want %q", got.Name, name)
	}
	assertAdvancedFields(t, &got, want)
}

// bindProxyPool 通过显式 proxy_mode=pool 为渠道绑定代理池配置。
func bindProxyPool(t *testing.T, channelID, configID int) {
	t.Helper()
	poolMode := model.ProxyUsageModePool
	req := &model.ChannelUpdateRequest{ID: channelID, ProxyMode: &poolMode, ProxyConfigID: &configID}
	if _, err := Update(req, context.Background()); err != nil {
		t.Fatalf("bind proxy pool failed: %v", err)
	}
}

// assertProxyBinding 断言渠道（缓存返回值 + DB 读回值）的代理模式与配置绑定。
func assertProxyBinding(t *testing.T, channelID int, wantMode model.ProxyUsageMode, wantConfigID *int) {
	t.Helper()
	var got model.Channel
	if err := db.GetDB().First(&got, channelID).Error; err != nil {
		t.Fatalf("reload from DB failed: %v", err)
	}
	if got.ProxyMode != wantMode {
		t.Fatalf("ProxyMode = %q, want %q", got.ProxyMode, wantMode)
	}
	if wantConfigID == nil {
		if got.ProxyConfigID != nil {
			t.Fatalf("ProxyConfigID = %d, want nil", *got.ProxyConfigID)
		}
		return
	}
	if got.ProxyConfigID == nil || *got.ProxyConfigID != *wantConfigID {
		t.Fatalf("ProxyConfigID = %v, want %d", got.ProxyConfigID, *wantConfigID)
	}
}

// TestUpdateLegacyProxyFieldsPreservePoolBinding 回归测试 issue #195：
// 已绑定代理池（proxy_mode=pool + proxy_config_id）的渠道，收到仅含旧字段
//（proxy 开关 / channel_proxy 文本）的更新请求时，绑定必须保留，
// 不得被 legacy 推导覆盖成 NULL 静默落库。
func TestUpdateLegacyProxyFieldsPreservePoolBinding(t *testing.T) {
	setupBatchGroupTest(t)
	seedChannel(t, 1, 1)

	configID := 3
	bindProxyPool(t, 1, configID)
	assertProxyBinding(t, 1, model.ProxyUsageModePool, &configID)

	// 模拟旧前端：仅切换 proxy 开关为 false（issue #195 复现步骤 6）
	proxyOff := false
	if _, err := Update(&model.ChannelUpdateRequest{ID: 1, Proxy: &proxyOff}, context.Background()); err != nil {
		t.Fatalf("legacy proxy-off Update returned error: %v", err)
	}
	assertProxyBinding(t, 1, model.ProxyUsageModePool, &configID)

	// 模拟旧前端：在渠道代理输入框输入再清空（提交空串）
	empty := ""
	if _, err := Update(&model.ChannelUpdateRequest{ID: 1, ChannelProxy: &empty}, context.Background()); err != nil {
		t.Fatalf("legacy channel-proxy-clear Update returned error: %v", err)
	}
	assertProxyBinding(t, 1, model.ProxyUsageModePool, &configID)

	// 模拟旧前端：proxy 开关重新打开且填入自定义地址，绑定同样保留
	proxyOn := true
	customURL := "http://127.0.0.1:7890"
	if _, err := Update(&model.ChannelUpdateRequest{ID: 1, Proxy: &proxyOn, ChannelProxy: &customURL}, context.Background()); err != nil {
		t.Fatalf("legacy custom-url Update returned error: %v", err)
	}
	assertProxyBinding(t, 1, model.ProxyUsageModePool, &configID)
}

// TestUpdateExplicitProxyModeStillUnbindsPool 显式传 proxy_mode 时用户意图明确，
// 仍可解除代理池绑定（direct 模式清空 proxy_config_id）。
func TestUpdateExplicitProxyModeStillUnbindsPool(t *testing.T) {
	setupBatchGroupTest(t)
	seedChannel(t, 1, 1)

	configID := 3
	bindProxyPool(t, 1, configID)

	directMode := model.ProxyUsageModeDirect
	if _, err := Update(&model.ChannelUpdateRequest{ID: 1, ProxyMode: &directMode}, context.Background()); err != nil {
		t.Fatalf("explicit direct Update returned error: %v", err)
	}
	assertProxyBinding(t, 1, model.ProxyUsageModeDirect, nil)
}

// TestUpdateLegacyDerivationStillWorksWithoutPoolBinding 未绑定代理池的渠道，
// legacy 字段推导行为保持不变（proxy=true 无地址 → system；proxy=false → direct）。
func TestUpdateLegacyDerivationStillWorksWithoutPoolBinding(t *testing.T) {
	setupBatchGroupTest(t)
	seedChannel(t, 1, 1)

	proxyOn := true
	if _, err := Update(&model.ChannelUpdateRequest{ID: 1, Proxy: &proxyOn}, context.Background()); err != nil {
		t.Fatalf("legacy proxy-on Update returned error: %v", err)
	}
	assertProxyBinding(t, 1, model.ProxyUsageModeSystem, nil)

	proxyOff := false
	if _, err := Update(&model.ChannelUpdateRequest{ID: 1, Proxy: &proxyOff}, context.Background()); err != nil {
		t.Fatalf("legacy proxy-off Update returned error: %v", err)
	}
	assertProxyBinding(t, 1, model.ProxyUsageModeDirect, nil)
}
