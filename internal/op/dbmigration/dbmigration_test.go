package dbmigration

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
)

func TestMigrateCopiesCoreDataAndSkipsLogsStatsByDefault(t *testing.T) {
	sourceDSN := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()+"-source")
	if err := db.InitDB("sqlite", sourceDSN, false); err != nil {
		t.Fatalf("init source db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := op.InitCache(); err != nil {
		t.Fatalf("init cache: %v", err)
	}
	if err := op.UserBootstrapCreate("admin", "super-secret-123"); err != nil {
		t.Fatalf("bootstrap user: %v", err)
	}
	if err := db.GetDB().Create(&model.RelayLog{Time: 1, ChannelName: "log-channel"}).Error; err != nil {
		t.Fatalf("seed relay log: %v", err)
	}
	if err := db.GetDB().Create(&model.StatsDaily{Date: "2026-05-26"}).Error; err != nil {
		t.Fatalf("seed stats daily: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "target.db")
	var savedType, savedPath string
	restore := SetSaveDatabaseConfigFuncForTest(func(dbType, path string) error {
		savedType = dbType
		savedPath = path
		return nil
	})
	defer restore()

	result, err := Migrate(context.Background(), model.DatabaseMigrationRequest{
		Type: "sqlite",
		Path: targetPath,
	})
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if !result.RestartNeeded || result.Type != "sqlite" || result.Path != targetPath {
		t.Fatalf("unexpected result: %+v", result)
	}
	if savedType != "sqlite" || savedPath != targetPath {
		t.Fatalf("saved config = (%q, %q), want (%q, %q)", savedType, savedPath, "sqlite", targetPath)
	}

	target, err := db.OpenStandalone("sqlite", targetPath, false)
	if err != nil {
		t.Fatalf("open target: %v", err)
	}
	sqlDB, err := target.DB()
	if err != nil {
		t.Fatalf("target DB(): %v", err)
	}
	defer sqlDB.Close()

	var users int64
	if err := target.Model(&model.User{}).Count(&users).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if users != 1 {
		t.Fatalf("users = %d, want 1", users)
	}
	var relayLogs int64
	if err := target.Model(&model.RelayLog{}).Count(&relayLogs).Error; err != nil {
		t.Fatalf("count relay logs: %v", err)
	}
	if relayLogs != 0 {
		t.Fatalf("relay logs = %d, want 0", relayLogs)
	}
	var statsDaily int64
	if err := target.Model(&model.StatsDaily{}).Count(&statsDaily).Error; err != nil {
		t.Fatalf("count stats daily: %v", err)
	}
	if statsDaily != 0 {
		t.Fatalf("stats daily = %d, want 0", statsDaily)
	}
}

func TestValidateRequestNormalizesPostgreSQL(t *testing.T) {
	got, err := ValidateRequest(model.DatabaseMigrationRequest{Type: "postgresql", Path: "postgres://example"})
	if err != nil {
		t.Fatalf("ValidateRequest() error = %v", err)
	}
	if got.Type != "postgres" {
		t.Fatalf("Type = %q, want postgres", got.Type)
	}
}
