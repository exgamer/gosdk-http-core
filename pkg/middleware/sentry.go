package middleware

import (
	"errors"
	"fmt"

	"github.com/exgamer/gosdk-core/pkg/context"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
)

// SentryMiddleware Middleware для обработки ошибок в sentry
func SentryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		exObj, exists := c.Get("exception")
		if !exists {
			return
		}

		// Вытаскиваем error
		err, ok := exObj.(error)
		if !ok {
			err = fmt.Errorf("exception in context is not error: %T", exObj)
		}

		// Пытаемся привести к HttpException
		var httpEx *exception.HttpException
		if !errors.As(err, &httpEx) {
			httpEx = exception.NewInternalServerErrorException(err, nil)
		}

		// Уважаем флаг TrackInSentry
		if !httpEx.TrackInSentry {
			return
		}

		serviceName := "UNKNOWN (maybe you not used RequestMiddleware)"
		requestId := "UNKNOWN (maybe you not used RequestMiddleware)"

		appEnv := "UNKNOWN"

		appInfo := context.GetAppInfoFromContext(c.Request.Context())
		if appInfo != nil {
			serviceName = appInfo.ServiceName
			appEnv = appInfo.AppEnv
		}

		httpInfo := gin2.GetHttpInfoFromContext(c.Request.Context())
		if httpInfo != nil {
			requestId = httpInfo.RequestId
		}

		// То, что реально ушло клиенту
		status := c.Writer.Status()
		if status == 0 {
			status = httpEx.Code
		}

		responseData := gin.H{
			"status":     status,
			"error":      httpEx.GetErrorType(),
			"message":    httpEx.Error(),
			"request_id": requestId,
			"hostname":   serviceName,
			"details":    httpEx.Context,
		}

		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("environment", appEnv)
			// Заголовки — лучше санитайзить (минимум: Authorization/Cookie) TODO расиширить
			mapHeaders := make(map[string]any, len(c.Request.Header))
			for key, values := range c.Request.Header {
				if key == "Authorization" || key == "Cookie" {
					mapHeaders[fmt.Sprintf("header_%s", key)] = "*****"

					continue
				}

				if len(values) > 0 {
					mapHeaders[fmt.Sprintf("header_%s", key)] = values[0]
				}
			}
			scope.SetContext("header", mapHeaders)

			// Query параметры — тоже осторожно с token/secret TODO расиширить
			mapQueries := make(map[string]any)
			for key, values := range c.Request.URL.Query() {
				if key == "token" || key == "access_token" {
					mapQueries[fmt.Sprintf("query_%s", key)] = "*****"

					continue
				}
				if len(values) > 0 {
					mapQueries[fmt.Sprintf("query_%s", key)] = values[0]
				}
			}

			scope.SetContext("query", mapQueries)

			if httpEx.Code >= 400 && httpEx.Code < 500 {
				scope.SetLevel(sentry.LevelWarning)
			} else {
				scope.SetLevel(sentry.LevelError)
			}

			scope.SetContext("error", responseData)

			// В Sentry отправляем исходную ошибку
			sentry.CaptureException(err)
		})
	}
}
