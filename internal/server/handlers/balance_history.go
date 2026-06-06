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
	router.NewGroupRouter("/api/v1/balance-history").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSitesRead)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/list/:site_id", http.MethodGet).
				Handle(listBalanceSnapshots),
		).
		AddRoute(
			router.NewRoute("/chart/:site_id", http.MethodGet).
				Handle(getBalanceChart),
		).
		AddRoute(
			router.NewRoute("/capture/:site_id", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(captureBalance),
		).
		AddRoute(
			router.NewRoute("/prediction/:site_id", http.MethodGet).
				Handle(getBalancePrediction),
		)
}

func listBalanceSnapshots(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	snapshots, err := remotesite.ListBalanceSnapshots(c.Request.Context(), siteID, startDate, endDate)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, snapshots)
}

func getBalanceChart(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	points, err := remotesite.GetBalanceChartData(c.Request.Context(), siteID, startDate, endDate)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, points)
}

func captureBalance(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	snapshot, err := remotesite.CaptureBalanceSnapshot(c.Request.Context(), siteID, "refresh")
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, snapshot)
}

func getBalancePrediction(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}

	prediction, err := remotesite.PredictBalance(c.Request.Context(), siteID)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, prediction)
}
