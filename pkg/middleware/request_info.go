package middleware

import (
	"context"
	config2 "github.com/exgamer/gosdk-core/pkg/config"
	constants2 "github.com/exgamer/gosdk-core/pkg/constants"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

// RequestInfoMiddleware Middleware заполняющий данные запроса
func RequestInfoMiddleware(baseConfig *config2.BaseConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		gin2.SetAppInfo(c, baseConfig)
		appInfo := gin2.GetAppInfo(c)
		ctx := context.WithValue(c.Request.Context(), constants2.AppInfoKey, appInfo)
		gin2.SetHttpInfo(c)
		httpInfo := gin2.GetHttpInfo(c)
		ctx = context.WithValue(c.Request.Context(), constants.HttpInfoKey, httpInfo)
		c.Request = c.Request.WithContext(ctx)

		//if c.GetHeader("Apelsin") == "sanya" {
		//	appInfo := gin2.GetAppInfo(c)
		//
		//	if appInfo.AppEnv != "prod" && appInfo.AppEnv != "PROD" {
		//		collector := debug.NewDebugCollector()
		//		collector.Meta["url"] = appInfo.RequestUrl
		//		collector.Meta["method"] = appInfo.RequestMethod
		//		collector.Meta["id"] = appInfo.RequestId
		//		ctx := context.WithValue(c.Request.Context(), debug.DebugKey, collector)
		//		c.Request = c.Request.WithContext(ctx)
		//	}
		//}
		c.Next()
	}
}
