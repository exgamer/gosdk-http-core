package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/exgamer/gosdk-core/pkg/helpers"
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
		queryParams := req.URL.Query()

		var requestBody []byte
		if req.Body != nil {
			requestBody, _ = io.ReadAll(io.LimitReader(req.Body, 1<<20))
			req.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		c.Next()

		latency := time.Since(start)
		appInfo := helpers.GetAppInfoFromContext(c.Request.Context())
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

			logger.FormattedError(appInfo.ServiceName, httpInfo.RequestMethod, httpInfo.RequestUrl, status, httpInfo.RequestId, httpEx.Error())

			return
		}

		// 2) если gin накопил ошибки (например c.Error(err)), тоже считаем это error-log
		if len(c.Errors) > 0 || status >= 500 {
			msg := c.Errors.String()
			if msg == "" {
				msg = "server error"
			}

			logger.FormattedError(appInfo.ServiceName, httpInfo.RequestMethod, httpInfo.RequestUrl, status, httpInfo.RequestId, msg)

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

		logger.FormattedInfo(appInfo.ServiceName, httpInfo.RequestMethod, httpInfo.RequestUrl, status, httpInfo.RequestId, messageBuilder.String())
	}
}

// TODO вынести в настройку, чтобы можно было внедрять свое
func sanitizeHeaders(h http.Header) http.Header {
	c := h.Clone()

	if c.Get("Authorization") != "" {
		c.Set("Authorization", "***")
	}
	if c.Get("Cookie") != "" {
		c.Set("Cookie", "***")
	}

	return c
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
