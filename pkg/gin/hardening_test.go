package gin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/config"
)

// A route excluded with WithoutHandlerTimeout runs past HANDLER_TIMEOUT and the client gets its answer, not 503;
// another route keeps the timeout.
func TestWithoutHandlerTimeout_RouteRunsPastTimeout(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 1})
	WithoutHandlerTimeout(http.MethodGet, "/stream/:id")
	slow := func(c *gin.Context) {
		time.Sleep(1300 * time.Millisecond)
		c.String(http.StatusOK, "done")
	}
	r.GET("/stream/:id", slow)
	r.GET("/other/:id", slow)

	if w := serve(r, http.MethodGet, "/stream/1"); w.Code != http.StatusOK || w.Body.String() != "done" {
		t.Fatalf("excluded route = %d %q, want 200 done", w.Code, w.Body.String())
	}

	if w := serve(r, http.MethodGet, "/other/1"); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("other route = %d, want 503 timeout", w.Code)
	}
}

// WriteDeadline under HANDLER_TIMEOUT cannot set the deadline — a warning names WithoutHandlerTimeout;
// on an excluded route it is set silently.
func TestWriteDeadline_UnderTimeoutWarns(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 30})
	WithoutHandlerTimeout(http.MethodGet, "/files/free")
	ok := func(c *gin.Context) { c.String(http.StatusOK, "x") }
	r.GET("/files/buffered", WriteDeadline(time.Minute), ok)
	r.GET("/files/free", WriteDeadline(time.Minute), ok)

	srv := httptest.NewServer(r)
	defer srv.Close()

	out := captureLog(t)
	get(t, srv.URL+"/files/free")
	if strings.Contains(out.String(), "write deadline") {
		t.Fatalf("excluded route: unexpected warning %q", out.String())
	}

	get(t, srv.URL+"/files/buffered")
	if !strings.Contains(out.String(), "WithoutHandlerTimeout") {
		t.Fatalf("buffered route: want warning naming WithoutHandlerTimeout, got %q", out.String())
	}
}

func TestMetricsSettings(t *testing.T) {
	for value, want := range map[string]bool{"": true, "true": true, "yes": true, "false": false, " FALSE ": false, "0": false} {
		if got := metricsEnabled(value); got != want {
			t.Errorf("metricsEnabled(%q) = %v, want %v", value, got, want)
		}
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	var label string
	r.GET("/items/:id", func(c *gin.Context) { label = routeLabel(c) })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items/42", nil))

	if label != "/items/:id" {
		t.Fatalf("routeLabel = %q, want the route template", label)
	}
}

// METRICS_ENABLED=false: no /metrics on the router; by default it is there.
func TestInitRouter_MetricsEnabled(t *testing.T) {
	off := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{MetricsEnabled: "false"})
	if w := serve(off, http.MethodGet, "/metrics"); w.Code != http.StatusNotFound {
		t.Fatalf("METRICS_ENABLED=false: /metrics = %d, want 404", w.Code)
	}

	on := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{})
	if w := serve(on, http.MethodGet, "/metrics"); w.Code != http.StatusOK {
		t.Fatalf("default: /metrics = %d, want 200", w.Code)
	}
}

// The Sentry event masks OAuth/OIDC parameters and secret headers (code, state, *token*, X-Api-Key, SENSITIVE_KEYS),
// keeps ordinary ones.
func TestCaptureToSentry_MasksSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(SensitiveKeysEnv, "otp")
	reporter := withPanicReporter(t)

	r := gin.New()
	r.GET("/cb", func(c *gin.Context) { CaptureToSentry(c, errors.New("boom")) })

	req := httptest.NewRequest(http.MethodGet, "/cb?code=C&state=S&refresh_token=R&otp=1&page=2", nil)
	req.Header.Set("X-Api-Key", "K")
	req.Header.Set("Accept", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	events := reporter.all()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}

	headers, _ := events[0].opts.Extra["header"].(map[string]any)
	query, _ := events[0].opts.Extra["query"].(map[string]any)

	for _, v := range []any{query["query_code"], query["query_state"], query["query_refresh_token"], query["query_otp"], headers["header_X-Api-Key"]} {
		if v != "*****" {
			t.Fatalf("event header/query = %v / %v, want secrets masked", headers, query)
		}
	}

	if query["query_page"] != "2" || headers["header_Accept"] != "application/json" {
		t.Fatalf("ordinary values must stay: %v / %v", headers, query)
	}
}

// A WithoutHandlerTimeout path that is not in the router (no group prefix) — a WARN at start; a registered one — none.
func TestCheckTimeoutExemptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Group("/api/v1").GET("/events/:id", func(*gin.Context) {})
	WithoutHandlerTimeout(http.MethodGet, "/api/v1/events/:id")
	WithoutHandlerTimeout(http.MethodGet, "/events/:id")

	out := captureLog(t)
	CheckTimeoutExemptions(r)

	log := out.String()
	if !strings.Contains(log, `"GET /events/:id"`) || strings.Contains(log, `"GET /api/v1/events/:id"`) {
		t.Fatalf("want a WARN only for the path without the group prefix, got %q", log)
	}
}
