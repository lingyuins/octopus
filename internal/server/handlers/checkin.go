package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/op/remotesite"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

func init() {
	router.NewGroupRouter("/api/v1/checkin").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSitesRead)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/status/:site_id", http.MethodGet).
				Handle(getCheckInStatus),
		).
		AddRoute(
			router.NewRoute("/execute/:site_id", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(executeCheckIn),
		).
		AddRoute(
			router.NewRoute("/execute-all", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(executeCheckInAll),
		).
		AddRoute(
			router.NewRoute("/history/:site_id", http.MethodGet).
				Handle(getCheckInHistory),
		)
}

func getCheckInStatus(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	record, err := remotesite.GetTodayCheckInStatus(c.Request.Context(), siteID)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, record)
}

func executeCheckIn(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	record, err := remotesite.ExecuteCheckIn(c.Request.Context(), siteID)
	if err != nil && record == nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, record)
}

func executeCheckInAll(c *gin.Context) {
	records := remotesite.ExecuteCheckInAll(c.Request.Context())
	resp.Success(c, records)
}

func getCheckInHistory(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	limitStr := c.DefaultQuery("limit", "30")
	limit, _ := strconv.Atoi(limitStr)

	records, err := remotesite.ListCheckInHistory(c.Request.Context(), siteID, limit)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, records)
}
