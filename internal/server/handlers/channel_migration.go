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
	router.NewGroupRouter("/api/v1/channel-migration").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSitesWrite)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/migrate", http.MethodPost).
				Handle(migrateChannel),
		).
		AddRoute(
			router.NewRoute("/migrate-all", http.MethodPost).
				Handle(migrateAllChannels),
		)
}

func migrateChannel(c *gin.Context) {
	var req remotesite.MigrateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	if err := remotesite.MigrateChannel(c.Request.Context(), &req); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, nil)
}

func migrateAllChannels(c *gin.Context) {
	sourceSiteID, err := strconv.Atoi(c.Query("source_site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	targetSiteID, _ := strconv.Atoi(c.Query("target_site_id")) // 0 = local

	count, err := remotesite.MigrateAllChannels(c.Request.Context(), sourceSiteID, targetSiteID)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, gin.H{"migrated": count})
}
