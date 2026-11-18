package response

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/exgamer/gosdk-core/pkg/context"
	exception2 "github.com/exgamer/gosdk-core/pkg/exception"
	"net/http"
	"reflect"
	"strings"

	"github.com/exgamer/gosdk-core/pkg/debug"
	"github.com/exgamer/gosdk-http-core/internal/publictext"
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
		// Код выведен из вида ошибки, а не задан явно: текст 5xx клиенту не уходит.
		httpErr := derivedHttpException(mapKindToStatus(appEx.Kind), appEx.Err, appEx.Context)
		httpErr.TrackInSentry = appEx.TrackInSentry
		c.Set(ctxKeyException, httpErr)
		c.AbortWithStatus(httpErr.Code)

		return
	}

	httpErr := exception.NewInternalServerErrorException(err, nil)
	c.Set(ctxKeyException, httpErr)
	c.AbortWithStatus(httpErr.Code)
}

// ErrorResponseWithStatus Ошибка со статусом, заданным сервисом явно: при 5xx текст простой ошибки (без вложенной
// причины) уходит клиенту, ошибка с причиной — текст статуса.
func ErrorResponseWithStatus(c *gin.Context, statusCode int, err error, context map[string]any) {
	errorResponseWithStatus(c, statusCode, err, context, true)
}

// errorResponseWithStatus explicit — статус задан сервисом явно (ErrorResponseWithStatus), а не именем функции
// (InternalServerError): от него зависит, уйдёт ли клиенту текст 5xx.
func errorResponseWithStatus(c *gin.Context, statusCode int, err error, context map[string]any, explicit bool) {
	// 1. Если это уже HttpException → используем как есть
	var httpEx *exception.HttpException
	if errors.As(err, &httpEx) {
		ErrorResponse(c, err)

		return
	}

	// 2. Если это AppException → превращаем в HttpException, сохранив поля
	var appEx *exception2.AppException
	if errors.As(err, &appEx) {
		httpErr := newHttpException(statusCode, appEx.Err, appEx.Context, explicit)
		httpErr.TrackInSentry = appEx.TrackInSentry
		ErrorResponse(c, httpErr)

		return
	}

	// 3. Обычная ошибка → оборачиваем стандартно
	ErrorResponse(c, newHttpException(statusCode, err, context, explicit))
}

// newHttpException Исключение с явным кодом (NewHttpException) или с кодом, выведенным SDK.
func newHttpException(code int, err error, context map[string]any, explicit bool) *exception.HttpException {
	if explicit {
		return exception.NewHttpException(code, err, context)
	}

	return derivedHttpException(code, err, context)
}

// derivedHttpException Исключение с кодом, выведенным SDK (из вида ошибки или имени функции): признак явного кода
// не ставится — текст 5xx клиенту не уходит.
func derivedHttpException(code int, err error, context map[string]any) *exception.HttpException {
	return &exception.HttpException{Err: err, Context: context, Code: code, TrackInSentry: true}
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

// InternalServerError 500: код задан именем функции, а не сервисом явно — клиенту текст статуса, ошибка — в лог и Sentry.
func InternalServerError(c *gin.Context, err error, ctx map[string]any) {
	errorResponseWithStatus(c, http.StatusInternalServerError, err, ctx, false)
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
// и Sentry, как раньше. Исключение: код задан сервисом явно (ErrorResponseWithStatus, ErrorResponseUntrackableSentry,
// их *Raw, NewHttpException, NewUntrackableHttpException) и ошибка — простое сообщение без вложенной причины
// (не %w, не обёртка, не errors.Join): намеренный текст («Сервис временно недоступен») уходит как есть, details — null.
// С HTTP_ERROR_DETAILS (gin.ErrorDetailsEnabled) — текст ошибки и details, как раньше. 4xx — как раньше.
func publicErrorText(c *gin.Context, httpEx *exception.HttpException) (string, map[string]any) {
	if httpEx.Code < http.StatusInternalServerError || gin2.ErrorDetailsEnabled(c) {
		return httpEx.Error(), httpEx.Context
	}

	if publictext.Explicit(httpEx) && isPlainMessage(httpEx.Err) {
		return httpEx.Err.Error(), nil
	}

	text := http.StatusText(httpEx.Code)
	if text == "" {
		text = http.StatusText(http.StatusInternalServerError)
	}

	return text, nil
}

// isPlainMessage Ошибка — простое сообщение без вложенной причины: errors.Unwrap — nil и нет Unwrap() []error.
func isPlainMessage(err error) bool {
	return err != nil && reflect.TypeOf(err) == plainMessageType
}

// plainMessageType Тип простого сообщения (`errors.New`, `fmt.Errorf` без `%w`): ошибка драйвера или своего типа
// (`*pgconn.PgError`) — не публичный текст, даже без вложенной причины
var plainMessageType = reflect.TypeOf(errors.New(""))

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
