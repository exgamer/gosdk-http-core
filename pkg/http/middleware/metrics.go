package middleware

import (
	"github.com/exgamer/gosdk-http-core/pkg/metricapp"
	"github.com/gin-gonic/gin"
	"time"
)

// MetricsCollect - мидлвар для обработки HTTP запросов метрик
func MetricsCollect() gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now()
		c.Next()

		metricapp.MetricsMiddlewaree(c, now)
	}
}
