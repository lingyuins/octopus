package middleware

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds common security-related HTTP response headers.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy", buildContentSecurityPolicy(c.GetHeader("Origin")))
		c.Next()
	}
}

func buildContentSecurityPolicy(requestOrigin string) string {
	connectSrc := []string{"'self'"}
	if origin := normalizeCSPOrigin(requestOrigin); origin != "" {
		connectSrc = append(connectSrc, origin)
	}
	return "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src " + strings.Join(connectSrc, " ") + "; font-src 'self'; object-src 'none'"
}

func normalizeCSPOrigin(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
