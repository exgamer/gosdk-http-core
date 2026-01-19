package response

import (
	"encoding/json"
	"errors"
	"github.com/exgamer/gosdk-core/pkg/context"
	exception2 "github.com/exgamer/gosdk-core/pkg/exception"
	"net/http"

	"github.com/exgamer/gosdk-core/pkg/debug"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

const (
	ctxKeyException  = "exception"
	ctxKeyData       = "data"
	ctxKeyStatusCode = "status_code"
)

func ErrorResponseUntrackableSentry(c *gin.Context, statusCode int, err error, context map[string]any) {
	ErrorResponse(c, exception.NewUntrackableHttpException(statusCode, err, context))
}

func ErrorResponse(c *gin.Context, err error) {
	c.Set(ctxKeyException, err)

	var httpEx *exception.HttpException
	if !errors.As(err, &httpEx) {
		httpEx = exception.NewInternalServerErrorException(err, nil)
	}

	c.AbortWithStatus(httpEx.Code)
}

func ErrorResponseWithStatus(c *gin.Context, statusCode int, err error, context map[string]any) {
	// 1. Если это уже HttpException → используем как есть
	var httpEx *exception.HttpException
	if errors.As(err, &httpEx) {
		ErrorResponse(c, err)

		return
	}

	// 2. Если это AppException → превращаем в HttpException, сохранив поля
	var appEx *exception2.AppException
	if errors.As(err, &appEx) {
		httpErr := exception.NewHttpException(statusCode, appEx.Err, appEx.Context)
		httpErr.TrackInSentry = appEx.TrackInSentry
		ErrorResponse(c, httpErr)

		return
	}

	// 3. Обычная ошибка → оборачиваем стандартно
	ErrorResponse(c, exception.NewHttpException(statusCode, err, context))
}

// ---------- базовые (error) ----------

func BadRequest(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatus(c, http.StatusBadRequest, err, ctx)
}

func Unauthorized(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatus(c, http.StatusUnauthorized, err, ctx)
}

func Forbidden(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatus(c, http.StatusForbidden, err, ctx)
}

func NotFound(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatus(c, http.StatusNotFound, err, ctx)
}

func Conflict(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatus(c, http.StatusConflict, err, ctx)
}

func UnprocessableEntity(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatus(c, http.StatusUnprocessableEntity, err, ctx)
}

func TooManyRequests(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatus(c, http.StatusTooManyRequests, err, ctx)
}

func InternalServerError(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatus(c, http.StatusInternalServerError, err, ctx)
}

func Success(c *gin.Context, data any) {
	c.Set(ctxKeyData, data)
}

func SuccessCreated(c *gin.Context, data any) {
	c.Set(ctxKeyData, data)
	c.Set(ctxKeyStatusCode, http.StatusCreated)
}

func SuccessDeleted(c *gin.Context, data any) {
	c.Set(ctxKeyData, data)
	c.Set(ctxKeyStatusCode, http.StatusNoContent)
}

func FormattedSuccessResponse(c *gin.Context, data any) {
	Success(c, data)
	Formatted(c)
}

func Formatted(c *gin.Context) {
	// ---- ERROR PATH ----
	if exObj, exists := c.Get(ctxKeyException); exists {
		err, ok := exObj.(error)
		if !ok {
			err = errors.New("exception in context is not error")
		}

		var httpEx *exception.HttpException
		if !errors.As(err, &httpEx) {
			httpEx = exception.NewInternalServerErrorException(err, nil)
		}

		serviceName := "UNKNOWN (maybe you not used RequestMiddleware)"
		requestId := "UNKNOWN (maybe you not used RequestMiddleware)"

		if appInfo := context.GetAppInfoFromContext(c.Request.Context()); appInfo != nil {
			serviceName = appInfo.ServiceName
		}

		if httpInfo := gin2.GetHttpInfoFromContext(c.Request.Context()); httpInfo != nil {
			requestId = httpInfo.RequestId
		}

		responseData := gin.H{
			"status":     httpEx.Code,
			"error":      httpEx.GetErrorType(),
			"message":    httpEx.Error(),
			"request_id": requestId,
			"hostname":   serviceName,
			"details":    httpEx.Context,
		}

		writeJSON(c, httpEx.Code, wrapWithDebug(c, false, responseData))

		return
	}

	// ---- SUCCESS PATH ----
	data, _ := c.Get(ctxKeyData)

	status := http.StatusOK
	if v, ok := c.Get(ctxKeyStatusCode); ok {
		if sc, ok := v.(int); ok && sc > 0 {
			status = sc
		}
	}

	writeJSON(c, status, wrapWithDebug(c, true, data))
}

func wrapWithDebug(c *gin.Context, success bool, data any) any {
	if dbg := debug.GetDebugFromContext(c.Request.Context()); dbg != nil {
		dbg.CalculateTotalTime()
		return struct {
			Success bool `json:"success"`
			Data    any  `json:"data"`
			Debug   any  `json:"debug"`
		}{
			Success: success,
			Data:    data,
			Debug:   dbg,
		}
	}

	return struct {
		Success bool `json:"success"`
		Data    any  `json:"data"`
	}{
		Success: success,
		Data:    data,
	}
}

func writeJSON(c *gin.Context, status int, payload any) {
	b, err := json.Marshal(payload)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal JSON"})
		return
	}
	c.Data(status, "application/json", b)
}
