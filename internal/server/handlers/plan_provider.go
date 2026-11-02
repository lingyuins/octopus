package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
	"github.com/lingyuins/octopus/internal/planprovider"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

func init() {
	// 注入代理池 URL 解析器（planprovider 与 op 双向依赖，无法由 op 注入，在此注入）。
	planprovider.ProxyURLByConfigFunc = op.ProxyURLForConfig

	// 额度管理路由 (读)
	router.NewGroupRouter("/api/v1/plan-provider").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSitesRead)).
		AddRoute(router.NewRoute("/balance/list", http.MethodGet).Handle(listBalanceProviders)).
		AddRoute(router.NewRoute("/tokenplan/list", http.MethodGet).Handle(listTokenPlanProviders)).
		AddRoute(router.NewRoute("/balance/categories", http.MethodGet).Handle(getBalanceCategories)).
		AddRoute(router.NewRoute("/tokenplan/categories", http.MethodGet).Handle(getTokenPlanCategories)).
		AddRoute(router.NewRoute("/wallet/transactions/:id", http.MethodGet).Handle(getTokenRhythmWallet))

	// 额度管理路由 (写)
	router.NewGroupRouter("/api/v1/plan-provider").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSitesWrite)).
		AddRoute(router.NewRoute("/add", http.MethodPost).Handle(addPlanProvider)).
		AddRoute(router.NewRoute("/refresh/:id", http.MethodPost).Handle(refreshPlanProvider)).
		AddRoute(router.NewRoute("/credentials/:id", http.MethodPut).Handle(updatePlanProviderCredentials)).
		AddRoute(router.NewRoute("/:id", http.MethodDelete).Handle(deletePlanProvider))
}

func listBalanceProviders(c *gin.Context) {
	providers, err := planprovider.ListProviders(c.Request.Context(), model.PlanProviderTypeBalance)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, providers)
}

func listTokenPlanProviders(c *gin.Context) {
	providers, err := planprovider.ListProviders(c.Request.Context(), model.PlanProviderTypeTokenPlan)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, providers)
}

func getBalanceCategories(c *gin.Context) {
	categories := planprovider.GetCategories(model.PlanProviderTypeBalance)
	resp.Success(c, categories)
}

func getTokenPlanCategories(c *gin.Context) {
	categories := planprovider.GetCategories(model.PlanProviderTypeTokenPlan)
	resp.Success(c, categories)
}

// getTokenRhythmWallet 基元律动资金明细。
// 口径与官网「用户中心 → 账户」页（/account/account）的「资金明细」表一致：
// 赠送额度账本（逐笔赠金 + 充值本金行，含已用尽/已到期历史）+ 账户余额 + 累计消费；
// 只对 tokenrhythm 类 balance provider 生效，字段见 planprovider.TokenRhythmWallet 注释。
func getTokenRhythmWallet(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	wallet, err := planprovider.QueryTokenRhythmWalletByID(c.Request.Context(), id)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, wallet)
}

type addPlanProviderRequest struct {
	Category      model.PlanProviderCategory `json:"category" binding:"required"`
	APIKey        string                     `json:"api_key"`
	ForwardAPIKey string                     `json:"forward_api_key,omitempty"`
	Name          string                     `json:"name"`
	// RefreshIntervalMin 自动刷新间隔（分钟），0 = 跟随全局默认。
	RefreshIntervalMin int `json:"refresh_interval_min,omitempty"`
	// 代理配置：目前仅 Codex 类生效（chatgpt.com 国内不可直连）。
	ProxyMode     model.ProxyUsageMode `json:"proxy_mode,omitempty"`
	ProxyConfigID *int                 `json:"proxy_config_id,omitempty"`
	// 智谱团队版（zhipu_team）专用：组织 ID / 项目 ID，其他厂商忽略。
	TeamOrganizationID string `json:"team_organization_id,omitempty"`
	TeamProjectID      string `json:"team_project_id,omitempty"`
	// 账号密码自动登录（sensenova_plan / deepseek / tokenrhythm 可选）：
	// sensenova 配置后系统自动登录并续期控制台 Bearer Token（api_key 可留空）；
	// deepseek 配置后用于查询官方 usage（api_key 仍必填，用于余额查询）；
	// tokenrhythm 配置后登录换会话 Cookie 作为主凭据（api_key 可留空）。
	LoginUsername string `json:"login_username,omitempty"`
	LoginPassword string `json:"login_password,omitempty"`
}

func addPlanProvider(c *gin.Context) {
	var req addPlanProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	provider, err := planprovider.AddProvider(c.Request.Context(), req.Category, req.APIKey, req.ForwardAPIKey, req.Name, req.RefreshIntervalMin, req.ProxyMode, req.ProxyConfigID, req.TeamOrganizationID, req.TeamProjectID, req.LoginUsername, req.LoginPassword)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, provider)
}

func refreshPlanProvider(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	provider, err := planprovider.RefreshProvider(c.Request.Context(), id)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, provider)
}

type updatePlanProviderCredentialsRequest struct {
	// APIKey 与 LoginUsername 至少填一个（sensenova_plan 支持账号密码模式，
	// 填了账号密码后系统自动登录拿 access_token）。
	APIKey        string `json:"api_key"`
	ForwardAPIKey string `json:"forward_api_key,omitempty"`
	// 智谱团队版（zhipu_team）专用：组织 ID / 项目 ID，留空则清空。
	TeamOrganizationID string `json:"team_organization_id,omitempty"`
	TeamProjectID      string `json:"team_project_id,omitempty"`
	// 账号密码自动登录（sensenova_plan 专用，可选）：填了保存账号密码并自动登录；
	// 不填则清除账号密码模式（切回纯 Bearer Token）。
	LoginUsername string `json:"login_username,omitempty"`
	LoginPassword string `json:"login_password,omitempty"`
}

func updatePlanProviderCredentials(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req updatePlanProviderCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	provider, err := planprovider.UpdateProviderCredentials(c.Request.Context(), id, req.APIKey, req.ForwardAPIKey, req.TeamOrganizationID, req.TeamProjectID, req.LoginUsername, req.LoginPassword)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, provider)
}
func deletePlanProvider(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	if err := planprovider.DeleteProvider(c.Request.Context(), id); err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, nil)
}
