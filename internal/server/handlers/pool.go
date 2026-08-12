package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/pool"
	"github.com/lingyuins/octopus/internal/relay/poolscheduler"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

func init() {
	router.NewGroupRouter("/api/v1/pool").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermChannelsRead)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/list", http.MethodGet).
				Handle(listPools),
		).
		AddRoute(
			router.NewRoute("/create", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(createPool),
		).
		AddRoute(
			router.NewRoute("/update", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(updatePool),
		).
		AddRoute(
			router.NewRoute("/delete/:id", http.MethodDelete).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(deletePool),
		).
		AddRoute(
			router.NewRoute("/:id/account/list", http.MethodGet).
				Handle(listPoolAccounts),
		).
		AddRoute(
			router.NewRoute("/:id/account/create", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(createPoolAccount),
		).
		AddRoute(
			router.NewRoute("/:id/account/update/:aid", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(updatePoolAccount),
		).
		AddRoute(
			router.NewRoute("/:id/account/delete/:aid", http.MethodDelete).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(deletePoolAccount),
		).
		AddRoute(
			router.NewRoute("/:id/account/test", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(testPoolAccount),
		).
		AddRoute(
			router.NewRoute("/:id/account/quota/:aid", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(fetchPoolAccountQuota),
		).
		AddRoute(
			router.NewRoute("/:id/account/refresh-token/:aid", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(refreshPoolAccountToken),
		).
		AddRoute(
			router.NewRoute("/:id/account/recover/:aid", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(recoverPoolAccount),
		).
		AddRoute(
			router.NewRoute("/:id/account/temp-unsched/:aid", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(tempUnschedPoolAccount),
		).
		AddRoute(
			router.NewRoute("/:id/account/batch-refresh", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(batchRefreshPoolAccounts),
		).
		AddRoute(
			router.NewRoute("/:id/account/batch-clear-error", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(batchClearErrorPoolAccounts),
		).
		AddRoute(
			router.NewRoute("/:id/account/batch-test", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(batchTestPoolAccounts),
		).
		AddRoute(
			router.NewRoute("/:id/account/export", http.MethodGet).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(exportPoolAccounts),
		).
		AddRoute(
			router.NewRoute("/import", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(importPoolAccounts),
		).
		AddRoute(
			router.NewRoute("/:id/account/:aid", http.MethodGet).
				Handle(getPoolAccount),
		)
}

func listPools(c *gin.Context) {
	pools, err := pool.ListPools()
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, pools)
}

func createPool(c *gin.Context) {
	var req model.AccountPool
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if err := pool.CreatePool(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, req)
}

func updatePool(c *gin.Context) {
	var req struct {
		ID                 int    `json:"id" binding:"required"`
		Name               string `json:"name"`
		Description        string `json:"description"`
		Strategy           string `json:"strategy"`
		DefaultConcurrency *int   `json:"default_concurrency"`
		CooldownBaseSec    *int   `json:"cooldown_base_sec"`
		Enabled            *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Strategy != "" {
		switch req.Strategy {
		case "ewma", "round_robin", "random", "least_loaded":
			updates["strategy"] = req.Strategy
		default:
			resp.Error(c, http.StatusBadRequest, "unsupported pool strategy")
			return
		}
	}
	if req.DefaultConcurrency != nil {
		updates["default_concurrency"] = *req.DefaultConcurrency
	}
	if req.CooldownBaseSec != nil {
		updates["cooldown_base_sec"] = *req.CooldownBaseSec
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if err := pool.UpdatePool(req.ID, updates); err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

func deletePool(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	if err := pool.DeletePool(id); err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

func listPoolAccounts(c *gin.Context) {
	poolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid pool id")
		return
	}
	accounts, err := pool.ListAccounts(poolID)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, pool.MaskAccounts(accounts))
}

// poolAccountRequest 账号创建/更新请求体。扩展自旧 model.PoolAccount 直绑，
// 增加平台/类型/模型/备注/token_expires_at 字段，凭据在写入前加密。
type poolAccountRequest struct {
	Name               string `json:"name"`
	Platform           string `json:"platform"`
	Type               string `json:"type"`
	Models             string `json:"models"`
	Credentials        string `json:"credentials"`
	BaseURL            string `json:"base_url"`
	Status             string `json:"status"`
	Schedulable        *bool  `json:"schedulable"`
	Priority           *int   `json:"priority"`
	Concurrency        *int   `json:"concurrency"`
	Weight             *int   `json:"weight"`
	LoadFactor         *int   `json:"load_factor"`
	AutoPauseOnExpired *bool  `json:"auto_pause_on_expired"`
	ExpiresAt          *int64 `json:"expires_at"`
	Extra              string `json:"extra"`
	ProxyConfigID      *int   `json:"proxy_config_id"`
	Notes              string `json:"notes"`
	TokenExpiresAt     *int64 `json:"token_expires_at"`
}

func createPoolAccount(c *gin.Context) {
	poolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid pool id")
		return
	}
	var req poolAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	acct := model.PoolAccount{
		PoolID:        poolID,
		Name:          req.Name,
		Platform:      req.Platform,
		Type:          req.Type,
		Models:        req.Models,
		Credentials:   pool.EncryptCredentials(req.Credentials),
		BaseURL:       req.BaseURL,
		Status:        req.Status,
		Priority:      derefInt(req.Priority),
		Concurrency:   derefInt(req.Concurrency),
		Weight:        derefInt(req.Weight),
		LoadFactor:    derefInt(req.LoadFactor),
		Extra:         req.Extra,
		ProxyConfigID: req.ProxyConfigID,
		Notes:         req.Notes,
	}
	if req.Schedulable != nil {
		acct.Schedulable = *req.Schedulable
	} else {
		acct.Schedulable = true
	}
	if req.TokenExpiresAt != nil {
		acct.TokenExpiresAt = *req.TokenExpiresAt
	}
	if req.AutoPauseOnExpired != nil {
		acct.AutoPauseOnExpired = *req.AutoPauseOnExpired
	}
	if req.ExpiresAt != nil {
		acct.ExpiresAt = *req.ExpiresAt
	}
	if err := pool.CreateAccount(&acct); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, pool.MaskAccount(&acct))
}

func updatePoolAccount(c *gin.Context) {
	poolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid pool id")
		return
	}
	accountID, err := strconv.Atoi(c.Param("aid"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid account id")
		return
	}
	var req poolAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Platform != "" {
		updates["platform"] = req.Platform
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Models != "" {
		updates["models"] = req.Models
	}
	if req.Credentials != "" {
		updates["credentials"] = pool.EncryptCredentials(req.Credentials)
	}
	if req.BaseURL != "" {
		updates["base_url"] = req.BaseURL
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Schedulable != nil {
		updates["schedulable"] = *req.Schedulable
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.Concurrency != nil {
		updates["concurrency"] = *req.Concurrency
	}
	if req.Weight != nil {
		updates["weight"] = *req.Weight
	}
	if req.LoadFactor != nil {
		updates["load_factor"] = *req.LoadFactor
	}
	if req.AutoPauseOnExpired != nil {
		updates["auto_pause_on_expired"] = *req.AutoPauseOnExpired
	}
	if req.ExpiresAt != nil {
		updates["expires_at"] = *req.ExpiresAt
	}
	if req.Extra != "" {
		updates["extra"] = req.Extra
	}
	if req.ProxyConfigID != nil {
		updates["proxy_config_id"] = *req.ProxyConfigID
	}
	if req.Notes != "" {
		updates["notes"] = req.Notes
	}
	if req.TokenExpiresAt != nil {
		updates["token_expires_at"] = *req.TokenExpiresAt
	}
	if err := pool.UpdateAccount(poolID, accountID, updates); err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

func deletePoolAccount(c *gin.Context) {
	poolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid pool id")
		return
	}
	accountID, err := strconv.Atoi(c.Param("aid"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid account id")
		return
	}
	if err := pool.DeleteAccount(poolID, accountID); err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

func getPoolAccount(c *gin.Context) {
	poolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid pool id")
		return
	}
	accountID, err := strconv.Atoi(c.Param("aid"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid account id")
		return
	}
	acct, err := pool.GetAccount(poolID, accountID)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, pool.MaskAccount(acct))
}

func testPoolAccount(c *gin.Context) {
	poolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid pool id")
		return
	}
	var req struct {
		AccountID int    `json:"account_id" binding:"required"`
		Model     string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	result, err := pool.TestAccount(poolID, req.AccountID, req.Model)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	// 测试成功 → 清除 error 状态与临时调度屏蔽（对齐 sub2api clear-error）。
	if result != nil && result.Success {
		_ = poolscheduler.RecoverAccount(poolID, req.AccountID)
	}
	resp.Success(c, result)
}

func fetchPoolAccountQuota(c *gin.Context) {
	poolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid pool id")
		return
	}
	accountID, err := strconv.Atoi(c.Param("aid"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid account id")
		return
	}
	result, err := pool.FetchAccountQuota(c.Request.Context(), poolID, accountID)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, result)
}

func refreshPoolAccountToken(c *gin.Context) {
	poolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid pool id")
		return
	}
	accountID, err := strconv.Atoi(c.Param("aid"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid account id")
		return
	}
	if err := pool.RefreshAccountToken(c.Request.Context(), poolID, accountID); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, nil)
}

func recoverPoolAccount(c *gin.Context) {
	poolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid pool id")
		return
	}
	accountID, err := strconv.Atoi(c.Param("aid"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid account id")
		return
	}
	if err := poolscheduler.RecoverAccount(poolID, accountID); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, nil)
}

func tempUnschedPoolAccount(c *gin.Context) {
	poolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid pool id")
		return
	}
	accountID, err := strconv.Atoi(c.Param("aid"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid account id")
		return
	}
	var req struct {
		Minutes int    `json:"minutes"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if req.Minutes <= 0 {
		poolscheduler.ClearTempUnsched(poolID, accountID)
		resp.Success(c, nil)
		return
	}
	if req.Reason == "" {
		req.Reason = "manual"
	}
	poolscheduler.SetTempUnsched(poolID, accountID, time.Now().Add(time.Duration(req.Minutes)*time.Minute), req.Reason)
	resp.Success(c, nil)
}

type batchAccountIDsRequest struct {
	AccountIDs []int  `json:"account_ids" binding:"required"`
	Model      string `json:"model"`
}

type batchItemError struct {
	ID    int    `json:"id"`
	Error string `json:"error"`
}

type batchTestItemResult struct {
	ID        int    `json:"id"`
	Success   bool   `json:"success"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

func batchRefreshPoolAccounts(c *gin.Context) {
	poolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid pool id")
		return
	}
	var req batchAccountIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	ok := 0
	failed := make([]batchItemError, 0)
	for _, aid := range req.AccountIDs {
		if err := pool.RefreshAccountToken(c.Request.Context(), poolID, aid); err != nil {
			failed = append(failed, batchItemError{ID: aid, Error: err.Error()})
		} else {
			ok++
		}
	}
	resp.Success(c, gin.H{"ok": ok, "failed": failed})
}

func batchClearErrorPoolAccounts(c *gin.Context) {
	poolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid pool id")
		return
	}
	var req batchAccountIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	ok := 0
	failed := make([]batchItemError, 0)
	for _, aid := range req.AccountIDs {
		if err := poolscheduler.RecoverAccount(poolID, aid); err != nil {
			failed = append(failed, batchItemError{ID: aid, Error: err.Error()})
		} else {
			ok++
		}
	}
	resp.Success(c, gin.H{"ok": ok, "failed": failed})
}

func batchTestPoolAccounts(c *gin.Context) {
	poolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid pool id")
		return
	}
	var req batchAccountIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	results := make([]batchTestItemResult, 0, len(req.AccountIDs))
	for _, aid := range req.AccountIDs {
		res, terr := pool.TestAccount(poolID, aid, req.Model)
		if terr != nil {
			results = append(results, batchTestItemResult{ID: aid, Success: false, Error: terr.Error()})
			continue
		}
		if res == nil {
			results = append(results, batchTestItemResult{ID: aid, Success: false, Error: "empty result"})
			continue
		}
		// 成功的的账号顺手恢复状态
		if res.Success {
			_ = poolscheduler.RecoverAccount(poolID, aid)
		}
		results = append(results, batchTestItemResult{
			ID:        aid,
			Success:   res.Success,
			LatencyMs: res.Latency,
			Error:     res.Error,
		})
	}
	resp.Success(c, results)
}

// PoolAccountExport 导出格式：不携带 id/pool_id/status/quota 等运行状态字段，
// 供跨站迁移使用（与 sub2api admin account_data 一致语义）。
type PoolAccountExport struct {
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	Type        string `json:"type"`
	Models      string `json:"models"`
	BaseURL     string `json:"base_url"`
	Priority    int    `json:"priority"`
	Concurrency int    `json:"concurrency"`
	Weight      int    `json:"weight"`
	LoadFactor  int    `json:"load_factor"`
	Notes       string `json:"notes"`
	Extra       string `json:"extra,omitempty"`
	Credentials string `json:"credentials"`
}

func exportPoolAccounts(c *gin.Context) {
	poolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid pool id")
		return
	}
	accounts, err := pool.ListAccounts(poolID)
	if err != nil {
		resp.InternalError(c)
		return
	}
	out := make([]PoolAccountExport, 0, len(accounts))
	for i := range accounts {
		a := &accounts[i]
		// 解密凭据返回明文，与导入格式保持一致。
		_ = pool.DecryptAccountCredentials(a)
		out = append(out, PoolAccountExport{
			Name:        a.Name,
			Platform:    a.Platform,
			Type:        a.Type,
			Models:      a.Models,
			BaseURL:     a.BaseURL,
			Priority:    a.Priority,
			Concurrency: a.Concurrency,
			Weight:      a.Weight,
			LoadFactor:  a.LoadFactor,
			Notes:       a.Notes,
			Extra:       a.Extra,
			Credentials: a.Credentials,
		})
	}
	resp.Success(c, out)
}

func importPoolAccounts(c *gin.Context) {
	var req struct {
		PoolID   int    `json:"pool_id" binding:"required"`
		Accounts string `json:"accounts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	accounts, err := pool.ParseImportedAccounts(req.Accounts, req.PoolID)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "parse accounts: "+err.Error())
		return
	}
	if err := pool.ImportAccounts(accounts); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, map[string]int{"imported": len(accounts)})
}

func derefInt(p *int) int {
	if p != nil {
		return *p
	}
	return 0
}
