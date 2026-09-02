package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Функции этого файла — аналоги http_response_helper.go, но без обёртки
// {success, data} (и без debug-поля) в финальном JSON. Используются для
// эндпойнтов с внешним/фиксированным контрактом ответа (вебхуки, колбэки,
// прокси на другие API и т.п.), где обёртка не нужна.

// ---------- raw success ----------

// Raw кладёт data как есть (без обёртки success/data) с произвольным статусом.
func Raw(c *gin.Context, data any, statusCode int) {
	c.Set(ctxKeyData, data)
	c.Set(ctxKeyRaw, true)
	c.Set(ctxKeyStatusCode, statusCode)
}

func RawSuccess(c *gin.Context, data any) {
	Raw(c, data, http.StatusOK)
}

func RawCreated(c *gin.Context, data any) {
	Raw(c, data, http.StatusCreated)
}

func RawDeleted(c *gin.Context, data any) {
	Raw(c, data, http.StatusNoContent)
}

func FormattedRawResponse(c *gin.Context, data any) {
	RawSuccess(c, data)
	Formatted(c)
}

// ---------- raw error ----------

func ErrorResponseRaw(c *gin.Context, err error) {
	c.Set(ctxKeyRaw, true)
	ErrorResponse(c, err)
}

func ErrorResponseWithStatusRaw(c *gin.Context, statusCode int, err error, context map[string]any) {
	c.Set(ctxKeyRaw, true)
	ErrorResponseWithStatus(c, statusCode, err, context)
}

func ErrorResponseUntrackableSentryRaw(c *gin.Context, statusCode int, err error, context map[string]any) {
	c.Set(ctxKeyRaw, true)
	ErrorResponseUntrackableSentry(c, statusCode, err, context)
}

func BadRequestRaw(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatusRaw(c, http.StatusBadRequest, err, ctx)
}

func UnauthorizedRaw(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatusRaw(c, http.StatusUnauthorized, err, ctx)
}

func ForbiddenRaw(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatusRaw(c, http.StatusForbidden, err, ctx)
}

func NotFoundRaw(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatusRaw(c, http.StatusNotFound, err, ctx)
}

func ConflictRaw(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatusRaw(c, http.StatusConflict, err, ctx)
}

func UnprocessableEntityRaw(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatusRaw(c, http.StatusUnprocessableEntity, err, ctx)
}

func TooManyRequestsRaw(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatusRaw(c, http.StatusTooManyRequests, err, ctx)
}

func InternalServerErrorRaw(c *gin.Context, err error, ctx map[string]any) {
	ErrorResponseWithStatusRaw(c, http.StatusInternalServerError, err, ctx)
}
