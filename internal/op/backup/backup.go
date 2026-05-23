package backup

import (
	"context"
	"fmt"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const dbDumpVersion = 1
const maxRelayLogsExport = 500_000
const maxAuditLogsExport = 500_000

func ExportAll(ctx context.Context, includeLogs, includeStats bool) (*model.DBDump, error) {
	conn := db.GetDB().WithContext(ctx)

	d := &model.DBDump{
		Version:      dbDumpVersion,
		ExportedAt:   time.Now().UTC(),
		IncludeLogs:  includeLogs,
		IncludeStats: includeStats,
	}

	// Core tables
	if err := conn.Find(&d.Channels).Error; err != nil {
		return nil, fmt.Errorf("export channels: %w", err)
	}
	if err := conn.Find(&d.ChannelKeys).Error; err != nil {
		return nil, fmt.Errorf("export channel_keys: %w", err)
	}
	if err := conn.Find(&d.ChannelGroups).Error; err != nil {
		return nil, fmt.Errorf("export channel_groups: %w", err)
	}
	if err := conn.Find(&d.Groups).Error; err != nil {
		return nil, fmt.Errorf("export groups: %w", err)
	}
	if err := conn.Find(&d.GroupItems).Error; err != nil {
		return nil, fmt.Errorf("export group_items: %w", err)
	}
	if err := conn.Find(&d.LLMInfos).Error; err != nil {
		return nil, fmt.Errorf("export llm_infos: %w", err)
	}
	if err := conn.Find(&d.APIKeys).Error; err != nil {
		return nil, fmt.Errorf("export api_keys: %w", err)
	}
	if err := conn.Find(&d.Users).Error; err != nil {
		return nil, fmt.Errorf("export users: %w", err)
	}
	if err := conn.Find(&d.Settings).Error; err != nil {
		return nil, fmt.Errorf("export settings: %w", err)
	}

	// Alert tables
	if err := conn.Find(&d.AlertRules).Error; err != nil {
		return nil, fmt.Errorf("export alert_rules: %w", err)
	}
	if err := conn.Find(&d.AlertNotifChannels).Error; err != nil {
		return nil, fmt.Errorf("export alert_notif_channels: %w", err)
	}
	if err := conn.Find(&d.AlertStateRecords).Error; err != nil {
		return nil, fmt.Errorf("export alert_state_records: %w", err)
	}
	if err := conn.Find(&d.AlertHistory).Error; err != nil {
		return nil, fmt.Errorf("export alert_history: %w", err)
	}

	// Audit & runtime
	if err := conn.Order("id DESC").Limit(maxAuditLogsExport).Find(&d.AuditLogs).Error; err != nil {
		return nil, fmt.Errorf("export audit_logs: %w", err)
	}
	if err := conn.Find(&d.RuntimeStates).Error; err != nil {
		return nil, fmt.Errorf("export runtime_states: %w", err)
	}

	if includeStats {
		if err := conn.Find(&d.StatsTotal).Error; err != nil {
			return nil, fmt.Errorf("export stats_total: %w", err)
		}
		if err := conn.Find(&d.StatsDaily).Error; err != nil {
			return nil, fmt.Errorf("export stats_daily: %w", err)
		}
		if err := conn.Find(&d.StatsHourly).Error; err != nil {
			return nil, fmt.Errorf("export stats_hourly: %w", err)
		}
		if err := conn.Find(&d.StatsModel).Error; err != nil {
			return nil, fmt.Errorf("export stats_model: %w", err)
		}
		if err := conn.Find(&d.StatsChannel).Error; err != nil {
			return nil, fmt.Errorf("export stats_channel: %w", err)
		}
		if err := conn.Find(&d.StatsAPIKey).Error; err != nil {
			return nil, fmt.Errorf("export stats_api_key: %w", err)
		}
	}

	if includeLogs {
		if err := conn.Order("id DESC").Limit(maxRelayLogsExport).Find(&d.RelayLogs).Error; err != nil {
			return nil, fmt.Errorf("export relay_logs: %w", err)
		}
	}

	return d, nil
}

type importConfig struct {
	conn    *gorm.DB
	res     *model.DBImportResult
	version int // dump version, 0 for old backward-compat
	isFull  bool
}

func appendStep(res *model.DBImportResult, table, mode string, rows int64, err error) {
	step := model.DBImportStep{Table: table, Mode: mode, RowsAffected: rows, OK: err == nil}
	if err != nil {
		step.Error = err.Error()
	}
	res.Progress = append(res.Progress, step)
}

func (c *importConfig) doNothing(table string, rows []any) error {
	if len(rows) == 0 {
		return nil
	}
	result := c.conn.Table(table).Clauses(clause.OnConflict{DoNothing: true}).Create(rows)
	appendStep(c.res, table, "insert", result.RowsAffected, result.Error)
	if result.Error != nil {
		return fmt.Errorf("%s: %w", table, result.Error)
	}
	return nil
}

func (c *importConfig) upsertAll(table string, rows []any, conflictColumns []clause.Column) error {
	if len(rows) == 0 {
		return nil
	}
	result := c.conn.Table(table).Clauses(clause.OnConflict{
		Columns:   conflictColumns,
		UpdateAll: true,
	}).Create(rows)
	appendStep(c.res, table, "upsert", result.RowsAffected, result.Error)
	if result.Error != nil {
		return fmt.Errorf("%s: %w", table, result.Error)
	}
	return nil
}

func (c *importConfig) upsertSettings(rows []model.Setting) error {
	if len(rows) == 0 {
		return nil
	}
	result := c.conn.Table("settings").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&rows)
	appendStep(c.res, "settings", "upsert", result.RowsAffected, result.Error)
	if result.Error != nil {
		return fmt.Errorf("settings: %w", result.Error)
	}
	return nil
}

