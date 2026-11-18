package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	sdkctx "github.com/exgamer/gosdk-core/pkg/context"
	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/exgamer/gosdk-http-core/pkg/response"
)

// seen is what the handler saw and what the client got.
type seen struct {
	ctx      context.Context
	corr     string
	info     *config.HttpInfo
	recorder *httptest.ResponseRecorder
}

// serveCorrelation serves GET /x through RequestInfoMiddleware (optionally under HANDLER_TIMEOUT) with the incoming
// headers and the extra middleware mws; the handler answers with handle.
func serveCorrelation(t *testing.T, timeout bool, headers map[string]string, handle func(c *gin.Context),
	mws ...gin.HandlerFunc,
) seen {
	t.Helper()
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	if timeout {
		engine = gin2.InitRouter(&baseConfig.BaseConfig{Name: "svc"}, &config.HttpConfig{HandlerTimeout: 5})
	}

	engine.Use(RequestInfoMiddleware(nil))
	engine.Use(mws...)

	var s seen

	engine.GET("/x", func(c *gin.Context) {
		s.ctx = c.Request.Context()
		s.corr = sdkctx.CorrelationID(s.ctx)
		s.info = gin2.GetHttpInfoFromContext(s.ctx)
		handle(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	s.recorder = httptest.NewRecorder()
	engine.ServeHTTP(s.recorder, req)

	if s.ctx == nil {
		t.Fatal("GET /x: the handler did not run")
	}

	return s
}

func ok(c *gin.Context) { c.Status(http.StatusOK) }

// Incoming order: X-Correlation-Id → X-Request-Id → Request-Id → new id; an invalid value is skipped.
func TestRequestInfoMiddleware_IncomingCorrelationOrder(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{name: "all three", want: "corr",
			headers: map[string]string{"X-Correlation-Id": "corr", "X-Request-Id": "xreq", "Request-Id": "req"}},
		{name: "x-request-id and request-id", want: "xreq",
			headers: map[string]string{"X-Request-Id": "xreq", "Request-Id": "req"}},
		{name: "request-id only", want: "req", headers: map[string]string{"Request-Id": "req"}},
		{name: "invalid x-correlation-id skipped", want: "xreq",
			headers: map[string]string{"X-Correlation-Id": "bad id", "X-Request-Id": "xreq"}},
		{name: "lowercase names", want: "corr",
			headers: map[string]string{"x-correlation-id": "corr", "request-id": "req"}},
	}

	for _, timeout := range []bool{false, true} {
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				s := serveCorrelation(t, timeout, c.headers, ok)

				if s.corr != c.want {
					t.Fatalf("timeout=%v: CorrelationID = %q, want %q", timeout, s.corr, c.want)
				}

				if s.info == nil || s.info.CorrelationId != c.want || s.info.RequestId != c.want { //nolint:staticcheck // SA1019: прежнее поле — тот же id
					t.Fatalf("timeout=%v: HttpInfo = %+v, want CorrelationId and RequestId %q", timeout, s.info, c.want)
				}

				if got := s.recorder.Header().Values(constants.CorrelationIdHeaderName); len(got) != 1 || got[0] != c.want {
					t.Fatalf("timeout=%v: response X-Correlation-Id = %q, want [%s]", timeout, got, c.want)
				}

				if got := s.recorder.Header().Get(constants.RequestIdHeaderName); got != "" { //nolint:staticcheck // SA1019: прежний заголовок наружу не уходит
					t.Fatalf("timeout=%v: response Request-Id = %q, want none", timeout, got)
				}
			})
		}
	}
}

// Without an incoming id one new id is the correlation id, HttpInfo ids and the response header.
func TestRequestInfoMiddleware_CorrelationIDGenerated(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		s := serveCorrelation(t, timeout, map[string]string{"Request-Id": "bad id\nwith newline"}, ok)

		if !gin2.IsValidRequestId(s.corr) {
			t.Fatalf("timeout=%v: CorrelationID = %q, want a generated id", timeout, s.corr)
		}

		if s.info == nil || s.info.CorrelationId != s.corr || s.info.RequestId != s.corr { //nolint:staticcheck // SA1019: прежнее поле — тот же id
			t.Fatalf("timeout=%v: HttpInfo = %+v, want ids %q", timeout, s.info, s.corr)
		}

		if got := s.recorder.Header().Get(constants.CorrelationIdHeaderName); got != s.corr {
			t.Fatalf("timeout=%v: response X-Correlation-Id = %q, want %q", timeout, got, s.corr)
		}
	}
}

// The error body carries correlation_id (not request_id) equal to the correlation id.
func TestErrorResponse_CorrelationIdField(t *testing.T) {
	s := serveCorrelation(t, false, map[string]string{"X-Correlation-Id": "corr-err"}, func(c *gin.Context) {
		response.BadRequest(c, errors.New("bad"), nil)
	}, FormattedResponseMiddleware())

	var body struct {
		Data map[string]any `json:"data"`
	}

	if err := json.Unmarshal(s.recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", s.recorder.Body.String(), err)
	}

	if body.Data["correlation_id"] != "corr-err" {
		t.Fatalf("data.correlation_id = %v, want corr-err (body %s)", body.Data["correlation_id"], s.recorder.Body.String())
	}

	if _, has := body.Data["request_id"]; has {
		t.Fatalf("data has request_id, want only correlation_id (body %s)", s.recorder.Body.String())
	}
}
