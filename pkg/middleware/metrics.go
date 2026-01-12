package middleware

import (
	"errors"
	"time"

	"github.com/exgamer/gosdk-core/pkg/app"
	app2 "github.com/exgamer/gosdk-http-core/pkg/app"
	exception2 "github.com/exgamer/gosdk-http-core/pkg/exception"
	"github.com/gin-gonic/gin"
)

// MetricsMiddleware - мидлвар для обработки HTTP запросов метрик
func MetricsMiddleware(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		metricsCollector, err := app2.GetMetricsCollector(a)
		if err != nil || metricsCollector == nil {
			return
		}

		duration := time.Since(start).Seconds()
		statusCode := c.Writer.Status()

		// Если по какой-то причине статус ещё не выставлен, пробуем взять из exception
		if statusCode == 0 {
			if exObj, ok := c.Get("exception"); ok {
				if err, ok := exObj.(error); ok {
					var he *exception2.HttpException
					if errors.As(err, &he) && he != nil {
						statusCode = he.Code
					}
				}
			}
		}

		path := c.FullPath()
		if path == "" {
			path = "__unknown__"
		}

		metricsCollector.GetMetrics(statusCode, c.Request.Method, path, duration)
	}
}
