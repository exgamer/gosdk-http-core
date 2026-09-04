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

	// c.FullPath() - шаблон роута ("/city/:id"), а не реальный URL с
	// подставленным id - так однотипные ошибки на разных id группируются
	// в Sentry по одному эндпойнту. Пустая строка бывает для несматченных
	// роутов (404) - тогда используем реальный путь запроса.
	route := c.FullPath()
	if route == "" {
		route = c.Request.URL.Path
	}
	endpoint := c.Request.Method + " " + route

	responseData := map[string]any{
		"status":     status,
		"error":      httpEx.GetErrorType(),
		"message":    httpEx.Error(),
		"request_id": requestId,
		"hostname":   serviceName,
		"endpoint":   endpoint,
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
		// endpoint тегом (не только в Extra) - чтобы можно было
		// фильтровать/группировать issues в Sentry по конкретному
		// эндпойнту, а не только видеть его внутри деталей события.
		Tags: map[string]string{
			"endpoint": endpoint,
		},
		Extra: map[string]any{
			"header": mapHeaders,
			"query":  mapQueries,
			"error":  responseData,
		},
	})
}
