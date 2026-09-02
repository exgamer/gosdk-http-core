package gin

import (
	"errors"
	"fmt"

	"github.com/exgamer/gosdk-core/pkg/context"
	"github.com/exgamer/gosdk-core/pkg/errorreporter"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	"github.com/gin-gonic/gin"
)

// CaptureToSentry отправляет ошибку в error-трекер (Sentry и т.п.) с
// контекстом запроса через errorreporter.Capture. Сам пакет ничего не
// знает про Sentry - реальную отправку делает адаптер, зарегистрированный
// через errorreporter.SetReporter (см. gosdk-sentry-core). Без него вызов
// безопасен и просто ничего не отправляет.
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

	appInfo := context.GetAppInfoFromContext(c.Request.Context())
	if appInfo != nil {
		serviceName = appInfo.ServiceName
	}

	httpInfo := GetHttpInfoFromContext(c.Request.Context())
	if httpInfo != nil {
		requestId = httpInfo.RequestId
	}

	status := c.Writer.Status()
	if status == 0 {
		status = httpEx.Code
	}

	responseData := map[string]any{
		"status":     status,
		"error":      httpEx.GetErrorType(),
		"message":    httpEx.Error(),
		"request_id": requestId,
		"hostname":   serviceName,
		"details":    httpEx.Context,
	}

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

	level := errorreporter.LevelError
	if httpEx.Code >= 400 && httpEx.Code < 500 {
		level = errorreporter.LevelWarning
	}

	errorreporter.Capture(c.Request.Context(), err, errorreporter.Options{
		Level: level,
		Extra: map[string]any{
			"header": mapHeaders,
			"query":  mapQueries,
			"error":  responseData,
		},
	})
}
