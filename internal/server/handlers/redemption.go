package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/remotesite"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

func init() {
	router.NewGroupRouter("/api/v1/redemption").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSitesRead)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/redeem", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(redeemCodes),
		).
		AddRoute(
			router.NewRoute("/redeem-all", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(redeemAllSites),
		).
		AddRoute(
			router.NewRoute("/history/:site_id", http.MethodGet).
				Handle(getRedemptionHistory),
		)
}

func redeemCodes(c *gin.Context) {
	var req model.RedemptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	if len(req.Codes) == 0 {
		resp.Error(c, http.StatusBadRequest, "codes list is empty")
		return
	}

	result, err := remotesite.RedeemCodes(c.Request.Context(), &req)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, result)
}

func redeemAllSites(c *gin.Context) {
	var req struct {
		Codes []string `json:"codes" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	if len(req.Codes) == 0 {
		resp.Error(c, http.StatusBadRequest, "codes list is empty")
		return
	}

	records := remotesite.RedeemAllSites(c.Request.Context(), req.Codes)
	resp.Success(c, records)
}

func getRedemptionHistory(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)

	records, err := remotesite.ListRedemptionHistory(c.Request.Context(), siteID, limit)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, records)
}
