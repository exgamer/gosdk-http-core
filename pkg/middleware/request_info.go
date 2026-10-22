package middleware

import (
	"github.com/exgamer/gosdk-core/pkg/app"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

// RequestInfoMiddleware Middleware заполняющий данные запроса
func RequestInfoMiddleware(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		httpInfo := gin2.GetInstanceHttpInfo(c)

		c.Request = c.Request.WithContext(gin2.WithHttpInfo(c.Request.Context(), httpInfo))

		c.Next()
	}
}
