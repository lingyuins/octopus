package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	usr "github.com/lingyuins/octopus/internal/op/user"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

func init() {
	router.NewGroupRouter("/api/v1/bootstrap").
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/status", http.MethodGet).
				Handle(getBootstrapStatus),
		).
		AddRoute(
			router.NewRoute("/create-admin", http.MethodPost).
				Use(middleware.LoginRateLimit()).
				Handle(createBootstrapAdmin),
		)
}

func getBootstrapStatus(c *gin.Context) {
	initialized, message, err := usr.BootstrapStatus()
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, resp.ErrInternalServer)
		return
	}
	resp.Success(c, gin.H{
		"initialized": initialized,
		"message":     message,
	})
}

func createBootstrapAdmin(c *gin.Context) {
	var user model.UserBootstrapCreate
	if err := c.ShouldBindJSON(&user); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	if err := usr.BootstrapCreate(user.Username, user.Password); err != nil {
		if errors.Is(err, usr.ErrBootstrapAlreadySetUp) {
			resp.Error(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, usr.ErrBootstrapCredentials) {
			messageKey, messageArgs := bootstrapCredentialMessage(err)
			resp.ErrorWithKey(c, http.StatusBadRequest, err.Error(), messageKey, messageArgs)
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, gin.H{"initialized": true})
}

func bootstrapCredentialMessage(err error) (string, map[string]any) {
	message := strings.ToLower(err.Error())

	switch {
	case strings.Contains(message, "username is required"):
		return "bootstrap.validation.usernameRequired", nil
	case strings.Contains(message, "password is required"):
		return "bootstrap.validation.passwordRequired", nil
	case strings.Contains(message, "password must be at least"):
		return "bootstrap.validation.passwordTooShort", map[string]any{"count": 12}
	default:
		return "bootstrap.error.generic", nil
	}
}
