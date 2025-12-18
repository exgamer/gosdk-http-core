package middleware

import (
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/exgamer/gosdk-http-core/pkg/logger"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"net/http"
	"time"
)

// LoggerMiddleware Middleware для логирования ответа и отправки ошибок в сентри
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		c.Next()
		endTime := time.Now()
		latency := endTime.Sub(startTime)
		appInfo := gin2.GetAppInfo(c)

		for _, err := range c.Errors {
			sentry.CaptureException(err)
			logger.FormattedErrorWithAppInfo(appInfo, err.Error())
		}

		appExceptionObject, exists := c.Get("exception")

		if exists {
			appException := exception.AppException{}
			mapstructure.Decode(appExceptionObject, &appException)
			sentry.WithScope(func(scope *sentry.Scope) {
				if appException.Code >= http.StatusBadRequest && appException.Code < http.StatusInternalServerError {
					scope.SetLevel(sentry.LevelWarning)
				} else {
					scope.SetLevel(sentry.LevelError)
				}
				sentry.CaptureException(appException.Error)
			})
			logger.FormattedErrorWithAppInfo(appInfo, appException.Error.Error())
		}

		logger.FormattedInfo(appInfo.ServiceName, appInfo.RequestMethod, appInfo.RequestUrl, c.Writer.Status(), appInfo.RequestId, "Exec time:"+latency.String())
	}
}
