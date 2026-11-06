package channel

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	nodeop "github.com/lingyuins/octopus/internal/op/node"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

func setupNodeFeatureTest(t *testing.T) context.Context {
	t.Helper()
	setupBatchGroupTest(t)
	crypto.Init("octopus-test-encryption-key")
	cache := setting.GetCache()
	previous := cache.GetAll()
	oldDisconnect := nodeop.DisconnectAllFunc
	t.Cleanup(func() {
		cache.Clear()
		for key, value := range previous {
			cache.Set(key, value)
		}
		nodeop.DisconnectAllFunc = oldDisconnect
	})
	if err := setting.RefreshCache(context.Background()); err != nil {
		t.Fatal(err)
	}
	return context.Background()
}

func assertNodeFeatureChannel(t *testing.T, ctx context.Context, id int, enabled, paused bool) {
	t.Helper()
	cached, err := Get(id, ctx)
	if err != nil {
		t.Fatal(err)
	}
	var stored model.Channel
	if err := db.GetDB().First(&stored, id).Error; err != nil {
		t.Fatal(err)
	}
	for _, item := range []model.Channel{*cached, stored} {
		if item.Enabled != enabled || item.NodeFeaturePaused != paused {
			t.Fatalf("channel %d: enabled=%v paused=%v, want %v/%v", id, item.Enabled, item.NodeFeaturePaused, enabled, paused)
		}
	}
}

