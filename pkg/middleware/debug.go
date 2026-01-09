package middleware

import (
	"github.com/exgamer/gosdk-core/pkg/debug"
	"github.com/exgamer/gosdk-core/pkg/helpers"
	"github.com/exgamer/gosdk-core/pkg/logger"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

// DebugMiddleware кладёт DebugCollector в context запроса и считает TotalTime в конце
func DebugMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		appInfo := helpers.GetAppInfoFromContext(c.Request.Context())

		if logger.ParseLevel(appInfo.LogLevel) < logger.LevelDebug {
			return
		}

		collector := debug.NewDebugCollector()
		if httpInfo := gin2.GetHttpInfoFromContext(c.Request.Context()); httpInfo != nil {
			collector.Meta["url"] = httpInfo.RequestUrl
			collector.Meta["method"] = httpInfo.RequestMethod
			collector.Meta["id"] = httpInfo.RequestId
		}

		// положили в request context
		c.Request = c.Request.WithContext(debug.WithDebugCollector(c.Request.Context(), collector))

		c.Next()

		collector.CalculateTotalTime()
	}
}
