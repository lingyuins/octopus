package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/helper"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/alert"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/lingyuins/octopus/internal/utils/xurl"
)

func init() {
	router.NewGroupRouter("/api/v1/alert").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSettingsRead)).
		Use(middleware.RequireJSON()).
		AddRoute(router.NewRoute("/rule/list", http.MethodGet).Handle(listAlertRules)).
		AddRoute(router.NewRoute("/rule/create", http.MethodPost).Use(middleware.RequirePermission(auth.PermSettingsWrite)).Handle(createAlertRule)).
		AddRoute(router.NewRoute("/rule/update", http.MethodPost).Use(middleware.RequirePermission(auth.PermSettingsWrite)).Handle(updateAlertRule)).
		AddRoute(router.NewRoute("/rule/delete/:id", http.MethodDelete).Use(middleware.RequirePermission(auth.PermSettingsWrite)).Handle(deleteAlertRule)).
		AddRoute(router.NewRoute("/notif/list", http.MethodGet).Handle(listNotifChannels)).
		AddRoute(router.NewRoute("/notif/create", http.MethodPost).Use(middleware.RequirePermission(auth.PermSettingsWrite)).Handle(createNotifChannel)).
		AddRoute(router.NewRoute("/notif/update", http.MethodPost).Use(middleware.RequirePermission(auth.PermSettingsWrite)).Handle(updateNotifChannel)).
		AddRoute(router.NewRoute("/notif/delete/:id", http.MethodDelete).Use(middleware.RequirePermission(auth.PermSettingsWrite)).Handle(deleteNotifChannel)).
		AddRoute(router.NewRoute("/notif/test", http.MethodPost).Use(middleware.RequirePermission(auth.PermSettingsWrite)).Handle(testNotifChannel)).
		AddRoute(router.NewRoute("/history", http.MethodGet).Handle(listAlertHistory))
}

func listAlertRules(c *gin.Context) {
	rules, err := alert.RuleList(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, rules)
}

func createAlertRule(c *gin.Context) {
	var req alertRulePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	rule := req.toModel()
	rule.ID = 0
	if err := alert.RuleCreate(c.Request.Context(), &rule); err != nil {
		if status, msg, ok := classifyAlertMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, rule)
}

