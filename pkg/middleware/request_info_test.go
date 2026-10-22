package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/constants"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
)

// HttpInfo from RequestInfoMiddleware is read by GetHttpInfoFromContext.
func TestRequestInfoMiddleware_HttpInfoInRequestContext(t *testing.T) {
	ctx := requestInfoContext(t, "rid-80")

	info := gin2.GetHttpInfoFromContext(ctx)
	if info == nil {
		t.Fatal("GetHttpInfoFromContext = nil, want HttpInfo put by RequestInfoMiddleware")
	}

	if info.RequestId != "rid-80" || info.RequestUrl != "/x" || info.RequestMethod != http.MethodGet {
		t.Errorf("HttpInfo = {RequestId: %q, RequestUrl: %q, RequestMethod: %q}, want {rid-80, /x, GET}",
			info.RequestId, info.RequestUrl, info.RequestMethod)
	}
}

// HttpInfo is no longer under the string key; read it only through GetHttpInfoFromContext.
func TestRequestInfoMiddleware_NoStringKey(t *testing.T) {
	ctx := requestInfoContext(t, "rid-81")

	if got := ctx.Value("http_info"); got != nil {
		t.Errorf(`ctx.Value("http_info") = %+v, want nil`, got)
	}

	if got := ctx.Value(constants.HttpInfoKey); got != nil { //nolint:staticcheck // SA1019: проверяем, что по устаревшему ключу значения нет
		t.Errorf("ctx.Value(constants.HttpInfoKey) = %+v, want nil", got)
	}
}

// requestInfoContext serves GET /x with the request id through RequestInfoMiddleware
// and returns the request context the handler saw.
func requestInfoContext(t *testing.T, requestID string) context.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var ctx context.Context

	engine := gin.New()
	engine.Use(RequestInfoMiddleware(nil))
	engine.GET("/x", func(c *gin.Context) {
		ctx = c.Request.Context()
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(constants.RequestIdHeaderName, requestID)
	engine.ServeHTTP(httptest.NewRecorder(), req)

	if ctx == nil {
		t.Fatal("GET /x: the handler did not run")
	}

	return ctx
}
