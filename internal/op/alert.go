package op

import (
	"context"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/alert"
)

// Deprecated: Use alert.RuleList from internal/op/alert instead.
func AlertRuleList(ctx context.Context) ([]model.AlertRule, error) { return alert.RuleList(ctx) }

// Deprecated: Use alert.RuleCreate from internal/op/alert instead.
func AlertRuleCreate(ctx context.Context, rule *model.AlertRule) error { return alert.RuleCreate(ctx, rule) }

// Deprecated: Use alert.RuleUpdate from internal/op/alert instead.
func AlertRuleUpdate(ctx context.Context, rule *model.AlertRule) error { return alert.RuleUpdate(ctx, rule) }

// Deprecated: Use alert.RuleDelete from internal/op/alert instead.
func AlertRuleDelete(ctx context.Context, id int) error { return alert.RuleDelete(ctx, id) }

// Deprecated: Use alert.NotifChannelList from internal/op/alert instead.
func AlertNotifChannelList(ctx context.Context) ([]model.AlertNotifChannel, error) { return alert.NotifChannelList(ctx) }

// Deprecated: Use alert.NotifChannelCreate from internal/op/alert instead.
func AlertNotifChannelCreate(ctx context.Context, ch *model.AlertNotifChannel) error { return alert.NotifChannelCreate(ctx, ch) }

// Deprecated: Use alert.NotifChannelUpdate from internal/op/alert instead.
func AlertNotifChannelUpdate(ctx context.Context, ch *model.AlertNotifChannel) error { return alert.NotifChannelUpdate(ctx, ch) }

// Deprecated: Use alert.NotifChannelDelete from internal/op/alert instead.
func AlertNotifChannelDelete(ctx context.Context, id int) error { return alert.NotifChannelDelete(ctx, id) }

// Deprecated: Use alert.StateGet from internal/op/alert instead.
func AlertStateGet(ruleID int) model.AlertStateRecord { return alert.StateGet(ruleID) }

// Deprecated: Use alert.StateSet from internal/op/alert instead.
func AlertStateSet(ruleID int, state model.AlertState) { alert.StateSet(ruleID, state) }

// Deprecated: Use alert.HistoryList from internal/op/alert instead.
func AlertHistoryList(ctx context.Context, limit int) ([]model.AlertHistory, error) { return alert.HistoryList(ctx, limit) }

// Deprecated: Use alert.HistoryAdd from internal/op/alert instead.
func AlertHistoryAdd(ctx context.Context, entry *model.AlertHistory) error { return alert.HistoryAdd(ctx, entry) }