func TestNodeFeaturePausesAndRestoresOnlyPreviouslyEnabledChannels(t *testing.T) {
	ctx := setupNodeFeatureTest(t)
	n, err := nodeop.Create(ctx, "local", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	items := []model.Channel{
		{Name: "node-on", GroupID: 1, NodeID: n.Node.ID, Enabled: true},
		{Name: "node-off", GroupID: 1, NodeID: n.Node.ID},
		{Name: "http-on", GroupID: 1, Enabled: true},
		{Name: "http-off", GroupID: 1},
	}
	for i := range items {
		if err := Create(&items[i], ctx); err != nil {
			t.Fatal(err)
		}
	}
	disconnected := 0
	nodeop.DisconnectAllFunc = func() {
		disconnected++
		if nodeop.FeatureEnabled() || nodeop.ValidateCredential(ctx, n.Node.ID, n.Token) {
			t.Error("disconnect ran before publishing feature disable")
		}
		var stored model.Channel
		if err := db.GetDB().First(&stored, items[0].ID).Error; err != nil || stored.Enabled || !stored.NodeFeaturePaused {
			t.Errorf("disconnect ran before committing channel pause: %+v %v", stored, err)
		}
	}
	for range 2 {
		if err := SetNodesFeatureEnabled(ctx, false); err != nil {
			t.Fatal(err)
		}
		assertNodeFeatureChannel(t, ctx, items[0].ID, false, true)
		assertNodeFeatureChannel(t, ctx, items[1].ID, false, false)
		assertNodeFeatureChannel(t, ctx, items[2].ID, true, false)
		assertNodeFeatureChannel(t, ctx, items[3].ID, false, false)
	}
	if disconnected != 2 {
		t.Fatalf("disconnect count=%d", disconnected)
	}

	// Reopen the database and rebuild both caches, rather than keeping the marker
	// only in the objects produced by the disable request.
	sqlDB, err := db.GetDB().DB()
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct{ File string }
	if err := db.GetDB().Raw("PRAGMA database_list").Scan(&rows).Error; err != nil || len(rows) == 0 {
		t.Fatalf("database path: %+v %v", rows, err)
	}
	path := filepath.Clean(rows[0].File)
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	chCache.Clear()
	setting.GetCache().Clear()
	if err := db.InitDB("sqlite", path, false); err != nil {
		t.Fatal(err)
	}
	if err := setting.RefreshCache(ctx); err != nil {
		t.Fatal(err)
	}
	if err := RefreshCache(ctx); err != nil {
		t.Fatal(err)
	}
	assertNodeFeatureChannel(t, ctx, items[0].ID, false, true)
	for range 2 {
		if err := SetNodesFeatureEnabled(ctx, true); err != nil {
			t.Fatal(err)
		}
		assertNodeFeatureChannel(t, ctx, items[0].ID, true, false)
		assertNodeFeatureChannel(t, ctx, items[1].ID, false, false)
		assertNodeFeatureChannel(t, ctx, items[2].ID, true, false)
		assertNodeFeatureChannel(t, ctx, items[3].ID, false, false)
	}
	if !nodeop.ValidateCredential(ctx, n.Node.ID, n.Token) {
		t.Fatal("re-enabling lost the node credential")
	}
}

func TestNodeFeatureGuardsMutationsAndClearsObsoleteResumeMarkers(t *testing.T) {
	ctx := setupNodeFeatureTest(t)
	n, err := nodeop.Create(ctx, "local", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	items := []model.Channel{
		{Name: "manually-disabled", GroupID: 1, NodeID: n.Node.ID, Enabled: true},
		{Name: "detached", GroupID: 1, NodeID: n.Node.ID, Enabled: true},
		{Name: "deleted", GroupID: 1, NodeID: n.Node.ID, Enabled: true},
		{Name: "retained", GroupID: 1, NodeID: n.Node.ID, Enabled: true},
		{Name: "http", GroupID: 1, Enabled: true},
	}
	for i := range items {
		if err := Create(&items[i], ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := SetNodesFeatureEnabled(ctx, false); err != nil {
		t.Fatal(err)
	}
	if err := Enabled(items[0].ID, true, ctx); !errors.Is(err, nodeop.ErrFeatureDisabled) {
		t.Fatalf("enabled node while feature disabled: %v", err)
	}
	on := true
	if _, err := Update(&model.ChannelUpdateRequest{ID: items[0].ID, Enabled: &on}, ctx); !errors.Is(err, nodeop.ErrFeatureDisabled) {
		t.Fatalf("patch enabled node while feature disabled: %v", err)
	}
	if _, err := Update(&model.ChannelUpdateRequest{ID: items[4].ID, NodeID: &n.Node.ID}, ctx); !errors.Is(err, nodeop.ErrFeatureDisabled) {
		t.Fatalf("bound HTTP channel while feature disabled: %v", err)
	}
	created := model.Channel{Name: "blocked", GroupID: 1, NodeID: n.Node.ID, Enabled: true}
	if err := Create(&created, ctx); !errors.Is(err, nodeop.ErrFeatureDisabled) {
		t.Fatalf("created node channel while feature disabled: %v", err)
	}
	name := "renamed"
	if _, err := Update(&model.ChannelUpdateRequest{ID: items[3].ID, Name: &name, NodeID: &n.Node.ID}, ctx); err != nil {
		t.Fatalf("ordinary patch of paused channel: %v", err)
	}
	assertNodeFeatureChannel(t, ctx, items[3].ID, false, true)
	if err := Enabled(items[0].ID, false, ctx); err != nil {
		t.Fatal(err)
	}
	zero := 0
	urls := []model.BaseUrl{{URL: "https://example.com/v1"}}
	if _, err := Update(&model.ChannelUpdateRequest{ID: items[1].ID, NodeID: &zero, BaseUrls: &urls, KeysToAdd: []model.ChannelKeyAddRequest{{Enabled: true, ChannelKey: "http-key"}}}, ctx); err != nil {
		t.Fatal(err)
	}
	if err := Delete(items[2].ID, ctx); err != nil {
		t.Fatal(err)
	}
	if err := SetNodesFeatureEnabled(ctx, true); err != nil {
		t.Fatal(err)
	}
	assertNodeFeatureChannel(t, ctx, items[0].ID, false, false)
	assertNodeFeatureChannel(t, ctx, items[1].ID, false, false)
	assertNodeFeatureChannel(t, ctx, items[3].ID, true, false)
	assertNodeFeatureChannel(t, ctx, items[4].ID, true, false)
	if _, err := Get(items[2].ID, ctx); err == nil {
		t.Fatal("deleted channel was resurrected")
	}
}

func TestNodeFeatureSettingAndChannelChangesRollbackTogether(t *testing.T) {
	ctx := setupNodeFeatureTest(t)
	item := model.Channel{Name: "rollback", NodeID: 1, Enabled: true}
	if err := db.GetDB().Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	chCache.Set(item.ID, item)
	disconnected := false
	nodeop.DisconnectAllFunc = func() { disconnected = true }
	if err := db.GetDB().Exec("CREATE TRIGGER reject_node_pause BEFORE UPDATE ON channels BEGIN SELECT RAISE(ABORT, 'injected failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	if err := SetNodesFeatureEnabled(ctx, false); err == nil {
		t.Fatal("injected failure was ignored")
	}
	assertNodeFeatureChannel(t, ctx, item.ID, true, false)
	var stored model.Setting
	if err := db.GetDB().Where("key = ?", model.SettingKeyNodesFeatureEnabled).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Value != "true" || !nodeop.FeatureEnabled() || disconnected {
		t.Fatalf("partial toggle: setting=%q cache=%v disconnected=%v", stored.Value, nodeop.FeatureEnabled(), disconnected)
	}
}

func TestNodeFeatureConcurrentChannelEnablesCannotEscapeDisable(t *testing.T) {
	ctx := setupNodeFeatureTest(t)
	for i := range 8 {
		item := model.Channel{Name: fmt.Sprintf("concurrent-%d", i), NodeID: 1, Enabled: true}
		if err := db.GetDB().Create(&item).Error; err != nil {
			t.Fatal(err)
		}
		chCache.Set(item.ID, item)
	}
	for range 10 {
		if err := SetNodesFeatureEnabled(ctx, true); err != nil {
			t.Fatal(err)
		}
		var workers sync.WaitGroup
		for id := range chCache.GetAll() {
			workers.Add(1)
			go func(id int) {
				defer workers.Done()
				if err := Enabled(id, true, ctx); err != nil && !errors.Is(err, nodeop.ErrFeatureDisabled) {
					t.Errorf("enable: %v", err)
				}
			}(id)
		}
		if err := SetNodesFeatureEnabled(ctx, false); err != nil {
			t.Fatal(err)
		}
		workers.Wait()
		for id := range chCache.GetAll() {
			assertNodeFeatureChannel(t, ctx, id, false, true)
		}
	}
}
