package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/server/resp"
)

func RequireJSON() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet ||
			c.Request.Method == http.MethodDelete ||
			c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		// 无 body 的请求（如 POST /refresh/:id）不需要检查 Content-Type
		if c.Request.ContentLength <= 0 {
			c.Next()
			return
		}

		contentType := c.GetHeader("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			resp.Error(c, http.StatusUnsupportedMediaType, resp.ErrInvalidJSON)
			c.Abort()
			return
		}

		c.Next()
	}
}
