package middleware

import (
	exception2 "github.com/exgamer/gosdk-core/pkg/exception"
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
		httpInfo := gin2.GetHttpInfo(c)

		for _, err := range c.Errors {
			sentry.CaptureException(err)
			logger.FormattedErrorWithAppInfo(appInfo, httpInfo, err.Error())
		}

		appExceptionObject, exists := c.Get("exception")

		if exists {
			appException := exception2.AppException{}
			mapstructure.Decode(appExceptionObject, &appException)
			sentry.WithScope(func(scope *sentry.Scope) {
				scope.SetLevel(sentry.LevelError)
				sentry.CaptureException(appException.Error)
			})
			logger.FormattedErrorWithAppInfo(appInfo, httpInfo, appException.Error.Error())

			return
		}

		if exists {
			appException := exception.HttpException{}
			mapstructure.Decode(appExceptionObject, &appException)
			sentry.WithScope(func(scope *sentry.Scope) {
				if appException.Code >= http.StatusBadRequest && appException.Code < http.StatusInternalServerError {
					scope.SetLevel(sentry.LevelWarning)
				} else {
					scope.SetLevel(sentry.LevelError)
				}
				sentry.CaptureException(appException.Error)
			})
			logger.FormattedErrorWithAppInfo(appInfo, httpInfo, appException.Error.Error())

			return
		}

		logger.FormattedInfo(appInfo.ServiceName, httpInfo.RequestMethod, httpInfo.RequestUrl, c.Writer.Status(), httpInfo.RequestId, "Exec time:"+latency.String())
	}
}
