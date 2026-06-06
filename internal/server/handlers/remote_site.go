package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/hub"
	_ "github.com/lingyuins/octopus/internal/hub/common"
	"github.com/lingyuins/octopus/internal/hub/ldoh"
	_ "github.com/lingyuins/octopus/internal/hub/octopus"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/remotesite"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

func init() {
	router.NewGroupRouter("/api/v1/remote-site").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSitesRead)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/list", http.MethodGet).
				Handle(listRemoteSites),
		).
		AddRoute(
			router.NewRoute("/create", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(createRemoteSite),
		).
		AddRoute(
			router.NewRoute("/update", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(updateRemoteSite),
		).
		AddRoute(
			router.NewRoute("/delete/:id", http.MethodDelete).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(deleteRemoteSite),
		).
		AddRoute(
			router.NewRoute("/refresh/:id", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(refreshRemoteSite),
		).
		AddRoute(
			router.NewRoute("/refresh-all", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(refreshAllRemoteSites),
		).
		AddRoute(
			router.NewRoute("/detect", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(detectSiteType),
		).
		AddRoute(
			router.NewRoute("/models/:id", http.MethodGet).
				Handle(fetchRemoteSiteModels),
		).
		AddRoute(
			router.NewRoute("/pricing/:id", http.MethodGet).
				Handle(fetchRemoteSitePricing),
		).
		AddRoute(
			router.NewRoute("/site-types", http.MethodGet).
				Handle(listSiteTypes),
		)

	router.NewGroupRouter("/api/v1/site-discovery").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSitesWrite)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/discover", http.MethodGet).
				Handle(discoverSites),
		)
}

func listRemoteSites(c *gin.Context) {
	sites, err := remotesite.List(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	if isViewerRole(c.GetString("user_role")) {
		redactRemoteSiteBaseURLsForViewer(sites)
	}
	resp.Success(c, sites)
}

func createRemoteSite(c *gin.Context) {
	var req model.RemoteSiteCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	site, err := remotesite.Create(c.Request.Context(), &req)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, site)
}

func updateRemoteSite(c *gin.Context) {
	var req model.RemoteSiteUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	site, err := remotesite.Update(c.Request.Context(), &req)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, site)
}

func deleteRemoteSite(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	if err := remotesite.Delete(c.Request.Context(), id); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, nil)
}

func refreshRemoteSite(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	result, err := remotesite.Refresh(c.Request.Context(), id)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, result)
}

func refreshAllRemoteSites(c *gin.Context) {
	results, err := remotesite.RefreshAll(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, results)
}

func detectSiteType(c *gin.Context) {
	var req model.RemoteSiteDetectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	siteType, err := remotesite.DetectSiteType(c.Request.Context(), req.BaseURL, req.AccessToken)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, gin.H{"site_type": siteType})
}

func fetchRemoteSiteModels(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	site, err := remotesite.Get(c.Request.Context(), id)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	adapter, err := hub.Get(site.SiteType)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	models, err := adapter.FetchModels(c.Request.Context(), site)
	if err != nil {
		resp.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	resp.Success(c, models)
}

func fetchRemoteSitePricing(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	site, err := remotesite.Get(c.Request.Context(), id)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	adapter, err := hub.Get(site.SiteType)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	pricing, err := adapter.FetchModelPricing(c.Request.Context(), site)
	if err != nil {
		resp.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	resp.Success(c, pricing)
}

func listSiteTypes(c *gin.Context) {
	resp.Success(c, model.AllSiteTypes())
}

func discoverSites(c *gin.Context) {
	sites, err := ldoh.DiscoverSites(c.Request.Context())
	if err != nil {
		resp.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	resp.Success(c, sites)
}
