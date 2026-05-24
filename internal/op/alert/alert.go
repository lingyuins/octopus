package alert

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
)

var stateCache sync.Map // int(ruleID) -> model.AlertStateRecord
var stateMu sync.Mutex  // protects read-modify-write in StateSet

var timeNow = func() int64 { return time.Now().UnixMilli() }

func RuleList(ctx context.Context) ([]model.AlertRule, error) {
	rules := make([]model.AlertRule, 0)
	if err := db.GetDB().WithContext(ctx).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func RuleCreate(ctx context.Context, rule *model.AlertRule) error {
	return db.GetDB().WithContext(ctx).Create(rule).Error
}

func RuleUpdate(ctx context.Context, rule *model.AlertRule) error {
	if rule == nil || rule.ID == 0 {
		return fmt.Errorf("alert rule not found")
	}
	var count int64
	if err := db.GetDB().WithContext(ctx).Model(&model.AlertRule{}).Where("id = ?", rule.ID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("alert rule not found")
	}
	return db.GetDB().WithContext(ctx).Save(rule).Error
}

func RuleDelete(ctx context.Context, id int) error {
	res := db.GetDB().WithContext(ctx).Delete(&model.AlertRule{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("alert rule not found")
	}
	return nil
}

func NotifChannelList(ctx context.Context) ([]model.AlertNotifChannel, error) {
	channels := make([]model.AlertNotifChannel, 0)
	if err := db.GetDB().WithContext(ctx).Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

func NotifChannelCreate(ctx context.Context, ch *model.AlertNotifChannel) error {
	return db.GetDB().WithContext(ctx).Create(ch).Error
}

func NotifChannelUpdate(ctx context.Context, ch *model.AlertNotifChannel) error {
	if ch == nil || ch.ID == 0 {
		return fmt.Errorf("alert notification channel not found")
	}
	var count int64
	if err := db.GetDB().WithContext(ctx).Model(&model.AlertNotifChannel{}).Where("id = ?", ch.ID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("alert notification channel not found")
	}
	return db.GetDB().WithContext(ctx).Save(ch).Error
}

func NotifChannelDelete(ctx context.Context, id int) error {
	res := db.GetDB().WithContext(ctx).Delete(&model.AlertNotifChannel{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("alert notification channel not found")
	}
	return nil
}

func StateGet(ruleID int) model.AlertStateRecord {
	if v, ok := stateCache.Load(ruleID); ok {
		if record, ok := v.(model.AlertStateRecord); ok {
			return record
		}
	}
	return model.AlertStateRecord{RuleID: ruleID, State: model.AlertStateOK}
}

func StateSet(ruleID int, state model.AlertState) {
	stateMu.Lock()
	defer stateMu.Unlock()

	record := StateGet(ruleID)
	record.State = state
	now := timeNow()
	if state == model.AlertStateFiring {
		record.LastFiredAt = now
		record.FiredCount++
	} else if state == model.AlertStateResolved {
		record.LastResolvedAt = now
	}
	record.LastCheckedAt = now
	stateCache.Store(ruleID, record)
}

func HistoryList(ctx context.Context, limit int) ([]model.AlertHistory, error) {
	if limit <= 0 {
		limit = 100
	}
	var history []model.AlertHistory
	if err := db.GetDB().WithContext(ctx).Order("time DESC").Limit(limit).Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

func HistoryAdd(ctx context.Context, entry *model.AlertHistory) error {
	return db.GetDB().WithContext(ctx).Create(entry).Error
}
