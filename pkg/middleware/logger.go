package middleware

import (
	"bytes"
	"encoding/json"
	"github.com/exgamer/gosdk-core/pkg/helpers"
	logger2 "github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/exgamer/gosdk-http-core/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"io"
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
		appInfo := helpers.GetAppInfoFromContext(c.Request.Context())
		httpInfo := gin2.GetHttpInfoFromContext(c.Request.Context())

		appExceptionObject, exists := c.Get("exception")

		if exists {
			appException := exception.HttpException{}
			mapstructure.Decode(appExceptionObject, &appException)
			logger.FormattedError(appInfo.ServiceName, httpInfo.RequestMethod, httpInfo.RequestUrl, appException.Code, httpInfo.RequestId, appException.Error.Error())

			return
		}

		messageBuilder := strings.Builder{}

		if logger2.IsDebugLevel() {
			messageBuilder.WriteString("headers: " + headersToJSON(headers) + "; ")
			messageBuilder.WriteString("query: " + queryToJSON(queryParams) + "; ")
			messageBuilder.WriteString("request_body: " + bodyToPrettyJSON(requestBody) + "; ")
		}

		messageBuilder.WriteString("Exec time:" + latency.String())

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
		return "{}"
	}

	var out bytes.Buffer
	if err := json.Indent(&out, body, "", "  "); err != nil {
		// если тело не JSON — просто строкой
		return string(body)
	}

	return out.String()
}