func (c *importConfig) deleteAll(table string) error {
	result := c.conn.Exec(fmt.Sprintf("DELETE FROM %s", table))
	appendStep(c.res, table, "delete", result.RowsAffected, result.Error)
	return result.Error
}

func ImportWithMode(ctx context.Context, dump *model.DBDump, mode string) (*model.DBImportResult, error) {
	if dump == nil {
		return nil, fmt.Errorf("empty dump")
	}
	isFull := mode == model.ImportModeFull
	res := &model.DBImportResult{RowsAffected: map[string]int64{}}
	cfg := &importConfig{conn: db.GetDB().WithContext(ctx), res: res, isFull: isFull, version: dump.Version}

	err := cfg.conn.Transaction(func(tx *gorm.DB) error {
		cfg.conn = tx

		if isFull {
			// Delete in reverse dependency order to avoid FK violations
			deleteOrder := []string{
				"relay_logs", "stats_api_key", "stats_channel", "stats_model",
				"stats_hourly", "stats_daily", "stats_total",
				"group_items", "group_channel_items", "groups",
				"alert_history", "alert_state_records", "alert_rules", "alert_notif_channels",
				"audit_logs", "runtime_states",
				"api_keys", "users", "channel_keys", "channel_groups", "channels",
				"llm_infos", "settings",
			}
			for _, table := range deleteOrder {
				if err := cfg.deleteAll(table); err != nil {
					return fmt.Errorf("full import: delete %s: %w", table, err)
				}
			}
		}

		// Import channels / keys / groups / items — skip existing
		if err := cfg.doNothing("channels", toAny(dump.Channels)); err != nil {
			return err
		}
		if err := cfg.doNothing("channel_keys", toAny(dump.ChannelKeys)); err != nil {
			return err
		}
		if err := cfg.doNothing("channel_groups", toAny(dump.ChannelGroups)); err != nil {
			return err
		}
		if err := cfg.doNothing("groups", toAny(dump.Groups)); err != nil {
			return err
		}
		if err := cfg.doNothing("group_items", toAny(dump.GroupItems)); err != nil {
			return err
		}

		// LLM prices — upsert by name
		if err := cfg.upsertAll("llm_infos", toAny(dump.LLMInfos), []clause.Column{{Name: "name"}}); err != nil {
			return err
		}

		// API keys — skip existing
		if err := cfg.doNothing("api_keys", toAny(dump.APIKeys)); err != nil {
			return err
		}

		// Users — skip existing (backward compat: might be nil in old dumps)
		if len(dump.Users) > 0 {
			if err := cfg.doNothing("users", toAny(dump.Users)); err != nil {
				return err
			}
		}

		// Settings — upsert by key
		if err := cfg.upsertSettings(dump.Settings); err != nil {
			return err
		}

		// Alerts — skip existing
		if len(dump.AlertRules) > 0 {
			if err := cfg.doNothing("alert_rules", toAny(dump.AlertRules)); err != nil {
				return err
			}
		}
		if len(dump.AlertNotifChannels) > 0 {
			if err := cfg.doNothing("alert_notif_channels", toAny(dump.AlertNotifChannels)); err != nil {
				return err
			}
		}
		if len(dump.AlertStateRecords) > 0 {
			if err := cfg.doNothing("alert_state_records", toAny(dump.AlertStateRecords)); err != nil {
				return err
			}
		}
		if len(dump.AlertHistory) > 0 {
			if err := cfg.doNothing("alert_history", toAny(dump.AlertHistory)); err != nil {
				return err
			}
		}

		// Audit & runtime — skip existing
		if len(dump.AuditLogs) > 0 {
			if err := cfg.doNothing("audit_logs", toAny(dump.AuditLogs)); err != nil {
				return err
			}
		}
		if len(dump.RuntimeStates) > 0 {
			if err := cfg.doNothing("runtime_states", toAny(dump.RuntimeStates)); err != nil {
				return err
			}
		}

		// Stats
		if dump.IncludeStats {
			if err := cfg.upsertAll("stats_total", toAny(dump.StatsTotal), []clause.Column{{Name: "id"}}); err != nil {
				return err
			}
			if err := cfg.upsertAll("stats_daily", toAny(dump.StatsDaily), []clause.Column{{Name: "date"}}); err != nil {
				return err
			}
			if err := cfg.upsertAll("stats_hourly", toAny(dump.StatsHourly), []clause.Column{{Name: "hour"}}); err != nil {
				return err
			}
			if err := cfg.upsertAll("stats_model", toAny(dump.StatsModel), []clause.Column{{Name: "id"}}); err != nil {
				return err
			}
			if err := cfg.upsertAll("stats_channel", toAny(dump.StatsChannel), []clause.Column{{Name: "channel_id"}}); err != nil {
				return err
			}
			if err := cfg.upsertAll("stats_api_key", toAny(dump.StatsAPIKey), []clause.Column{{Name: "api_key_id"}}); err != nil {
				return err
			}
		}

		// Relay logs
		if dump.IncludeLogs {
			if err := cfg.doNothing("relay_logs", toAny(dump.RelayLogs)); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Summarize rows affected from progress
	for _, step := range res.Progress {
		res.RowsAffected[step.Table] += step.RowsAffected
	}
	return res, nil
}

func toAny[T any](slice []T) []any {
	out := make([]any, len(slice))
	for i := range slice {
		out[i] = &slice[i]
	}
	return out
}

// ImportIncremental is the backward-compatible wrapper.
func ImportIncremental(ctx context.Context, dump *model.DBDump) (*model.DBImportResult, error) {
	return ImportWithMode(ctx, dump, model.ImportModeIncremental)
}
