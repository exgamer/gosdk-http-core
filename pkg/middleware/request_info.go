package middleware

import (
	"context"
	"github.com/exgamer/gosdk-core/pkg/app"
	constants2 "github.com/exgamer/gosdk-core/pkg/constants"
	"github.com/exgamer/gosdk-core/pkg/di"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

// RequestInfoMiddleware Middleware заполняющий данные запроса
func RequestInfoMiddleware(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		baseConfig, err := di.GetBaseConfig(a.Container)

		if err != nil {
			return
		}

		appInfo := gin2.GetInstanceAppInfo(baseConfig)
		httpInfo := gin2.GetInstanceHttpInfo(c)

		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, constants2.AppInfoKey, appInfo)
		ctx = context.WithValue(ctx, constants.HttpInfoKey, httpInfo)

		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
