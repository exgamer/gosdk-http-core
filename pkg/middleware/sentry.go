package middleware

import (
	"fmt"
	config2 "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/exgamer/gosdk-core/pkg/constants"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	constants2 "github.com/exgamer/gosdk-http-core/pkg/constants"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
)

// SentryMiddleware Middleware для обработки ошибок в sentry
func SentryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		appExceptionObject, exists := c.Get("exception")
		if !exists {
			return
		}

		appException := exception.HttpException{}
		mapstructure.Decode(appExceptionObject, &appException)
		serviceName := "UNKNOWN (maybe you not used RequestMiddleware)"
		requestId := "UNKNOWN (maybe you not used RequestMiddleware)"
		value, exists := c.Get(constants.AppInfoKey)
		if exists {
			appInfo := value.(*config2.AppInfo)
			serviceName = appInfo.ServiceName
		}

		value, exists = c.Get(constants2.HttpInfoKey)

		if exists {
			httpInfo := value.(*config.HttpInfo)
			requestId = httpInfo.RequestId
		}

		responseData := gin.H{
			"status":     appException.Code,
			"error":      appException.GetErrorType(),
			"message":    appException.Error.Error(),
			"request_id": requestId,
			"hostname":   serviceName,
			"details":    appException.Context,
		}

		sentry.WithScope(func(scope *sentry.Scope) {
			// Добавляем заголовки запроса
			mapHeaders := make(map[string]any)
			for key, values := range c.Request.Header {
				for _, value := range values {
					mapHeaders[fmt.Sprintf("header_%s", key)] = value
				}
			}
			scope.SetContext("header", mapHeaders)

			// Добавляем Query параметры
			mapQueries := make(map[string]any)
			for key, values := range c.Request.URL.Query() {
				for _, value := range values {
					mapQueries[fmt.Sprintf("query_%s", key)] = value
				}
			}
			scope.SetContext("query", mapQueries)

			if appException.Code >= 400 && appException.Code < 500 {
				scope.SetLevel(sentry.LevelWarning)
			} else {
				scope.SetLevel(sentry.LevelError)
			}

			// Захватываем ошибку
			scope.SetContext("error", responseData)

			sentry.CaptureException(appException.Error)
		})
	}
}
