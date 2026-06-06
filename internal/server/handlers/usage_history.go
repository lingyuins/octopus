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
	router.NewGroupRouter("/api/v1/usage-history").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSitesRead)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("", http.MethodGet).
				Handle(queryUsageHistory),
		).
		AddRoute(
			router.NewRoute("/summary", http.MethodGet).
				Handle(queryUsageSummary),
		).
		AddRoute(
			router.NewRoute("/hourly", http.MethodGet).
				Handle(queryUsageHourly),
		).
		AddRoute(
			router.NewRoute("/models/:site_id", http.MethodGet).
				Handle(getUsageModels),
		).
		AddRoute(
			router.NewRoute("/sync/:site_id", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(syncUsageHistory),
		).
		AddRoute(
			router.NewRoute("/sync-all", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(syncAllUsageHistory),
		)
}

func queryUsageHistory(c *gin.Context) {
	q := &model.RemoteUsageQuery{
		DayFrom:   c.Query("day_from"),
		DayTo:     c.Query("day_to"),
		ModelName: c.Query("model_name"),
		TokenName: c.Query("token_name"),
	}
	if v := c.Query("site_id"); v != "" {
		q.SiteID, _ = strconv.Atoi(v)
	}
	if v := c.DefaultQuery("limit", "100"); v != "" {
		q.Limit, _ = strconv.Atoi(v)
	}
	if v := c.DefaultQuery("offset", "0"); v != "" {
		q.Offset, _ = strconv.Atoi(v)
	}

	records, total, err := remotesite.QueryUsageHistory(c.Request.Context(), q)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, gin.H{
		"records": records,
		"total":   total,
		"limit":   q.Limit,
		"offset":  q.Offset,
	})
}

func queryUsageSummary(c *gin.Context) {
	q := &model.RemoteUsageQuery{
		DayFrom:   c.Query("day_from"),
		DayTo:     c.Query("day_to"),
		ModelName: c.Query("model_name"),
		TokenName: c.Query("token_name"),
	}
	if v := c.Query("site_id"); v != "" {
		q.SiteID, _ = strconv.Atoi(v)
	}

	summaries, err := remotesite.QueryUsageSummary(c.Request.Context(), q)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, summaries)
}

func queryUsageHourly(c *gin.Context) {
	siteID := 0
	if v := c.Query("site_id"); v != "" {
		siteID, _ = strconv.Atoi(v)
	}
	dayKey := c.Query("day_key")

	hourly, err := remotesite.QueryUsageHourly(c.Request.Context(), siteID, dayKey)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, hourly)
}

func getUsageModels(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}

	models, err := remotesite.GetUsageModels(c.Request.Context(), siteID)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, models)
}

func syncUsageHistory(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}

	n, err := remotesite.SyncUsageHistory(c.Request.Context(), siteID)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, gin.H{"inserted": n})
}

func syncAllUsageHistory(c *gin.Context) {
	n := remotesite.SyncAllUsageHistory(c.Request.Context())
	resp.Success(c, gin.H{"inserted": n})
}
