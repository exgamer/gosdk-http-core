package middleware

import (
	"net/http"
	"strings"

	"github.com/exgamer/gosdk-core/pkg/app"
	di2 "github.com/exgamer/gosdk-http-core/pkg/di"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

// CorsMiddleware разрешает cross-origin запросы.
// Origins берутся из env CORS_ALLOWED_ORIGINS (список через запятую).
// Значение "*" разрешает все origins.
func CorsMiddleware(a *app.App) gin.HandlerFunc {
	httpConfig, _ := di2.GetHttpConfig(a.Container)

	return func(c *gin.Context) {
		httpInfo := gin2.GetInstanceHttpInfo(c)

		if httpInfo.RequestOrigin == "" {
			c.Next()

			return
		}

		if httpConfig == nil {
			c.Next()

			return
		}

		raw := strings.TrimSpace(httpConfig.CorsAllowedOrigins)
		if raw == "" {
			c.Next()

			return
		}

		if !originAllowed(raw, httpInfo.RequestOrigin) {
			c.Next()

			return
		}

		c.Header("Access-Control-Allow-Origin", httpInfo.RequestOrigin)
		c.Header("Vary", "Origin")

		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-Id")
		c.Header("Access-Control-Max-Age", "86400")

		if httpInfo.RequestMethod == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)

			return
		}

		c.Next()
	}
}

func originAllowed(raw, origin string) bool {
	if raw == "*" {
		return true
	}

	for _, o := range strings.Split(raw, ",") {
		if strings.TrimSpace(o) == origin {
			return true
		}
	}

	return false
}
