package op

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func initChannelGroupTestDB(t *testing.T) context.Context {
	t.Helper()

	ctx := context.Background()
	testName := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", testName)

	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	if err := InitCache(); err != nil {
		t.Fatalf("init cache: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	return ctx
}

func TestChannelGroupsMigrationBackfillsExistingChannels(t *testing.T) {
    dbPath := filepath.Join(t.TempDir(), "legacy-octopus.db")
    legacyDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
    if err != nil {
        t.Fatalf("open legacy db: %v", err)
    }

    createChannelGroupSQL := `
CREATE TABLE channel_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    is_default NUMERIC NOT NULL DEFAULT 0,
    created_at INTEGER,
    updated_at INTEGER
)`
    if err := legacyDB.Exec(createChannelGroupSQL).Error; err != nil {
        t.Fatalf("create channel_groups table: %v", err)
    }

    createChannelSQL := `
CREATE TABLE channels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    group_id INTEGER NOT NULL DEFAULT 0,
    type INTEGER,
    enabled NUMERIC DEFAULT 1,
    base_urls TEXT,
    model TEXT,
    custom_model TEXT,
    proxy NUMERIC DEFAULT 0,
    auto_sync NUMERIC DEFAULT 0,
    auto_group INTEGER DEFAULT 0,
    custom_header TEXT,
    param_override TEXT,
    channel_proxy TEXT,
    request_rewrite TEXT,
    match_regex TEXT
)`
    if err := legacyDB.Exec(createChannelSQL).Error; err != nil {
        t.Fatalf("create channels table: %v", err)
    }
    if err := legacyDB.Exec(`INSERT INTO channels (name, group_id, type, enabled, base_urls, model, custom_model, proxy, auto_sync, auto_group, custom_header) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        "legacy-channel", 0, 0, true, "[]", "gpt-4o", "", false, false, 0, "[]").Error; err != nil {
        t.Fatalf("insert legacy channel: %v", err)
    }

    var defaultGroup model.ChannelGroup
    defaultGroup = model.ChannelGroup{Name: model.DefaultChannelGroupName, IsDefault: true}
    if err := legacyDB.Create(&defaultGroup).Error; err != nil {
        t.Fatalf("create default group: %v", err)
    }
    if err := legacyDB.Model(&model.Channel{}).Where("group_id IS NULL OR group_id = 0").Update("group_id", defaultGroup.ID).Error; err != nil {
        t.Fatalf("backfill channels.group_id: %v", err)
    }

    sqlDB, err := legacyDB.DB()
    if err != nil {
        t.Fatalf("legacy sql db: %v", err)
    }
    _ = sqlDB.Close()

    if err := db.InitDB("sqlite", dbPath, false); err != nil {
        t.Fatalf("init migrated db: %v", err)
    }
    if err := InitCache(); err != nil {
        t.Fatalf("init cache: %v", err)
    }
    t.Cleanup(func() {
        _ = db.Close()
    })

    groups, err := ChannelGroupList(context.Background())
    if err != nil {
        t.Fatalf("list channel groups: %v", err)
    }
    if len(groups) != 1 {
        t.Fatalf("len(groups) = %d, want 1", len(groups))
    }
    if !groups[0].IsDefault {
        t.Fatalf("expected default group, got %+v", groups[0])
    }

    channel, err := ChannelGet(1, context.Background())
    if err != nil {
        t.Fatalf("get channel: %v", err)
    }
    if channel.GroupID != groups[0].ID {
        t.Fatalf("channel.GroupID = %d, want %d", channel.GroupID, groups[0].ID)
    }
}

func TestChannelCreateUsesDefaultGroupWhenGroupIDMissing(t *testing.T) {
	ctx := initChannelGroupTestDB(t)

	defaultGroupID, err := ChannelGroupDefaultID(ctx)
	if err != nil {
		t.Fatalf("default group id: %v", err)
	}

	channel := &model.Channel{
		Name:      "default-group-channel",
		Type:      0,
		Enabled:   true,
		BaseUrls:  []model.BaseUrl{{URL: "https://example.com", Delay: 0}},
		Model:     "gpt-4o",
		AutoGroup: model.AutoGroupTypeNone,
		Keys:      []model.ChannelKey{{Enabled: true, ChannelKey: "sk-test"}},
	}
	if err := ChannelCreate(channel, ctx); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if channel.GroupID != defaultGroupID {
		t.Fatalf("channel.GroupID = %d, want %d", channel.GroupID, defaultGroupID)
	}
}

func TestChannelCreateAndUpdateRejectUnknownGroupID(t *testing.T) {
	ctx := initChannelGroupTestDB(t)

	channel := &model.Channel{
		Name:      "invalid-group-create",
		GroupID:   9999,
		Type:      0,
		Enabled:   true,
		BaseUrls:  []model.BaseUrl{{URL: "https://example.com", Delay: 0}},
		Model:     "gpt-4o",
		AutoGroup: model.AutoGroupTypeNone,
		Keys:      []model.ChannelKey{{Enabled: true, ChannelKey: "sk-test"}},
	}
	if err := ChannelCreate(channel, ctx); err == nil || !strings.Contains(err.Error(), "channel group not found") {
		t.Fatalf("expected channel group not found on create, got %v", err)
	}

	validChannel := &model.Channel{
		Name:      "valid-group-channel",
		Type:      0,
		Enabled:   true,
		BaseUrls:  []model.BaseUrl{{URL: "https://example.com", Delay: 0}},
		Model:     "gpt-4o",
		AutoGroup: model.AutoGroupTypeNone,
		Keys:      []model.ChannelKey{{Enabled: true, ChannelKey: "sk-test-2"}},
	}
	if err := ChannelCreate(validChannel, ctx); err != nil {
		t.Fatalf("create valid channel: %v", err)
	}

	invalidGroupID := 9999
	_, err := ChannelUpdate(&model.ChannelUpdateRequest{ID: validChannel.ID, GroupID: &invalidGroupID}, ctx)
	if err == nil || !strings.Contains(err.Error(), "channel group not found") {
		t.Fatalf("expected channel group not found on update, got %v", err)
	}
}

func TestChannelGroupDeleteRules(t *testing.T) {
	ctx := initChannelGroupTestDB(t)

	defaultGroupID, err := ChannelGroupDefaultID(ctx)
	if err != nil {
		t.Fatalf("default group id: %v", err)
	}
	if err := ChannelGroupDelete(defaultGroupID, ctx); err == nil || !strings.Contains(err.Error(), "default channel group cannot be deleted") {
		t.Fatalf("expected default group delete error, got %v", err)
	}

	occupiedGroup, err := ChannelGroupCreate("Occupied", ctx)
	if err != nil {
		t.Fatalf("create occupied group: %v", err)
	}
	channel := &model.Channel{
		Name:      "occupied-group-channel",
		GroupID:   occupiedGroup.ID,
		Type:      0,
		Enabled:   true,
		BaseUrls:  []model.BaseUrl{{URL: "https://example.com", Delay: 0}},
		Model:     "gpt-4o",
		AutoGroup: model.AutoGroupTypeNone,
		Keys:      []model.ChannelKey{{Enabled: true, ChannelKey: "sk-occupied"}},
	}
	if err := ChannelCreate(channel, ctx); err != nil {
		t.Fatalf("create occupied channel: %v", err)
	}
	if err := ChannelGroupDelete(occupiedGroup.ID, ctx); err == nil || !strings.Contains(err.Error(), "channel group is not empty") {
		t.Fatalf("expected not-empty delete error, got %v", err)
	}

	emptyGroup, err := ChannelGroupCreate("Empty", ctx)
	if err != nil {
		t.Fatalf("create empty group: %v", err)
	}
	if err := ChannelGroupDelete(emptyGroup.ID, ctx); err != nil {
		t.Fatalf("delete empty group: %v", err)
	}
	if _, err := ChannelGroupGet(emptyGroup.ID, ctx); err == nil || !strings.Contains(err.Error(), "channel group not found") {
		t.Fatalf("expected deleted group to be missing, got %v", err)
	}
}
