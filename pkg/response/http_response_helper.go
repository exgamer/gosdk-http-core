package response

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/exgamer/gosdk-core/pkg/context"
	exception2 "github.com/exgamer/gosdk-core/pkg/exception"
	"net/http"
	"strings"

	"github.com/exgamer/gosdk-core/pkg/debug"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

const (
	ctxKeyException  = "exception"
	ctxKeyData       = "data"
	ctxKeyStatusCode = "status_code"
	ctxKeyRaw        = "raw_response"
	ctxKeyNoBody     = "no_body_response"
)

func mapKindToStatus(kind exception2.ErrorKind) int {
	switch kind {
	case exception2.ErrorKindValidation:
		return http.StatusUnprocessableEntity
	case exception2.ErrorKindNotFound:
		return http.StatusNotFound
	case exception2.ErrorKindForbidden:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

func ErrorResponseUntrackableSentry(c *gin.Context, statusCode int, err error, context map[string]any) {
	ErrorResponse(c, exception.NewUntrackableHttpException(statusCode, err, context))
}

func ErrorResponse(c *gin.Context, err error) {
	var httpEx *exception.HttpException
	if errors.As(err, &httpEx) {
		c.Set(ctxKeyException, httpEx)
		c.AbortWithStatus(httpEx.Code)

		return
	}

	var appEx *exception2.AppException
	if errors.As(err, &appEx) {
		code := mapKindToStatus(appEx.Kind)
		httpErr := exception.NewHttpException(
			code,
			appEx.Err,
			appEx.Context,
		)
		httpErr.TrackInSentry = appEx.TrackInSentry
		c.Set(ctxKeyException, httpErr)
		c.AbortWithStatus(httpErr.Code)

		return
	}

	httpErr := exception.NewInternalServerErrorException(err, nil)
	c.Set(ctxKeyException, httpErr)
	c.AbortWithStatus(httpErr.Code)
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

// SuccessAccepted — 202: запрос принят, обработка продолжается асинхронно
func SuccessAccepted(c *gin.Context, data any) {
	c.Set(ctxKeyData, data)
	c.Set(ctxKeyStatusCode, http.StatusAccepted)
}

func SuccessDeleted(c *gin.Context, data any) {
	c.Set(ctxKeyData, data)
	c.Set(ctxKeyStatusCode, http.StatusNoContent)
}

// NoContent Ответ без тела со статусом status (0 и меньше — 204): Formatted не пишет JSON-обёртку.
// Для ответов, где тело не нужно или запрещено протоколом (вебхук, back-channel logout, 204).
func NoContent(c *gin.Context, status int) {
	if status <= 0 {
		status = http.StatusNoContent
	}

	c.Set(ctxKeyNoBody, true)
	c.Set(ctxKeyStatusCode, status)
}

func FormattedSuccessResponse(c *gin.Context, data any) {
	Success(c, data)
	Formatted(c)
}

func Formatted(c *gin.Context) {
	// Обработчик уже записал тело сам (файл, текст, c.JSON): вторая запись дописала бы JSON к его телу.
	// Только заголовки (AbortWithStatus у ErrorResponse) — не тело: ответ пишется, как раньше.
	if c.Writer.Size() > 0 {
		return
	}

	// Запрос прерван (c.Abort, c.AbortWithStatus), а ответа в контексте нет — не успех.
	// Исключение в контекст не кладётся: лог и Sentry — как раньше (LoggerMiddleware — по статусу и c.Errors).
	if c.IsAborted() && !hasAnswer(c) {
		status := http.StatusInternalServerError
		if c.Writer.Written() {
			status = c.Writer.Status()
			// Middleware ответил сам статусом без тела (CORS preflight 204) — как есть.
			if status < http.StatusBadRequest {
				return
			}
		} else {
			// Голый c.Abort() — ошибка сервиса: 500, причина — в лог (LoggerMiddleware пишет c.Errors).
			_ = c.Error(ErrAbortedWithoutResponse)
		}

		writeError(c, exception.NewUntrackableHttpException(status, errors.New(http.StatusText(status)), nil))

		return
	}

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

		writeError(c, httpEx)

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

	if noBody, _ := c.Get(ctxKeyNoBody); noBody == true {
		c.Status(status)
		c.Writer.WriteHeaderNow()

		return
	}

	if isRaw(c) {
		writeJSON(c, status, data)

		return
	}

	writeJSON(c, status, wrapWithDebug(c, true, data))
}

// writeError Тело ошибки SDK (status, error, message, correlation_id, hostname, details) со статусом исключения.
func writeError(c *gin.Context, httpEx *exception.HttpException) {
	serviceName := "UNKNOWN (maybe you not used RequestMiddleware)"
	requestId := "UNKNOWN (maybe you not used RequestMiddleware)"

	if appInfo := context.GetAppInfoFromContext(c.Request.Context()); appInfo != nil {
		serviceName = appInfo.ServiceName
	}

	if httpInfo := gin2.GetHttpInfoFromContext(c.Request.Context()); httpInfo != nil {
		requestId = httpInfo.CorrelationId
	}

	message, details := publicErrorText(c, httpEx)

	responseData := gin.H{
		"status":         httpEx.Code,
		"error":          httpEx.GetErrorType(),
		"message":        message,
		"correlation_id": requestId,
		"hostname":       serviceName,
		"details":        details,
	}

	if isRaw(c) {
		writeJSON(c, httpEx.Code, responseData)

		return
	}

	writeJSON(c, httpEx.Code, wrapWithDebug(c, false, responseData))
}

// publicErrorText Сообщение и details для клиента. Ошибка сервера (5xx) — только текст статуса («Internal Server
// Error») без details: текст внутренней ошибки (адрес БД, пользователь, SQL) клиенту не уходит; полная ошибка — в лог
// и Sentry, как раньше. С HTTP_ERROR_DETAILS (gin.ErrorDetailsEnabled) — текст ошибки и details, как раньше. 4xx — как раньше.
func publicErrorText(c *gin.Context, httpEx *exception.HttpException) (string, map[string]any) {
	if httpEx.Code < http.StatusInternalServerError || gin2.ErrorDetailsEnabled(c) {
		return httpEx.Error(), httpEx.Context
	}

	text := http.StatusText(httpEx.Code)
	if text == "" {
		text = http.StatusText(http.StatusInternalServerError)
	}

	return text, nil
}

// ErrAbortedWithoutResponse Запрос прерван c.Abort() без ответа (response.* или ErrorResponse): FormattedResponseMiddleware отвечает 500.
var ErrAbortedWithoutResponse = errors.New("request aborted without response")

// hasAnswer В контексте есть ответ: ошибка, данные, статус или NoContent.
func hasAnswer(c *gin.Context) bool {
	for _, key := range []string{ctxKeyException, ctxKeyData, ctxKeyStatusCode, ctxKeyNoBody} {
		if _, ok := c.Get(key); ok {
			return true
		}
	}

	return false
}

func isRaw(c *gin.Context) bool {
	v, ok := c.Get(ctxKeyRaw)
	if !ok {
		return false
	}

	raw, ok := v.(bool)

	return ok && raw
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
			Debug:   maskedDebug(dbg),
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

// debugMask Замена секрета в debug-поле ответа.
const debugMask = "***"

// maskedDebug Debug-поле ответа без секретов: значения чувствительных ключей (gin.IsSensitiveKey) на любой глубине —
// заголовки, параметры, ответы — и секреты внутри строковых полей body (JSON или форма исходящего запроса) — «***».
// Коллектор не меняется. Ошибка сериализации — debug-поле пустое, а не сырые данные.
func maskedDebug(dbg *debug.DebugCollector) any {
	raw, err := json.Marshal(dbg)
	if err != nil {
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var v any
	if err := decoder.Decode(&v); err != nil {
		return nil
	}

	return maskBodies(gin2.MaskJSONKeys(v, debugMask))
}

// maskBodies Строковые значения ключа body — через gin.MaskBodyString.
func maskBodies(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for key, value := range t {
			if s, ok := value.(string); ok && strings.EqualFold(key, "body") {
				t[key] = gin2.MaskBodyString(s, debugMask)

				continue
			}

			t[key] = maskBodies(value)
		}
	case []any:
		for i, value := range t {
			t[i] = maskBodies(value)
		}
	}

	return v
}
