package middleware

import (
	"bytes"
	"encoding/json"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/exgamer/gosdk-http-core/pkg/logger"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// LoggerMiddleware Middleware для логирования ответа и отправки ошибок в сентри
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		req := c.Request
		headers := sanitizeHeaders(req.Header)
		queryParams := req.URL.Query()
		var requestBody []byte
		if req.Body != nil {
			requestBody, _ = io.ReadAll(req.Body)
			req.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		c.Next()
		endTime := time.Now()
		latency := endTime.Sub(startTime)
		appInfo := gin2.GetAppInfo(c)
		httpInfo := gin2.GetHttpInfo(c)

		for _, err := range c.Errors {
			sentry.CaptureException(err)
			logger.FormattedErrorWithAppInfo(appInfo, httpInfo, err.Error())
		}

		appExceptionObject, exists := c.Get("exception")

		if exists {
			appException := exception.HttpException{}
			mapstructure.Decode(appExceptionObject, &appException)
			sentry.WithScope(func(scope *sentry.Scope) {
				if appException.Code >= http.StatusBadRequest && appException.Code < http.StatusInternalServerError {
					scope.SetLevel(sentry.LevelWarning)
				} else {
					scope.SetLevel(sentry.LevelError)
				}

				scope.SetExtras(map[string]interface{}{
					"headers": headers,
					"query":   queryParams,
					"request": string(requestBody),
					"status":  c.Writer.Status(),
				})

				sentry.CaptureException(appException.Error)
			})
			logger.FormattedErrorWithAppInfo(appInfo, httpInfo, appException.Error.Error())

			return
		}

		messageBuilder := strings.Builder{}

		if appInfo.DebugMode {
			messageBuilder.WriteString("headers: " + headersToJSON(headers) + "; ")
			messageBuilder.WriteString("query: " + queryToJSON(queryParams) + "; ")
			messageBuilder.WriteString("request_body: " + bodyToPrettyJSON(requestBody) + "; ")
		}

		messageBuilder.WriteString("Exec time:" + latency.String())

		log.Println(messageBuilder.String())

		logger.FormattedInfo(appInfo.ServiceName, httpInfo.RequestMethod, httpInfo.RequestUrl, c.Writer.Status(), httpInfo.RequestId, messageBuilder.String())
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
		return ""
	}

	var out bytes.Buffer
	if err := json.Indent(&out, body, "", "  "); err != nil {
		// если тело не JSON — просто строкой
		return string(body)
	}

	return out.String()
}
