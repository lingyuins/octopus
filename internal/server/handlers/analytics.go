package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/analytics"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

func init() {
	router.NewGroupRouter("/api/v1/analytics").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermStatsRead)).
		AddRoute(
			router.NewRoute("/overview", http.MethodGet).
				Handle(getAnalyticsOverview),
		).
		AddRoute(
			router.NewRoute("/utilization", http.MethodGet).
				Handle(getAnalyticsUtilization),
		).
		AddRoute(
			router.NewRoute("/evaluation", http.MethodGet).
				Handle(getAnalyticsEvaluation),
		).
		AddRoute(
			router.NewRoute("/group-health", http.MethodGet).
				Handle(getAnalyticsGroupHealth),
		).
		AddRoute(
			router.NewRoute("/provider-breakdown", http.MethodGet).
				Handle(getAnalyticsProviderBreakdown),
		).
		AddRoute(
			router.NewRoute("/model-breakdown", http.MethodGet).
				Handle(getAnalyticsModelBreakdown),
		).
		AddRoute(
			router.NewRoute("/channel-model", http.MethodGet).
				Handle(getAnalyticsChannelModel),
		).
		AddRoute(
			router.NewRoute("/auto-strategy", http.MethodGet).
				Handle(getAnalyticsAutoStrategy),
		).
		AddRoute(
			router.NewRoute("/apikey-breakdown", http.MethodGet).
				Handle(getAnalyticsAPIKeyBreakdown),
		).
		AddRoute(
			router.NewRoute("/latency-distribution", http.MethodGet).
				Handle(getAnalyticsLatencyDistribution),
		)
}

func getAnalyticsOverview(c *gin.Context) {
	analyticsRange, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}

	data, err := analytics.AnalyticsOverviewGet(c.Request.Context(), analyticsRange)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, data)
}

func getAnalyticsUtilization(c *gin.Context) {
	analyticsRange, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}

	data, err := analytics.AnalyticsUtilizationGet(c.Request.Context(), analyticsRange)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, data)
}

func getAnalyticsEvaluation(c *gin.Context) {
	data, err := analytics.AnalyticsEvaluationGet(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, data)
}

func getAnalyticsGroupHealth(c *gin.Context) {
	data, err := analytics.AnalyticsGroupHealthGet(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, data)
}

func getAnalyticsProviderBreakdown(c *gin.Context) {
	analyticsRange, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}

	data, err := analytics.AnalyticsProviderBreakdownGet(c.Request.Context(), analyticsRange)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, data)
}

func getAnalyticsModelBreakdown(c *gin.Context) {
	analyticsRange, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}

	data, err := analytics.AnalyticsModelBreakdownGet(c.Request.Context(), analyticsRange)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, data)
}

func getAnalyticsChannelModel(c *gin.Context) {
	analyticsRange, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}
	var groupID *int
	if v := c.Query("group_id"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			resp.Error(c, http.StatusBadRequest, "invalid group_id")
			return
		}
		groupID = &n
	}

	data, err := analytics.AnalyticsChannelModelBreakdownGet(c.Request.Context(), analyticsRange, groupID)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, data)
}

func getAnalyticsAutoStrategy(c *gin.Context) {
	var groupID *int
	if v := c.Query("group_id"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			resp.Error(c, http.StatusBadRequest, "invalid group_id")
			return
		}
		groupID = &n
	}

	data, err := analytics.AnalyticsAutoStrategyGet(c.Request.Context(), groupID)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, data)
}

func getAnalyticsAPIKeyBreakdown(c *gin.Context) {
	analyticsRange, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}

	data, err := analytics.AnalyticsAPIKeyBreakdownGet(c.Request.Context(), analyticsRange)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, data)
}

func getAnalyticsLatencyDistribution(c *gin.Context) {
	analyticsRange, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}
	data, err := analytics.AnalyticsLatencyDistributionGet(c.Request.Context(), analyticsRange)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, data)
}

func parseAnalyticsRange(c *gin.Context) (model.AnalyticsRange, bool) {
	analyticsRange, err := model.ParseAnalyticsRange(c.Query("range"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return "", false
	}
	return analyticsRange, true
}
