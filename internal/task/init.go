package task

import (
	"context"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/backup"
	"github.com/lingyuins/octopus/internal/op/relaylog"
	"github.com/lingyuins/octopus/internal/op/remotesite"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/op/stats"
	"github.com/lingyuins/octopus/internal/price"
	"github.com/lingyuins/octopus/internal/relay/balancer"
	"github.com/lingyuins/octopus/internal/utils/log"
)

const (
	TaskPriceUpdate       = "price_update"
	TaskStatsSave         = "stats_save"
	TaskRuntimeState      = "runtime_state_save"
	TaskRelayLogSave      = "relay_log_save"
	TaskSyncLLM           = "sync_llm"
	TaskCleanLLM          = "clean_llm"
	TaskBaseUrlDelay      = "base_url_delay"
	TaskBalanceCapture    = "hub_balance_capture"
	TaskAutoCheckIn       = "hub_auto_checkin"
	TaskAnnouncementFetch = "hub_announcement_fetch"
	TaskUsageHistorySync  = "hub_usage_history_sync"
	TaskWebDAVBackup      = "webdav_backup"
)

func Init() {
	if db.IsSQLite() {
		db.StartSerialWriter(context.Background())
	}
	relaylog.StartFlushWorker(context.Background())
	priceUpdateIntervalHours, err := setting.GetInt(model.SettingKeyModelInfoUpdateInterval)
	if err != nil {
		log.Errorf("failed to get model info update interval: %v", err)
	} else {
		priceUpdateInterval := time.Duration(priceUpdateIntervalHours) * time.Hour
		Register(string(model.SettingKeyModelInfoUpdateInterval), priceUpdateInterval, true, func() {
			if err := price.UpdateLLMPrice(context.Background()); err != nil {
				log.Warnf("failed to update price info: %v", err)
			}
		})
	}

	Register(TaskBaseUrlDelay, 1*time.Hour, true, ChannelBaseUrlDelayTask)

	syncLLMIntervalHours, err := setting.GetInt(model.SettingKeySyncLLMInterval)
	if err != nil {
		log.Warnf("failed to get sync LLM interval: %v", err)
	} else {
		syncLLMInterval := time.Duration(syncLLMIntervalHours) * time.Hour
		Register(string(model.SettingKeySyncLLMInterval), syncLLMInterval, true, SyncModelsTask)
	}

	statsSaveIntervalMinutes, err := setting.GetInt(model.SettingKeyStatsSaveInterval)
	if err != nil {
		log.Warnf("failed to get stats save interval: %v", err)
	} else {
		statsSaveInterval := time.Duration(statsSaveIntervalMinutes) * time.Minute
		if db.IsSQLite() {
			Register(TaskStatsSave, statsSaveInterval, false, func() {
				db.EnqueueWrite(db.WriteJob{Name: "stats_save", Fn: func(_ context.Context) error {
					stats.SaveDBTask()
					return nil
				}})
			})
			Register(TaskRuntimeState, statsSaveInterval, false, func() {
				db.EnqueueWrite(db.WriteJob{Name: "runtime_state_save", Fn: func(_ context.Context) error {
					balancer.RuntimeStateSaveDBTask()
					return nil
				}})
			})
		} else {
			Register(TaskStatsSave, statsSaveInterval, false, stats.SaveDBTask)
			Register(TaskRuntimeState, statsSaveInterval, false, balancer.RuntimeStateSaveDBTask)
		}
	}

	Register(TaskRelayLogSave, 10*time.Minute, false, func() {
		if db.IsSQLite() {
			db.EnqueueWrite(db.WriteJob{Name: "relay_log_save", Fn: func(_ context.Context) error {
				return relaylog.RelayLogSaveDBTask(context.Background())
			}})
		} else {
			if err := relaylog.RelayLogSaveDBTask(context.Background()); err != nil {
				log.Warnf("relay log save db task failed: %v", err)
			}
		}
	})

	Register(TaskAlertEvaluate, 60*time.Second, false, EvaluateAlertRules)

	// Hub: capture balance snapshots every 6 hours
	Register(TaskBalanceCapture, 6*time.Hour, false, func() {
		n := remotesite.CaptureAllBalanceSnapshots(context.Background())
		if n > 0 {
			log.Infof("captured balance snapshots for %d remote sites", n)
		}
	})

	// Hub: auto check-in daily at task tick (every 12 hours; the check-in logic is idempotent per day)
	Register(TaskAutoCheckIn, 12*time.Hour, false, func() {
		records := remotesite.ExecuteCheckInAll(context.Background())
		if len(records) > 0 {
			log.Infof("auto check-in completed for %d remote sites", len(records))
		}
	})

	// Hub: fetch announcements every 4 hours
	Register(TaskAnnouncementFetch, 4*time.Hour, false, func() {
		n := remotesite.FetchAllAnnouncements(context.Background())
		if n > 0 {
			log.Infof("fetched announcements for %d remote sites", n)
		}
	})

	// Hub: sync usage history every 6 hours
	Register(TaskUsageHistorySync, 6*time.Hour, false, func() {
		n := remotesite.SyncAllUsageHistory(context.Background())
		if n > 0 {
			log.Infof("synced %d usage history records", n)
		}
	})

	// WebDAV cloud backup every 6 hours
	Register(TaskWebDAVBackup, 6*time.Hour, false, func() {
		if err := backup.PerformWebDAVBackup(context.Background()); err != nil {
			log.Warnf("webdav backup failed: %v", err)
		}
	})

	// Site sync task
	siteSyncIntervalHours, err := setting.GetInt(model.SettingKeySiteSyncInterval)
	if err != nil {
		log.Warnf("failed to get site sync interval: %v", err)
	} else {
		siteSyncInterval := time.Duration(siteSyncIntervalHours) * time.Hour
		Register(string(model.SettingKeySiteSyncInterval), siteSyncInterval, true, SiteSyncTask)
	}

	// Site checkin task
	siteCheckinIntervalHours, err := setting.GetInt(model.SettingKeySiteCheckinInterval)
	if err != nil {
		log.Warnf("failed to get site checkin interval: %v", err)
	} else {
		siteCheckinInterval := time.Duration(siteCheckinIntervalHours) * time.Hour
		Register(string(model.SettingKeySiteCheckinInterval), siteCheckinInterval, true, SiteCheckinTask)
	}
}
