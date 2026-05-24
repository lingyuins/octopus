package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/alert"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
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
	resp.Success(c, channels)
}

func createNotifChannel(c *gin.Context) {
	var req alertNotifChannelPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	ch := req.toModel()
	ch.ID = 0
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
	ScopeChannelID int                          `json:"scope_channel_id,omitempty"`
	ScopeAPIKeyID  int                          `json:"scope_api_key_id,omitempty"`
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
		ScopeChannelID: p.ScopeChannelID,
		ScopeAPIKeyID:  p.ScopeAPIKeyID,
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
