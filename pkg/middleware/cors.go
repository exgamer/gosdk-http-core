package middleware

import (
	"net/http"
	"strings"

	"github.com/exgamer/gosdk-core/pkg/app"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
	di2 "github.com/exgamer/gosdk-http-core/pkg/di"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

// corsBaseAllowedHeaders Базовый список Access-Control-Allow-Headers.
// X-Correlation-Id — заголовок SDK (constants.CorrelationIdHeaderName): фронт передаёт свой id корреляции;
// X-Request-Id и Request-Id — прежние, тоже принимаются на входе.
var corsBaseAllowedHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", constants.CorrelationIdHeaderName,
	constants.XRequestIdHeaderName, "Request-Id"}

// CorsMiddleware разрешает cross-origin запросы.
// Origins берутся из env CORS_ALLOWED_ORIGINS (список через запятую).
// Значение "*" разрешает все origins без Access-Control-Allow-Credentials (с ним любой сайт делал бы запросы с cookie
// пользователя); cookie и credentials — только для origins из явного списка. Origin "null" с credentials не разрешается.
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

		credentials := raw != "*"
		if credentials && httpInfo.RequestOrigin == "null" {
			c.Next()

			return
		}

		c.Header("Access-Control-Allow-Origin", httpInfo.RequestOrigin)
		addVary(c.Writer.Header(), "Origin")
		// X-Correlation-Id ответа читается скриптом фронта (поиск запроса в логах по id).
		c.Writer.Header().Add("Access-Control-Expose-Headers", constants.CorrelationIdHeaderName)

		if credentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

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

// addVary Добавляет value в Vary, не затирая значения, выставленные раньше (Accept-Encoding от gzip и т. п.);
// повтор (без учёта регистра) не добавляется.
func addVary(h http.Header, value string) {
	for _, line := range h.Values("Vary") {
		for _, v := range strings.Split(line, ",") {
			if strings.EqualFold(strings.TrimSpace(v), value) {
				return
			}
		}
	}

	h.Add("Vary", value)
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
