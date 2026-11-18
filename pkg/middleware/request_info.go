package middleware

import (
	"github.com/exgamer/gosdk-core/pkg/app"
	sdkctx "github.com/exgamer/gosdk-core/pkg/context"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

// RequestInfoMiddleware Middleware заполняющий данные запроса. Id корреляции запроса (X-Correlation-Id → X-Request-Id →
// Request-Id → новый UUID) кладётся в контекст (sdkctx.WithCorrelationID, HttpInfo.CorrelationId) и в заголовок
// ответа X-Correlation-Id.
func RequestInfoMiddleware(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		httpInfo := gin2.GetInstanceHttpInfo(c)

		ctx := gin2.WithHttpInfo(c.Request.Context(), httpInfo)
		ctx = sdkctx.WithCorrelationID(ctx, httpInfo.CorrelationId)
		c.Request = c.Request.WithContext(ctx)
		c.Header(constants.CorrelationIdHeaderName, httpInfo.CorrelationId)

		c.Next()
	}
}
