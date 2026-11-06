package backup

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/nodetransport"
	channelop "github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/op/node"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

func TestNodeBackupCrossDatabaseAndLiveRestore(t *testing.T) {
	ctx := context.Background()
	if err := db.InitDB("sqlite", filepath.Join(t.TempDir(), "source.db"), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	created, err := node.Create(ctx, "edge", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	channel := model.Channel{Name: "bound", NodeID: created.Node.ID, NodeFeaturePaused: true}
	if err := db.GetDB().Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	dump, err := ExportAll(ctx, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(dump.Nodes) != 1 || dump.Nodes[0].TokenHash != "" {
		t.Fatal("export contains credential digest")
	}
	data, err := json.Marshal(dump)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), created.Token) || strings.Contains(string(data), "token_hash") {
		t.Fatal("backup leaked node credential")
	}
	for _, mode := range []string{model.ImportModeIncremental, model.ImportModeFull} {
		t.Run(mode, func(t *testing.T) {
			target, err := db.OpenStandalone("sqlite", filepath.Join(t.TempDir(), "target.db"), false)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { sql, _ := target.DB(); _ = sql.Close() })
			if err := db.Migrate(target); err != nil {
				t.Fatal(err)
			}
			var restored model.DBDump
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			// Even in-process imports and malicious DTOs must not restore secrets.
			restored.Nodes[0].TokenHash = "injected"
			restored.Nodes[0].Enabled = true
			if _, err := ImportWithModeToDB(ctx, target, &restored, mode); err != nil {
				t.Fatal(err)
			}
			var n model.Node
			if err := target.First(&n, created.Node.ID).Error; err != nil {
				t.Fatal(err)
			}
			if n.Enabled || n.TokenHash != "" || n.Name != "edge" {
				t.Fatal("restored node not safely disabled")
			}
			var ch model.Channel
			if err := target.First(&ch, channel.ID).Error; err != nil {
				t.Fatal(err)
			}
			if ch.NodeID != n.ID {
				t.Fatal("channel node pairing lost")
			}
			if ch.Enabled || !ch.NodeFeaturePaused {
				t.Fatal("backup lost the persistent node feature pause marker")
			}
			if _, err := ImportWithModeToDB(ctx, target, &restored, model.ImportModeIncremental); err == nil || !strings.Contains(err.Error(), "conflicts") {
				t.Fatalf("node ID collision accepted: %v", err)
			}
		})
	}
	old := node.NodeRevokedFunc
	t.Cleanup(func() { node.NodeRevokedFunc = old })
	revoked := false
	node.NodeRevokedFunc = func(id int) {
		if id == created.Node.ID {
			revoked = true
		}
		if node.ValidateCredential(ctx, id, created.Token) {
			t.Error("restore disconnected before committing revoked credential")
		}
	}
	if _, err := ImportWithMode(ctx, dump, model.ImportModeFull); err != nil {
		t.Fatal(err)
	}
	if !revoked || node.ValidateCredential(ctx, created.Node.ID, created.Token) {
		t.Fatal("live restore did not revoke old session and credential")
	}
}

func TestNodeBackupNormalizesUntrustedChannelTransport(t *testing.T) {
	ctx := context.Background()
	crypto.Init("octopus-test-encryption-key")
	for _, mode := range []string{model.ImportModeIncremental, model.ImportModeFull} {
		t.Run(mode, func(t *testing.T) {
			if err := db.InitDB("sqlite", filepath.Join(t.TempDir(), "restore.db"), false); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			dump := &model.DBDump{Nodes: []model.Node{{ID: 42, Name: "restored", Enabled: true}}, Channels: []model.Channel{{ID: 88, Name: "untrusted", NodeID: 42, BaseUrls: []model.BaseUrl{{URL: "https://arbitrary.example"}}, Keys: []model.ChannelKey{{ID: 123, ChannelKey: "embedded-secret"}}}}, ChannelKeys: []model.ChannelKey{{ID: 124, ChannelID: 88, ChannelKey: "table-secret"}, {ID: 125, ChannelID: 88, ChannelKey: "second-secret"}}}
			if _, err := ImportWithMode(ctx, dump, mode); err != nil {
				t.Fatal(err)
			}
			var restored model.Channel
			if err := db.GetDB().Preload("Keys").First(&restored, 88).Error; err != nil {
				t.Fatal(err)
			}
			if restored.NodeID != 42 || len(restored.BaseUrls) != 1 || restored.BaseUrls[0].URL != nodetransport.BaseURL || restored.ProxyMode != model.ProxyUsageModeDirect {
				t.Fatal("node transport was not normalized")
			}
			if len(restored.Keys) != 1 || !restored.Keys[0].Enabled {
				t.Fatal("node restore did not create one enabled placeholder")
			}
			key, err := crypto.Decrypt(restored.Keys[0].ChannelKey)
			if err != nil || key != channelop.NodeChannelKey {
				t.Fatal("node restore retained untrusted credential")
			}
		})
	}
}

