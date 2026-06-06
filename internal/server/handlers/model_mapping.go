package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/modelmapping"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

func init() {
	router.NewGroupRouter("/api/v1/model-mapping").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSettingsRead)).
		AddRoute(router.NewRoute("", http.MethodGet).Handle(listModelMappings)).
		AddRoute(router.NewRoute("/:id", http.MethodGet).Handle(getModelMapping)).
		AddRoute(router.NewRoute("", http.MethodPost).
			Use(middleware.RequirePermission(auth.PermSettingsWrite)).
			Handle(createModelMapping)).
		AddRoute(router.NewRoute("/:id", http.MethodPut).
			Use(middleware.RequirePermission(auth.PermSettingsWrite)).
			Handle(updateModelMapping)).
		AddRoute(router.NewRoute("/:id", http.MethodDelete).
			Use(middleware.RequirePermission(auth.PermSettingsWrite)).
			Handle(deleteModelMapping))
}

func listModelMappings(c *gin.Context) {
	mappings, err := modelmapping.List(c.Request.Context())
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, mappings)
}

func getModelMapping(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	mapping, err := modelmapping.Get(c.Request.Context(), id)
	if err != nil {
		resp.Error(c, http.StatusNotFound, "model mapping not found")
		return
	}

	resp.Success(c, mapping)
}

func createModelMapping(c *gin.Context) {
	var req model.ModelMappingCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Validate match type
	if req.MatchType != model.MatchExact && req.MatchType != model.MatchWildcard && req.MatchType != model.MatchRegex {
		resp.Error(c, http.StatusBadRequest, "invalid match_type: must be exact, wildcard, or regex")
		return
	}

	mapping, err := modelmapping.Create(c.Request.Context(), &req)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	resp.Success(c, mapping)
}

func updateModelMapping(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req model.ModelMappingUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Validate match type if provided
	if req.MatchType != nil {
		if *req.MatchType != model.MatchExact && *req.MatchType != model.MatchWildcard && *req.MatchType != model.MatchRegex {
			resp.Error(c, http.StatusBadRequest, "invalid match_type: must be exact, wildcard, or regex")
			return
		}
	}

	mapping, err := modelmapping.Update(c.Request.Context(), id, &req)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	resp.Success(c, mapping)
}

func deleteModelMapping(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	if err := modelmapping.Delete(c.Request.Context(), id); err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	resp.Success(c, nil)
}