func updateAlertRule(c *gin.Context) {
	var req alertRulePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	rule := req.toModel()
	if err := alert.RuleUpdate(c.Request.Context(), &rule); err != nil {
		if status, msg, ok := classifyAlertMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

func deleteAlertRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	if err := alert.RuleDelete(c.Request.Context(), id); err != nil {
		if status, msg, ok := classifyAlertMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

func listNotifChannels(c *gin.Context) {
	channels, err := alert.NotifChannelList(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	redacted := make([]model.AlertNotifChannel, len(channels))
	copy(redacted, channels)
	for i := range redacted {
		redactNotifChannel(&redacted[i])
	}
	resp.Success(c, redacted)
}

func createNotifChannel(c *gin.Context) {
	var req alertNotifChannelPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	ch := req.toModel()
	ch.ID = 0
	if err := validateNotifChannelURL(&ch); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := alert.NotifChannelCreate(c.Request.Context(), &ch); err != nil {
		if status, msg, ok := classifyAlertMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, ch)
}

func updateNotifChannel(c *gin.Context) {
	var req alertNotifChannelPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	ch := req.toModel()
	if err := mergeNotifChannelSecrets(c.Request.Context(), &ch); err != nil {
		log.Warnf("failed to merge notification channel %d secrets: %v", ch.ID, err)
		resp.InternalError(c)
		return
	}
	if err := validateNotifChannelURL(&ch); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := alert.NotifChannelUpdate(c.Request.Context(), &ch); err != nil {
		if status, msg, ok := classifyAlertMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

func deleteNotifChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	if err := alert.NotifChannelDelete(c.Request.Context(), id); err != nil {
		if status, msg, ok := classifyAlertMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

// testNotifChannel sends a test notification using the supplied channel configuration.
// It accepts the same payload shape as create/update so unsaved drafts can be
// verified directly from the management UI.
func testNotifChannel(c *gin.Context) {
	var req alertNotifChannelPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	ch := req.toModel()
	if err := validateNotifChannelURL(&ch); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := helper.TestNotification(&ch); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, nil)
}

func listAlertHistory(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit < 1 || limit > 500 {
		limit = 100
	}
	history, err := alert.HistoryList(c.Request.Context(), limit)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, history)
}

type alertRulePayload struct {
	ID             int                          `json:"id"`
	Name           string                       `json:"name"`
	Enabled        bool                         `json:"enabled"`
	ConditionType  model.AlertRuleConditionType `json:"condition_type"`
	Threshold      float64                      `json:"threshold"`
	ConditionJSON  string                       `json:"condition_json,omitempty"`
	NotifChannelID int                          `json:"notif_channel_id"`
	CooldownSec    int                          `json:"cooldown_sec"`
	WindowSec      int                          `json:"window_sec,omitempty"`
	ScopeChannelID int                          `json:"scope_channel_id,omitempty"`
	ScopeAPIKeyID  int                          `json:"scope_api_key_id,omitempty"`
	ScopeGroupID   int                          `json:"scope_group_id,omitempty"`
	ScopeModelName string                       `json:"scope_model_name,omitempty"`
}

func (p alertRulePayload) toModel() model.AlertRule {
	return model.AlertRule{
		ID:             p.ID,
		Name:           p.Name,
		Enabled:        p.Enabled,
		ConditionType:  p.ConditionType,
		Threshold:      p.Threshold,
		ConditionJSON:  p.ConditionJSON,
		NotifChannelID: p.NotifChannelID,
		CooldownSec:    p.CooldownSec,
		WindowSec:      p.WindowSec,
		ScopeChannelID: p.ScopeChannelID,
		ScopeAPIKeyID:  p.ScopeAPIKeyID,
		ScopeGroupID:   p.ScopeGroupID,
		ScopeModelName: p.ScopeModelName,
	}
}

type alertNotifChannelPayload struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	URL     string `json:"url"`
	Secret  string `json:"secret,omitempty"`
	Headers string `json:"headers,omitempty"`
	Config  string `json:"config,omitempty"`
}

func (p alertNotifChannelPayload) toModel() model.AlertNotifChannel {
	return model.AlertNotifChannel{
		ID:      p.ID,
		Name:    p.Name,
		Type:    p.Type,
		URL:     p.URL,
		Secret:  p.Secret,
		Headers: p.Headers,
		Config:  p.Config,
	}
}

// validateNotifChannelURL 校验通知渠道的服务端出站 URL（webhook URL、gotify
// server_url），阻止把通知 POST 到内网/元数据地址（SSRF）。存量渠道不受影响，
// 仅在新写入/更新/测试时校验。
func validateNotifChannelURL(ch *model.AlertNotifChannel) error {
	if ch == nil {
		return fmt.Errorf("notification channel is nil")
	}
	if url := strings.TrimSpace(ch.URL); url != "" {
		if err := xurl.AssertSafeURL(url); err != nil {
			return fmt.Errorf("unsafe notification url: %w", err)
		}
	}
	if ch.Config != "" {
		var gotifyCfg model.GotifyConfig
		if err := json.Unmarshal([]byte(ch.Config), &gotifyCfg); err == nil && strings.TrimSpace(gotifyCfg.ServerURL) != "" {
			if err := xurl.AssertSafeURL(gotifyCfg.ServerURL); err != nil {
				return fmt.Errorf("unsafe gotify server_url: %w", err)
			}
		}
	}
	return nil
}

func maskNotificationSecret(s string) string {
	if s == "" || strings.Contains(s, "***") {
		return s
	}
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "***" + s[len(s)-4:]
}

func redactNotifChannel(ch *model.AlertNotifChannel) {
	ch.Secret = maskNotificationSecret(ch.Secret)
	if ch.Config == "" {
		return
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(ch.Config), &raw); err != nil {
		return
	}
	for _, key := range []string{"token", "password", "bot_token", "webhook_key", "secret", "access_token"} {
		if v, ok := raw[key].(string); ok && v != "" {
			raw[key] = maskNotificationSecret(v)
		}
	}
	if b, err := json.Marshal(raw); err == nil {
		ch.Config = string(b)
	}
}

// mergeNotifChannelSecrets 将更新负载中被掩码（***）或清空的密钥恢复为库中
// 现值。读取现值失败时返回 error——继续更新会把掩码/空值原样写库，导致真密钥丢失。
// 渠道不存在时返回 nil（静默跳过）：后续的 NotifChannelUpdate 负责报 404。
func mergeNotifChannelSecrets(ctx context.Context, ch *model.AlertNotifChannel) error {
	if ch == nil || ch.ID == 0 {
		return nil
	}
	channels, err := alert.NotifChannelList(ctx)
	if err != nil {
		return fmt.Errorf("list notification channels: %w", err)
	}
	var old *model.AlertNotifChannel
	for i := range channels {
		if channels[i].ID == ch.ID {
			old = &channels[i]
			break
		}
	}
	if old == nil {
		return nil
	}
	if ch.Secret == "" || strings.Contains(ch.Secret, "***") {
		ch.Secret = old.Secret
	}
	ch.Config = mergeMaskedConfig(old.Config, ch.Config)
	return nil
}

func mergeMaskedConfig(oldConfig, newConfig string) string {
	if newConfig == "" {
		return oldConfig
	}
	var oldRaw, newRaw map[string]any
	if json.Unmarshal([]byte(oldConfig), &oldRaw) != nil || json.Unmarshal([]byte(newConfig), &newRaw) != nil {
		return newConfig
	}
	for _, key := range []string{"token", "password", "bot_token", "webhook_key", "secret", "access_token"} {
		if v, ok := newRaw[key].(string); ok && (v == "" || strings.Contains(v, "***")) {
			if oldVal, ok := oldRaw[key]; ok {
				newRaw[key] = oldVal
			}
		}
	}
	b, err := json.Marshal(newRaw)
	if err != nil {
		return newConfig
	}
	return string(b)
}

func classifyAlertMutationError(err error) (int, string, bool) {
	if err == nil {
		return 0, "", false
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "alert rule not found"):
		return http.StatusNotFound, "alert rule not found", true
	case strings.Contains(msg, "alert notification channel not found"):
		return http.StatusNotFound, "alert notification channel not found", true
	case strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "duplicate entry") ||
		strings.Contains(msg, "duplicate key"):
		return http.StatusConflict, "alert resource already exists", true
	default:
		return 0, "", false
	}
}