func TestNodeBackupRejectsConflictingTransportAndChannelIdentity(t *testing.T) {
	ctx := context.Background()
	crypto.Init("octopus-test-encryption-key")
	if err := db.InitDB("sqlite", filepath.Join(t.TempDir(), "restore.db"), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	proxyID := 1
	for _, ch := range []model.Channel{{ID: 88, Name: "pool", NodeID: 42, PoolID: 1}, {ID: 88, Name: "proxy", NodeID: 42, Proxy: true}, {ID: 88, Name: "proxy-id", NodeID: 42, ProxyConfigID: &proxyID}} {
		dump := &model.DBDump{Nodes: []model.Node{{ID: 42, Name: "node"}}, Channels: []model.Channel{ch}}
		if _, err := ImportWithMode(ctx, dump, model.ImportModeIncremental); err == nil || !strings.Contains(err.Error(), "pool or proxy") {
			t.Fatalf("unsafe node transport accepted: %v", err)
		}
	}
	if err := db.GetDB().Create(&model.Channel{ID: 88, Name: "existing-http"}).Error; err != nil {
		t.Fatal(err)
	}
	dump := &model.DBDump{Nodes: []model.Node{{ID: 42, Name: "node"}}, Channels: []model.Channel{{ID: 88, Name: "node", NodeID: 42}}}
	if _, err := ImportWithMode(ctx, dump, model.ImportModeIncremental); err == nil || !strings.Contains(err.Error(), "channel ID conflicts") {
		t.Fatalf("node restored onto HTTP channel: %v", err)
	}
}

func TestNodeLiveRestoreReplacesOldCachedBindingBeforeUnlock(t *testing.T) {
	ctx := context.Background()
	crypto.Init("octopus-test-encryption-key")
	if err := db.InitDB("sqlite", filepath.Join(t.TempDir(), "cache.db"), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	old := model.Channel{ID: 15, Name: "old", NodeID: 1}
	if err := db.GetDB().Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	if err := channelop.RefreshCache(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := channelop.Get(old.ID, ctx); err != nil {
		t.Fatal(err)
	}
	dump := &model.DBDump{Nodes: []model.Node{{ID: 1, Name: "unrelated-restored-node"}}}
	if _, err := ImportWithMode(ctx, dump, model.ImportModeFull); err != nil {
		t.Fatal(err)
	}
	rotated, err := node.RotateToken(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	if _, err := node.Update(ctx, 1, nil, &enabled, nil); err != nil {
		t.Fatal(err)
	}
	if !node.ValidateCredential(ctx, 1, rotated.Token) {
		t.Fatal("new node could not authenticate")
	}
	if _, err := channelop.Get(old.ID, ctx); err == nil {
		t.Fatal("old cached channel retained reference to unrelated restored node")
	}
}

func TestNodeSQLiteSequenceDoesNotReuseDeletedOrRestoredIDs(t *testing.T) {
	ctx := context.Background()
	if err := db.InitDB("sqlite", filepath.Join(t.TempDir(), "ids.db"), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var ddl string
	if err := db.GetDB().Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'nodes'").Scan(&ddl).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToUpper(ddl), "AUTOINCREMENT") {
		t.Fatal("nodes table lacks non-reusing AUTOINCREMENT")
	}
	created, err := node.Create(ctx, "first", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	if err := node.Delete(ctx, created.Node.ID); err != nil {
		t.Fatal(err)
	}
	next, err := node.Create(ctx, "next", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	if next.Node.ID <= created.Node.ID {
		t.Fatal("deleted node ID reused")
	}
	dump := &model.DBDump{Nodes: []model.Node{{ID: 1000, Name: "imported"}}}
	if _, err := ImportWithMode(ctx, dump, model.ImportModeFull); err != nil {
		t.Fatal(err)
	}
	if err := node.Delete(ctx, 1000); err != nil {
		t.Fatal(err)
	}
	next, err = node.Create(ctx, "after-import", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	if next.Node.ID <= 1000 {
		t.Fatal("restored high node ID reused")
	}
}

func TestNodeBackupMissingTableAndReferenceSafety(t *testing.T) {
	ctx := context.Background()
	if err := db.InitDB("sqlite", filepath.Join(t.TempDir(), "legacy.db"), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.GetDB().Migrator().DropTable(&model.Node{}); err != nil {
		t.Fatal(err)
	}
	dump, err := ExportAll(ctx, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(dump.Nodes) != 0 {
		t.Fatal("legacy export invented nodes")
	}
	if _, err := ImportWithMode(ctx, dump, model.ImportModeFull); err != nil {
		t.Fatalf("legacy restore without nodes table: %v", err)
	}
	dump.Nodes = []model.Node{{ID: 10, Name: "edge", Enabled: true}}
	if _, err := ImportWithMode(ctx, dump, model.ImportModeIncremental); err == nil || !strings.Contains(err.Error(), "table missing") {
		t.Fatalf("missing target table accepted: %v", err)
	}
	dump.Nodes = nil
	dump.Channels = []model.Channel{{ID: 5, Name: "orphan", NodeID: 10}}
	if _, err := ImportWithMode(ctx, dump, model.ImportModeIncremental); err == nil || !strings.Contains(err.Error(), "reference missing") {
		t.Fatalf("missing backup identity accepted: %v", err)
	}
	if err := db.GetDB().AutoMigrate(&model.Node{}); err != nil {
		t.Fatal(err)
	}
	if err := db.GetDB().Create(&model.Channel{Name: "existing orphan", NodeID: 10}).Error; err != nil {
		t.Fatal(err)
	}
	dump.Channels = nil
	dump.Nodes = []model.Node{{ID: 10, Name: "unrelated"}}
	if _, err := ImportWithMode(ctx, dump, model.ImportModeIncremental); err == nil || !strings.Contains(err.Error(), "already reference") {
		t.Fatalf("orphan channel rebound: %v", err)
	}
}
