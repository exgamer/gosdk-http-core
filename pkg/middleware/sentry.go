package middleware

import (
	"fmt"

	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

// SentryMiddleware Middleware для обработки ошибок в sentry
func SentryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		exObj, exists := c.Get("exception")
		if !exists {
			return
		}

		err, ok := exObj.(error)
		if !ok {
			err = fmt.Errorf("exception in context is not error: %T", exObj)
		}

		gin2.CaptureToSentry(c, err)
	}
}
