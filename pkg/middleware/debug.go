package middleware

import (
	"github.com/exgamer/gosdk-core/pkg/debug"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

// DebugMiddleware кладёт DebugCollector в context запроса и считает TotalTime в конце
func DebugMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		collector := debug.NewDebugCollector()
		// положили в request context
		c.Request = c.Request.WithContext(debug.WithDebugCollector(c.Request.Context(), collector))

		c.Next()

		httpConfig := gin2.GetHttpInfoFromContext(c.Request.Context())
		collector.Meta["url"] = httpConfig.RequestUrl
		collector.Meta["method"] = httpConfig.RequestMethod
		collector.Meta["id"] = httpConfig.RequestId

		collector.CalculateTotalTime()
	}
}
