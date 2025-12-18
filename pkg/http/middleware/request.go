package middleware

import (
	"context"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
	"github.com/exgamer/gosdk-http-core/pkg/debug"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

// RequestMiddleware Middleware заполняющий данные запроса
func RequestMiddleware(baseConfig *config.BaseConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		gin2.SetAppInfo(c, baseConfig)
		appInfo := gin2.GetAppInfo(c)
		ctx := context.WithValue(c.Request.Context(), constants.AppInfoKey, appInfo)
		c.Request = c.Request.WithContext(ctx)

		if c.GetHeader("Apelsin") == "sanya" {
			appInfo := gin2.GetAppInfo(c)

			if appInfo.AppEnv != "prod" && appInfo.AppEnv != "PROD" {
				collector := debug.NewDebugCollector()
				collector.Meta["url"] = appInfo.RequestUrl
				collector.Meta["method"] = appInfo.RequestMethod
				collector.Meta["id"] = appInfo.RequestId
				ctx := context.WithValue(c.Request.Context(), debug.DebugKey, collector)
				c.Request = c.Request.WithContext(ctx)
			}
		}
		c.Next()
	}
}
