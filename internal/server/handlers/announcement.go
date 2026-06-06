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
	router.NewGroupRouter("/api/v1/announcement").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSitesRead)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/list", http.MethodGet).
				Handle(listAnnouncements),
		).
		AddRoute(
			router.NewRoute("/list/:site_id", http.MethodGet).
				Handle(listAnnouncementsBySite),
		).
		AddRoute(
			router.NewRoute("/refresh/:site_id", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(refreshAnnouncement),
		).
		AddRoute(
			router.NewRoute("/refresh-all", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(refreshAllAnnouncements),
		)
}

func listAnnouncements(c *gin.Context) {
	announcements, err := remotesite.ListAnnouncements(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, announcements)
}

func listAnnouncementsBySite(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	announcements, err := remotesite.ListAnnouncementsBySite(c.Request.Context(), siteID)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, announcements)
}

func refreshAnnouncement(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	if err := remotesite.FetchAndStoreAnnouncement(c.Request.Context(), siteID); err != nil {
		resp.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	resp.Success(c, nil)
}

func refreshAllAnnouncements(c *gin.Context) {
	count := remotesite.FetchAllAnnouncements(c.Request.Context())
	resp.Success(c, gin.H{"refreshed": count})
}
