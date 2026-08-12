package middleware

import (
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

// corsOriginAllowed 判断 origin 是否命中白名单配置。
// allowed 取值：空（禁跨域）、"*"（全放行）、逗号分隔的域名列表。
func corsOriginAllowed(allowed, origin string) bool {
	allowed = strings.TrimSpace(allowed)
	if allowed == "" {
		return false
	}
	if allowed == "*" {
		return true
	}

	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}

	// 提取 origin 的 host 部分用于匹配
	originHost := origin
	if idx := strings.Index(origin, "://"); idx != -1 {
		originHost = origin[idx+3:]
	}
	originHost = strings.TrimRight(originHost, "/")

	for _, item := range strings.Split(allowed, ",") {
		item = strings.TrimSpace(item)
		item = strings.TrimRight(item, "/")
		if item == "" {
			continue
		}
		// 支持完整 origin (https://example.com) 或仅域名 (example.com)
		if item == origin || item == originHost {
			return true
		}
	}
	return false
}

func Cors() gin.HandlerFunc {
	buildConfig := func(allowCredentials bool) cors.Config {
		config := cors.DefaultConfig()
		config.AllowCredentials = allowCredentials
		config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
		config.AllowHeaders = []string{"*"}
		config.ExposeHeaders = []string{"Content-Disposition"}
		// CORS 白名单:
		// - 为空: 不允许跨域
		// - "*": 允许所有来源
		// - 逗号分隔的域名列表: 只允许指定的域名 (如 "https://example.com,https://example2.com")
		config.AllowOriginFunc = func(origin string) bool {
			allowed, err := setting.GetString(model.SettingKeyCORSAllowOrigins)
			if err != nil {
				return false
			}
			return corsOriginAllowed(allowed, origin)
		}
		return config
	}
	// "*" 与 AllowCredentials=true 是危险组合：任意站点都能以用户浏览器身份
	// （含凭据）跨域调用 API。星号模式下禁用凭据；白名单模式保留凭据。
	handlerWithCredentials := cors.New(buildConfig(true))
	handlerWithoutCredentials := cors.New(buildConfig(false))

	return func(c *gin.Context) {
		allowed, err := setting.GetString(model.SettingKeyCORSAllowOrigins)
		if err != nil {
			allowed = ""
		}
		// Private Network Access (PNA): Chrome requires this header when a
		// public website (e.g. claude.ai) accesses private/local addresses.
		// 仅在 origin 命中白名单时授予，避免为任意来源访问内网部署开绿灯。
		if c.GetHeader("Access-Control-Request-Private-Network") == "true" &&
			corsOriginAllowed(allowed, c.GetHeader("Origin")) {
			c.Header("Access-Control-Allow-Private-Network", "true")
		}
		if strings.TrimSpace(allowed) == "*" {
			handlerWithoutCredentials(c)
		} else {
			handlerWithCredentials(c)
		}
	}
}
