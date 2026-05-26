package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	usr "github.com/lingyuins/octopus/internal/op/user"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

func init() {
	publicUserRoutes := router.NewGroupRouter("/api/v1/user").
		Use(middleware.RequireJSON())

	publicUserRoutes.AddRoute(
		router.NewRoute("/login", http.MethodPost).
			Use(middleware.LoginRateLimit()).
			Handle(login),
	)

	router.NewGroupRouter("/api/v1/user").
		Use(middleware.Auth()).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/create", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermUsersWrite)).
				Handle(createUser),
		).
		AddRoute(
			router.NewRoute("/change-password", http.MethodPost).
				Handle(changePassword),
		).
		AddRoute(
			router.NewRoute("/change-username", http.MethodPost).
				Handle(changeUsername),
		).
		AddRoute(
			router.NewRoute("/status", http.MethodGet).
				Handle(status),
		).
		AddRoute(
			router.NewRoute("/list", http.MethodGet).
				Use(middleware.RequirePermission(auth.PermUsersRead)).
				Handle(listUsers),
		).
		AddRoute(
			router.NewRoute("/update-role", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermUsersWrite)).
				Handle(updateUserRole),
		).
		AddRoute(
			router.NewRoute("/delete/:id", http.MethodDelete).
				Use(middleware.RequirePermission(auth.PermUsersWrite)).
				Handle(deleteUser),
		)
}

func createUser(c *gin.Context) {
	var req model.UserCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	if err := usr.Create(req, c.Request.Context()); err != nil {
		resp.Error(c, http.StatusBadRequest, "failed to create user")
		return
	}
	resp.Success(c, nil)
}

func listUsers(c *gin.Context) {
	users, err := usr.List(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, users)
}

func updateUserRole(c *gin.Context) {
	var req struct {
		ID   uint   `json:"id" binding:"required"`
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	if err := usr.UpdateRole(req.ID, req.Role, c.Request.Context()); err != nil {
		if status, msg, ok := classifyUserMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.Error(c, http.StatusBadRequest, "failed to update role")
		return
	}
	resp.Success(c, nil)
}

func deleteUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid user id")
		return
	}
	currentUserID := uint(c.GetInt("user_id"))
	if err := usr.Delete(uint(id), currentUserID, c.Request.Context()); err != nil {
		if status, msg, ok := classifyUserMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.Error(c, http.StatusBadRequest, "failed to delete user")
		return
	}
	resp.Success(c, nil)
}

func login(c *gin.Context) {
	var user model.UserLogin
	if err := c.ShouldBindJSON(&user); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	loginKey := c.GetString("login_rate_limit_key")
	userObj, err := usr.Verify(user.Username, user.Password)
	if err != nil {
		if errors.Is(err, usr.ErrBootstrapAlreadySetUp) {
			resp.Error(c, http.StatusConflict, err.Error())
			return
		}
		if isTransientDatabaseError(err) {
			resp.Error(c, http.StatusServiceUnavailable, resp.ErrDatabase)
			return
		}
		if !isCredentialError(err) {
			resp.Error(c, http.StatusInternalServerError, resp.ErrDatabase)
			return
		}
		middleware.RecordLoginFailure(loginKey, time.Now())
		resp.Error(c, http.StatusUnauthorized, resp.ErrUnauthorized)
		return
	}
	middleware.ClearLoginFailures(loginKey)
	token, expire, err := auth.GenerateJWTToken(user.Expire, userObj.ID, userObj.Role)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, resp.ErrInternalServer)
		return
	}
	resp.Success(c, model.UserLoginResponse{Token: token, ExpireAt: expire})
}

func isTransientDatabaseError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") || strings.Contains(msg, "sqlite_busy") || strings.Contains(msg, "sql: database is closed")
}

func isCredentialError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "incorrect username") || strings.Contains(msg, "incorrect password")
}

func changePassword(c *gin.Context) {
	var user model.UserChangePassword
	if err := c.ShouldBindJSON(&user); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	currentUserID := uint(c.GetInt("user_id"))
	if err := usr.ChangePassword(currentUserID, user.OldPassword, user.NewPassword); err != nil {
		if strings.Contains(err.Error(), "incorrect old password") {
			resp.Error(c, http.StatusUnauthorized, resp.ErrUnauthorized)
			return
		}
		resp.Error(c, http.StatusInternalServerError, resp.ErrDatabase)
		return
	}
	resp.Success(c, "password changed successfully")
}

func changeUsername(c *gin.Context) {
	var user model.UserChangeUsername
	if err := c.ShouldBindJSON(&user); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	currentUserID := uint(c.GetInt("user_id"))
	if err := usr.ChangeUsername(currentUserID, user.NewUsername); err != nil {
		if strings.Contains(err.Error(), "same as the old username") || strings.Contains(err.Error(), "username already exists") || strings.Contains(err.Error(), "username is required") {
			resp.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, "username changed successfully")
}

func status(c *gin.Context) {
	if !usr.Ready() {
		resp.Error(c, http.StatusConflict, usr.ErrBootstrapAlreadySetUp.Error())
		return
	}
	resp.Success(c, "ok")
}

func classifyUserMutationError(err error) (int, string, bool) {
	if err == nil {
		return 0, "", false
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "user not found"):
		return http.StatusNotFound, "user not found", true
	case strings.Contains(msg, "invalid role"):
		return http.StatusBadRequest, err.Error(), true
	case strings.Contains(msg, "username already exists"):
		return http.StatusConflict, "username already exists", true
	case strings.Contains(msg, "cannot delete the active user"):
		return http.StatusBadRequest, "cannot delete the active user", true
	default:
		return 0, "", false
	}
}
