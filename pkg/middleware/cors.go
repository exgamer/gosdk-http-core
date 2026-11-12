package middleware

import (
	"net/http"
	"strings"

	"github.com/exgamer/gosdk-core/pkg/app"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	di2 "github.com/exgamer/gosdk-http-core/pkg/di"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

// corsBaseAllowedHeaders Базовый список Access-Control-Allow-Headers.
var corsBaseAllowedHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-Id"}

// CorsMiddleware разрешает cross-origin запросы.
// Origins берутся из env CORS_ALLOWED_ORIGINS (список через запятую).
// Значение "*" разрешает все origins.
// Дополнительные разрешённые заголовки — env CORS_EXTRA_ALLOWED_HEADERS (список через запятую).
func CorsMiddleware(a *app.App) gin.HandlerFunc {
	httpConfig, _ := di2.GetHttpConfig(a.Container)

	return corsHandler(httpConfig)
}

func corsHandler(httpConfig *config.HttpConfig) gin.HandlerFunc {
	allowHeaders := ""
	if httpConfig != nil {
		allowHeaders = corsAllowedHeaders(httpConfig.CorsExtraAllowedHeaders)
	}

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
		c.Header("Access-Control-Allow-Headers", allowHeaders)
		c.Header("Access-Control-Max-Age", "86400")

		if httpInfo.RequestMethod == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)

			return
		}

		c.Next()
	}
}

// corsAllowedHeaders Базовый список плюс дополнительные заголовки: пустые элементы и повторы (без учёта регистра)
// пропускаются, порядок сохраняется.
func corsAllowedHeaders(extra string) string {
	headers := make([]string, 0, len(corsBaseAllowedHeaders))
	seen := make(map[string]struct{}, len(corsBaseAllowedHeaders))

	add := func(h string) {
		h = strings.TrimSpace(h)
		if h == "" {
			return
		}

		key := strings.ToLower(h)
		if _, ok := seen[key]; ok {
			return
		}

		seen[key] = struct{}{}
		headers = append(headers, h)
	}

	for _, h := range corsBaseAllowedHeaders {
		add(h)
	}

	for _, h := range strings.Split(extra, ",") {
		add(h)
	}

	return strings.Join(headers, ", ")
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
