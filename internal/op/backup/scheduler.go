package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// WebDAVBackupConfig holds the WebDAV cloud backup configuration.
type WebDAVBackupConfig struct {
	Enabled       bool   `json:"enabled"`
	BaseURL       string `json:"base_url"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	RemotePath    string `json:"remote_path"`
	IntervalHours int    `json:"interval_hours"`
	IncludeStats  bool   `json:"include_stats"`
	IncludeLogs   bool   `json:"include_logs"`
	MaxBackups    int    `json:"max_backups"`
}

// GetWebDAVConfig reads the current WebDAV config from settings.
func GetWebDAVConfig() (*WebDAVBackupConfig, error) {
	raw, err := setting.GetString(model.SettingKeyWebDAVConfig)
	if err != nil {
		return nil, fmt.Errorf("get webdav config: %w", err)
	}
	var cfg WebDAVBackupConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("parse webdav config: %w", err)
	}
	return &cfg, nil
}

// SetWebDAVConfig persists a WebDAV config to settings.
func SetWebDAVConfig(cfg *WebDAVBackupConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal webdav config: %w", err)
	}
	return setting.SetString(model.SettingKeyWebDAVConfig, string(data))
}

// PerformWebDAVBackup exports the database and uploads it to the configured WebDAV server.
func PerformWebDAVBackup(ctx context.Context) error {
	cfg, err := GetWebDAVConfig()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	if !cfg.Enabled {
		return nil
	}
	if cfg.BaseURL == "" {
		return fmt.Errorf("webdav base URL is empty")
	}

	client := NewWebDAVClient(cfg.BaseURL, cfg.Username, cfg.Password)

	dump, err := ExportAll(ctx, cfg.IncludeLogs, cfg.IncludeStats)
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	data, err := json.Marshal(dump)
	if err != nil {
		return fmt.Errorf("marshal dump: %w", err)
	}

	filename := fmt.Sprintf("octopus-backup-%s.json", time.Now().UTC().Format("20060102-150405"))
	remotePath := strings.TrimSuffix(cfg.RemotePath, "/") + "/" + filename

	if err := client.Upload(remotePath, data); err != nil {
		return fmt.Errorf("upload %s: %w", remotePath, err)
	}

	log.Infof("webdav backup uploaded: %s (%d bytes)", remotePath, len(data))

	// Cleanup old backups
	if cfg.MaxBackups > 0 {
		if err := cleanupOldBackups(client, cfg.RemotePath, cfg.MaxBackups); err != nil {
			log.Warnf("webdav backup cleanup failed: %v", err)
		}
	}

	return nil
}

// RestoreFromWebDAV downloads a backup file from WebDAV and imports it.
func RestoreFromWebDAV(ctx context.Context, filename string) (*model.DBImportResult, error) {
	cfg, err := GetWebDAVConfig()
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("webdav base URL is empty")
	}

	client := NewWebDAVClient(cfg.BaseURL, cfg.Username, cfg.Password)

	remotePath := strings.TrimSuffix(cfg.RemotePath, "/") + "/" + filename
	data, err := client.Download(remotePath)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", remotePath, err)
	}

	var dump model.DBDump
	if err := json.Unmarshal(data, &dump); err != nil {
		return nil, fmt.Errorf("parse backup: %w", err)
	}

	result, err := ImportWithMode(ctx, &dump, model.ImportModeIncremental)
	if err != nil {
		return nil, fmt.Errorf("import: %w", err)
	}

	return result, nil
}

// ListWebDAVBackups returns available backup files from the remote WebDAV server.
func ListWebDAVBackups() ([]WebDAVFile, error) {
	cfg, err := GetWebDAVConfig()
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("webdav base URL is empty")
	}

	client := NewWebDAVClient(cfg.BaseURL, cfg.Username, cfg.Password)
	files, err := client.List(cfg.RemotePath)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	// Filter to only backup JSON files, exclude directories
	var backups []WebDAVFile
	for _, f := range files {
		if !f.IsDir && strings.HasSuffix(f.Name, ".json") {
			backups = append(backups, f)
		}
	}

	// Sort by name descending (newest first, since names contain timestamps)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Name > backups[j].Name
	})

	return backups, nil
}

// DeleteWebDAVBackup removes a specific backup file from the remote server.
func DeleteWebDAVBackup(filename string) error {
	cfg, err := GetWebDAVConfig()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	client := NewWebDAVClient(cfg.BaseURL, cfg.Username, cfg.Password)
	remotePath := strings.TrimSuffix(cfg.RemotePath, "/") + "/" + filename
	return client.Delete(remotePath)
}

// cleanupOldBackups removes old backup files, keeping only the newest maxBackups.
func cleanupOldBackups(client *WebDAVClient, remotePath string, maxBackups int) error {
	files, err := client.List(remotePath)
	if err != nil {
		return fmt.Errorf("list for cleanup: %w", err)
	}

	var backups []WebDAVFile
	for _, f := range files {
		if !f.IsDir && strings.HasSuffix(f.Name, ".json") {
			backups = append(backups, f)
		}
	}

	if len(backups) <= maxBackups {
		return nil
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Name > backups[j].Name
	})

	for _, f := range backups[maxBackups:] {
		if err := client.Delete(f.Path); err != nil {
			log.Warnf("failed to delete old backup %s: %v", f.Name, err)
		} else {
			log.Infof("deleted old webdav backup: %s", f.Name)
		}
	}

	return nil
}
