package gin

import (
	"errors"
	"fmt"

	"github.com/exgamer/gosdk-core/pkg/context"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
)

// CaptureToSentry отправляет ошибку в Sentry с контекстом запроса.
// Вызывается из SentryMiddleware (обычные ошибки) и ErrorHandler (panic).
func CaptureToSentry(c *gin.Context, err error) {
	var httpEx *exception.HttpException
	if !errors.As(err, &httpEx) {
		httpEx = exception.NewInternalServerErrorException(err, nil)
	}

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

	httpInfo := GetHttpInfoFromContext(c.Request.Context())
	if httpInfo != nil {
		requestId = httpInfo.RequestId
	}

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

		sentry.CaptureException(err)
	})
}
