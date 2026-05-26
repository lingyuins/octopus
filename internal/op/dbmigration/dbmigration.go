package dbmigration

import (
	"context"
	"fmt"
	"strings"

	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/backup"
)

type SaveDatabaseConfigFunc func(dbType, path string) error

var saveDatabaseConfig SaveDatabaseConfigFunc = conf.SaveDatabaseConfig

func SetSaveDatabaseConfigFuncForTest(fn SaveDatabaseConfigFunc) func() {
	old := saveDatabaseConfig
	saveDatabaseConfig = fn
	return func() { saveDatabaseConfig = old }
}

func ValidateRequest(req model.DatabaseMigrationRequest) (model.DatabaseMigrationRequest, error) {
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	req.Path = strings.TrimSpace(req.Path)
	if req.Type == "postgresql" {
		req.Type = "postgres"
	}
	if req.Type != "sqlite" && req.Type != "mysql" && req.Type != "postgres" {
		return req, fmt.Errorf("unsupported database type: %s", req.Type)
	}
	if req.Path == "" {
		return req, fmt.Errorf("database path is required")
	}
	return req, nil
}

func TestConnection(ctx context.Context, req model.DatabaseMigrationRequest) error {
	req, err := ValidateRequest(req)
	if err != nil {
		return err
	}
	target, err := db.OpenStandalone(req.Type, req.Path, false)
	if err != nil {
		return err
	}
	sqlDB, err := target.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	return sqlDB.PingContext(ctx)
}

func Migrate(ctx context.Context, req model.DatabaseMigrationRequest) (*model.DatabaseMigrationResult, error) {
	req, err := ValidateRequest(req)
	if err != nil {
		return nil, err
	}
	dump, err := backup.ExportAll(ctx, req.IncludeLogs, req.IncludeStats)
	if err != nil {
		return nil, fmt.Errorf("export current database: %w", err)
	}

	target, err := db.OpenStandalone(req.Type, req.Path, false)
	if err != nil {
		return nil, fmt.Errorf("open target database: %w", err)
	}
	sqlDB, err := target.DB()
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	if err := db.Migrate(target); err != nil {
		return nil, fmt.Errorf("migrate target schema: %w", err)
	}
	importResult, err := backup.ImportWithModeToDB(ctx, target, dump, model.ImportModeFull)
	if err != nil {
		return nil, fmt.Errorf("import target database: %w", err)
	}
	if err := saveDatabaseConfig(req.Type, req.Path); err != nil {
		return nil, err
	}
	return &model.DatabaseMigrationResult{
		Type:          req.Type,
		Path:          req.Path,
		IncludeLogs:   req.IncludeLogs,
		IncludeStats:  req.IncludeStats,
		RestartNeeded: true,
		ImportResult:  *importResult,
	}, nil
}
