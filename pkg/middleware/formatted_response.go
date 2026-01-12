package middleware

import (
	"github.com/exgamer/gosdk-http-core/pkg/response"
	"github.com/gin-gonic/gin"
)

// FormattedResponseMiddleware Middleware для обработки ответа
func FormattedResponseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		response.Formatted(c)
	}
}
