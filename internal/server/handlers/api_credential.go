package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/credential"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

func init() {
	router.NewGroupRouter("/api/v1/api-credential").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermAPIKeysRead)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/list", http.MethodGet).
				Handle(listCredentials),
		).
		AddRoute(
			router.NewRoute("/create", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermAPIKeysWrite)).
				Handle(createCredential),
		).
		AddRoute(
			router.NewRoute("/update", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermAPIKeysWrite)).
				Handle(updateCredential),
		).
		AddRoute(
			router.NewRoute("/delete/:id", http.MethodDelete).
				Use(middleware.RequirePermission(auth.PermAPIKeysWrite)).
				Handle(deleteCredential),
		).
		AddRoute(
			router.NewRoute("/api-types", http.MethodGet).
				Handle(listAPITypes),
		).
		AddRoute(
			router.NewRoute("/cli-tools", http.MethodGet).
				Handle(listCLITools),
		)
}

func listCredentials(c *gin.Context) {
	profiles, err := credential.List(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	if isViewerRole(c.GetString("user_role")) {
		redactCredentialBaseURLsForViewer(profiles)
	}
	resp.Success(c, profiles)
}

func createCredential(c *gin.Context) {
	var req model.APICredentialCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	p, err := credential.Create(c.Request.Context(), &req)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, p)
}

func updateCredential(c *gin.Context) {
	var req model.APICredentialUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	p, err := credential.Update(c.Request.Context(), &req)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, p)
}

func deleteCredential(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	if err := credential.Delete(c.Request.Context(), id); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, nil)
}

func listAPITypes(c *gin.Context) {
	resp.Success(c, model.AllAPITypes())
}

func listCLITools(c *gin.Context) {
	resp.Success(c, model.AllCLITools())
}
