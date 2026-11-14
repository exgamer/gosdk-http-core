package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	logger2 "github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/exgamer/gosdk-http-core/pkg/logger"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// LoggerMiddleware Middleware для логирования ответа и отправки ошибок в сентри
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		req := c.Request
		headers := sanitizeHeaders(req.Header)
		queryParams := gin2.MaskQuery(req.URL.Query(), "***")

		// Тело читается только для debug-лога: до 1 МиБ в лог, обработчик получает тело целиком
		// (прочитанная часть + остаток исходного тела).
		var requestBody []byte
		if req.Body != nil && req.Body != http.NoBody && logger2.IsDebugLevel() {
			requestBody, _ = io.ReadAll(io.LimitReader(req.Body, logBodyLimit))
			req.Body = readCloser{Reader: io.MultiReader(bytes.NewReader(requestBody), req.Body), Closer: req.Body}
		}

		c.Next()

		latency := time.Since(start)
		httpInfo := gin2.GetHttpInfoFromContext(c.Request.Context())
		status := c.Writer.Status()

		// 1) если выставили exception
		if exObj, exists := c.Get("exception"); exists {
			var err error
			if e, ok := exObj.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("exception in context is not error: %T", exObj)
			}

			var httpEx *exception.HttpException
			if !errors.As(err, &httpEx) {
				httpEx = exception.NewInternalServerErrorException(err, nil)
			}

			// 4xx — ошибка клиента, warning; 5xx и прочее — error
			if isClientError(httpEx.Code) && status < http.StatusInternalServerError {
				logger.FormattedWarning(c.Request.Context(), httpInfo.RequestMethod, httpInfo.RequestUrl, status, httpInfo.RequestId, httpEx.Error())

				return
			}

			logger.FormattedError(c.Request.Context(), httpInfo.RequestMethod, httpInfo.RequestUrl, status, httpInfo.RequestId, httpEx.Error())

			return
		}

		// 2) если gin накопил ошибки (например c.Error(err)), тоже считаем это error-log
		if len(c.Errors) > 0 || status >= 500 {
			msg := c.Errors.String()
			if msg == "" {
				msg = "server error"
			}

			logger.FormattedError(c.Request.Context(), httpInfo.RequestMethod, httpInfo.RequestUrl, status, httpInfo.RequestId, msg)

			return
		}

		// info log
		messageBuilder := strings.Builder{}
		if logger2.IsDebugLevel() {
			messageBuilder.WriteString("headers: " + headersToJSON(headers) + "; ")
			messageBuilder.WriteString("query: " + queryToJSON(queryParams) + "; ")
			messageBuilder.WriteString("request_body: " + bodyToPrettyJSON(requestBody) + "; ")
		}
		messageBuilder.WriteString("Exec time:" + latency.String())

		if isClientError(status) {
			logger.FormattedWarning(c.Request.Context(), httpInfo.RequestMethod, httpInfo.RequestUrl, status, httpInfo.RequestId, messageBuilder.String())

			return
		}

		logger.FormattedInfo(c.Request.Context(), httpInfo.RequestMethod, httpInfo.RequestUrl, status, httpInfo.RequestId, messageBuilder.String())
	}
}

func isClientError(status int) bool {
	return status >= http.StatusBadRequest && status < http.StatusInternalServerError
}

// logBodyLimit Сколько байт тела запроса попадает в debug-лог.
const logBodyLimit = 1 << 20

// readCloser Тело запроса после чтения части для лога: читается прочитанная часть и остаток, Close — исходного тела.
type readCloser struct {
	io.Reader
	io.Closer
}

// sanitizeHeaders Чувствительные заголовки (gin.IsSensitiveKey, SENSITIVE_KEYS) — «***».
func sanitizeHeaders(h http.Header) http.Header {
	return gin2.MaskHeaders(h, "***")
}

func headersToJSON(h http.Header) string {
	// http.Header = map[string][]string → отлично маршалится
	b, err := json.Marshal(h)
	if err != nil {
		return "{}"
	}

	return string(b)
}

func queryToJSON(q url.Values) string {
	// url.Values = map[string][]string
	b, err := json.Marshal(q)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func bodyToPrettyJSON(body []byte) string {
	if len(body) == 0 {
		return "{}"
	}

	var out bytes.Buffer
	if err := json.Indent(&out, body, "", "  "); err != nil {
		// если тело не JSON — просто строкой
		return string(body)
	}

	return out.String()
}
