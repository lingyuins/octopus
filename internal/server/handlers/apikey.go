package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/apikey"
	"github.com/lingyuins/octopus/internal/op/group"
	st "github.com/lingyuins/octopus/internal/op/stats"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
	"github.com/samber/lo"
)

func init() {
	router.NewGroupRouter("/api/v1/apikey").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermAPIKeysRead)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/create", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermAPIKeysWrite)).
				Handle(createAPIKey),
		).
		AddRoute(
			router.NewRoute("/list", http.MethodGet).
				Handle(listAPIKey),
		).
		AddRoute(
			router.NewRoute("/update", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermAPIKeysWrite)).
				Handle(updateAPIKey),
		).
		AddRoute(
			router.NewRoute("/delete/:id", http.MethodDelete).
				Use(middleware.RequirePermission(auth.PermAPIKeysWrite)).
				Handle(deleteAPIKey),
		)
	router.NewGroupRouter("/api/v1/apikey").
		Use(middleware.APIKeyAuth()).
		AddRoute(
			router.NewRoute("/stats", http.MethodGet).
				Handle(getStatsAPIKeyById),
		).
		AddRoute(
			router.NewRoute("/login", http.MethodGet).
				Handle(loginAPIKey),
		)
}

func createAPIKey(c *gin.Context) {
	var req apiKeyRequestPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	apiKey := req.toModel()
	apiKey.ID = 0
	apiKey.APIKey = auth.GenerateAPIKey()
	if err := apikey.Create(&apiKey, c.Request.Context()); err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, apiKey)
}

func listAPIKey(c *gin.Context) {
	apiKeys, err := apikey.List(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	if !auth.HasPermission(c.GetString("user_role"), auth.PermAPIKeysWrite) {
		apiKeys = maskAPIKeys(apiKeys)
	}
	resp.Success(c, apiKeys)
}

func updateAPIKey(c *gin.Context) {
	var req apiKeyRequestPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	apiKey := req.toModel()
	if err := apikey.Update(&apiKey, c.Request.Context()); err != nil {
		if status, msg, ok := classifyAPIKeyMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.InternalError(c)
		return
	}
	apiKey.APIKey = maskAPIKeyValue(apiKey.APIKey)
	resp.Success(c, apiKey)
}

func deleteAPIKey(c *gin.Context) {
	id := c.Param("id")
	idNum, err := strconv.Atoi(id)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	if err := apikey.Delete(idNum, c.Request.Context()); err != nil {
		if status, msg, ok := classifyAPIKeyMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

func getStatsAPIKeyById(c *gin.Context) {
	id := c.GetInt("api_key_id")
	stats := st.APIKeyGet(id)
	info, err := apikey.Get(id, c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	models, err := group.GroupListModel(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	var modelsString string
	if info.SupportedModels == "" {
		modelsString = strings.Join(models, ", ")
	} else {
		supportedModels := lo.Map(strings.Split(info.SupportedModels, ","), func(s string, _ int) string {
			return strings.TrimSpace(s)
		})
		models = lo.Filter(models, func(m string, _ int) bool {
			return lo.Contains(supportedModels, m)
		})
		modelsString = strings.Join(models, ", ")
	}
	info.SupportedModels = modelsString
	resp.Success(c, map[string]any{
		"stats": stats,
		"info":  info,
	})
}

func loginAPIKey(c *gin.Context) {
	resp.Success(c, nil)
}

func maskAPIKeys(keys []model.APIKey) []model.APIKey {
	if len(keys) == 0 {
		return make([]model.APIKey, 0)
	}

	masked := make([]model.APIKey, len(keys))
	for i, key := range keys {
		key.APIKey = maskAPIKeyValue(key.APIKey)
		masked[i] = key
	}
	return masked
}

func maskAPIKeyValue(raw string) string {
	return maskSecretValue(raw)
}

type apiKeyRequestPayload struct {
	ID                int     `json:"id"`
	Name              string  `json:"name"`
	Enabled           bool    `json:"enabled"`
	ExpireAt          int64   `json:"expire_at,omitempty"`
	MaxCost           float64 `json:"max_cost,omitempty"`
	SupportedModels   string  `json:"supported_models,omitempty"`
	RateLimitRPM      int     `json:"rate_limit_rpm,omitempty"`
	RateLimitTPM      int     `json:"rate_limit_tpm,omitempty"`
	PerModelQuotaJSON string  `json:"per_model_quota_json,omitempty"`
	AllowedIPs        string  `json:"allowed_ips,omitempty"`
}

func (p apiKeyRequestPayload) toModel() model.APIKey {
	return model.APIKey{
		ID:                p.ID,
		Name:              p.Name,
		Enabled:           p.Enabled,
		ExpireAt:          p.ExpireAt,
		MaxCost:           p.MaxCost,
		SupportedModels:   p.SupportedModels,
		RateLimitRPM:      p.RateLimitRPM,
		RateLimitTPM:      p.RateLimitTPM,
		PerModelQuotaJSON: p.PerModelQuotaJSON,
		AllowedIPs:        p.AllowedIPs,
	}
}

func classifyAPIKeyMutationError(err error) (int, string, bool) {
	if err == nil {
		return 0, "", false
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "api key not found"):
		return http.StatusNotFound, "API key not found", true
	case strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "duplicate entry") ||
		strings.Contains(msg, "duplicate key"):
		return http.StatusConflict, "API key already exists", true
	default:
		return 0, "", false
	}
}
