package middleware

import (
	"github.com/exgamer/gosdk-core/pkg/app"
	app2 "github.com/exgamer/gosdk-http-core/pkg/app"
	exception2 "github.com/exgamer/gosdk-http-core/pkg/exception"
	"github.com/gin-gonic/gin"
	"time"
)

// MetricsMiddleware - мидлвар для обработки HTTP запросов метрик
func MetricsMiddleware(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		metricsCollector, err := app2.GetMetricsCollector(a)
		if err != nil {
			return
		}

		if metricsCollector == nil {
			return
		}

		duration := time.Since(start).Seconds()

		ex, _ := c.Get("exception")
		statusCode := c.Writer.Status()
		if he, ok := ex.(*exception2.HttpException); ok {
			statusCode = he.Code
		}

		path := c.FullPath()
		if path == "" {
			path = "__unknown__"
		}

		metricsCollector.GetMetrics(statusCode, c.Request.Method, path, duration)
	}
}
