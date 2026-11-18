package gin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	sdkctx "github.com/exgamer/gosdk-core/pkg/context"
	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
)

func requestIdOf(t *testing.T, header string, set bool) (string, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	if set {
		c.Request.Header.Set(constants.RequestIdHeaderName, header) //nolint:staticcheck // SA1019: прежний заголовок принимается на входе
	}

	info := GetInstanceHttpInfo(c)

	return info.CorrelationId, c.Request.Header.Get(constants.CorrelationIdHeaderName)
}

// A well-formed incoming Request-Id (UUID, ULID, hex trace id, base64) is kept as is.
func TestRequestId_ValidKept(t *testing.T) {
	for _, id := range []string{
		"2f1c7a0e-8f6b-4c1d-9a3e-5b2d4c6e8f10",
		"01HZY3N8Q4K5V6W7X8Y9Z0ABCD",
		"4bf92f3577b34da6a3ce929d0e0e4736",
		"YWJjZA==",
		"svc.a:42/x+y-z_1",
		strings.Repeat("a", 128),
	} {
		got, hdr := requestIdOf(t, id, true)
		if got != id || hdr != id {
			t.Errorf("Request-Id %q → %q (header %q), want kept", id, got, hdr)
		}
	}
}

// Missing, empty or malformed (spaces, quotes, control characters, over 128) — a generated id replaces it,
// also in the request header passed on to outgoing calls.
func TestRequestId_InvalidReplaced(t *testing.T) {
	cases := map[string]string{
		"empty":   "",
		"space":   "abc def",
		"quote":   `abc"}`,
		"newline": "abc\nforged",
		"tab":     "abc\tx",
		"unicode": "идентификатор",
		"long":    strings.Repeat("a", 129),
	}
	for name, id := range cases {
		got, hdr := requestIdOf(t, id, true)
		if got == id || !IsValidRequestId(got) || hdr != got {
			t.Errorf("%s: Request-Id %q → %q (header %q), want a generated valid id in both", name, id, got, hdr)
		}
	}

	got, hdr := requestIdOf(t, "", false)
	if !IsValidRequestId(got) || hdr != got {
		t.Errorf("no header → %q (header %q), want a generated id", got, hdr)
	}
}

// HANDLER_TIMEOUT alone (no RequestInfoMiddleware) already puts the incoming id into the handler context
// as the correlation id and into the response header X-Correlation-Id.
func TestHandlerTimeout_CorrelationIDInHandlerContext(t *testing.T) {
	for _, incoming := range []string{"rid-timeout-1", ""} {
		t.Run("incoming="+incoming, func(t *testing.T) {
			r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 5})

			var corr, header string

			r.GET("/x", func(c *gin.Context) {
				corr = sdkctx.CorrelationID(c.Request.Context())
				header = c.GetHeader(constants.CorrelationIdHeaderName)
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if incoming != "" {
				req.Header.Set(constants.XRequestIdHeaderName, incoming)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if corr == "" || corr != header {
				t.Fatalf("CorrelationID = %q, X-Correlation-Id request header = %q, want the same non-empty id", corr, header)
			}

			if incoming != "" && corr != incoming {
				t.Fatalf("CorrelationID = %q, want incoming %q", corr, incoming)
			}

			if got := w.Header().Get(constants.CorrelationIdHeaderName); got != corr {
				t.Fatalf("response X-Correlation-Id = %q, want %q", got, corr)
			}
		})
	}
}

// IncomingCorrelationID takes the first valid id: X-Correlation-Id, X-Request-Id, Request-Id.
func TestIncomingCorrelationID_Order(t *testing.T) {
	cases := []struct {
		name string
		h    map[string]string
		want string
	}{
		{name: "none", h: nil, want: ""},
		{name: "all", h: map[string]string{"X-Correlation-Id": "a", "X-Request-Id": "b", "Request-Id": "c"}, want: "a"},
		{name: "x-request-id first", h: map[string]string{"X-Request-Id": "b", "Request-Id": "c"}, want: "b"},
		{name: "legacy", h: map[string]string{"Request-Id": "c"}, want: "c"},
		{name: "invalid skipped", h: map[string]string{"X-Correlation-Id": "a b", "X-Request-Id": "", "Request-Id": "c"}, want: "c"},
		{name: "all invalid", h: map[string]string{"X-Correlation-Id": "a\nb"}, want: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := http.Header{}
			for k, v := range c.h {
				h.Set(k, v)
			}

			if got := IncomingCorrelationID(h); got != c.want {
				t.Fatalf("IncomingCorrelationID = %q, want %q", got, c.want)
			}
		})
	}
}
