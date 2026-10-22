package gin

import (
	"bytes"
	"encoding/json"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/config"
)

const notFoundBody = `{"code":"PAGE_NOT_FOUND","message":"404 page not found"}`

// InitRouter is called many times in one process: Prometheus logs a
// re-registration error each time; no test relies on it.

type swaggerEnabledCase struct {
	name       string
	settings   []string
	enabled    bool
	recognized bool
}

func TestSwaggerEnabled(t *testing.T) {
	cases := []swaggerEnabledCase{
		{name: "unset or blank", settings: []string{"", "   ", "\t\n"}, enabled: false, recognized: true},
		{name: "on", settings: []string{"true", "1", " True ", "TRUE", "1\n"}, enabled: true, recognized: true},
		{name: "off", settings: []string{"false", "0", "FALSE", " False ", "0\n"}, enabled: false, recognized: true},
		{
			name: "unrecognized: not strconv.ParseBool, no yes/on",
			settings: []string{
				"yes", "on", "t", "T", "f", "F", "ture", "flase", "enabled",
				"2", "-1", "true1", "1 0", "да", "no", "off",
			},
			enabled:    false,
			recognized: false,
		},
	}

	for _, tc := range cases {
		for _, setting := range tc.settings {
			assertSwaggerEnabled(t, tc, setting)
		}
	}
}

func TestInitRouter_SwaggerOffClosesWholePath(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{SwaggerPrefix: "docs"})

	// The whole path is closed: UI, schema and static files.
	paths := []string{"/docs/index.html", "/docs/doc.json", "/docs/swagger-ui.css", "/docs/swagger-ui-bundle.js", "/docs/", "/docs"}
	for _, path := range paths {
		assertNotFound(t, serve(r, http.MethodGet, path), "GET "+path)
	}

	// 404, not 405 — the route must not look existing.
	requests := []struct{ method, path string }{
		{http.MethodPost, "/docs/x"},
		{http.MethodPut, "/docs/index.html"},
		{http.MethodDelete, "/docs/doc.json"},
	}
	for _, req := range requests {
		assertNotFound(t, serve(r, req.method, req.path), req.method+" "+req.path)
	}

	// Indistinguishable from an unknown path.
	docs := serve(r, http.MethodGet, "/docs/doc.json")
	unknown := serve(r, http.MethodGet, "/no-such-path-xyz")

	if docs.Code != unknown.Code || docs.Body.String() != unknown.Body.String() ||
		docs.Header().Get("Content-Type") != unknown.Header().Get("Content-Type") {
		t.Errorf("disabled swagger differs from unknown path: %d %q %q vs %d %q %q",
			docs.Code, docs.Header().Get("Content-Type"), docs.Body.String(),
			unknown.Code, unknown.Header().Get("Content-Type"), unknown.Body.String())
	}
}

// Unset, off and unrecognized values on any APP_ENV: APP_ENV never enables swagger.
func TestInitRouter_SwaggerOffOnAnyEnv(t *testing.T) {
	settings := []string{"", "   ", "false", "0", "FALSE", " False ", "yes", "on", "t", "ture", "enabled", "2"}
	envs := []string{"", "local", "stage", "production"}

	for _, setting := range settings {
		for _, env := range envs {
			base := &baseConfig.BaseConfig{Name: "svc", AppEnv: env}
			r := newRouter(t, base, config.HttpConfig{SwaggerPrefix: "docs", SwaggerEnabled: setting})

			assertNotFound(t, serve(r, http.MethodGet, "/docs/index.html"),
				"SWAGGER_ENABLED="+setting+" APP_ENV="+env)
		}
	}
}

func TestInitRouter_SwaggerOn(t *testing.T) {
	// APP_ENV does not matter.
	for _, setting := range []string{"true", "1", " True ", "TRUE"} {
		for _, env := range []string{"", "production"} {
			base := &baseConfig.BaseConfig{Name: "svc", AppEnv: env}
			r := newRouter(t, base, config.HttpConfig{SwaggerPrefix: "docs", SwaggerEnabled: setting})

			assertSwaggerUI(t, serve(r, http.MethodGet, "/docs/index.html"), "SWAGGER_ENABLED="+setting+" APP_ENV="+env)
		}
	}

	// doc.json reaches the swagger handler (its code depends on a registered swag doc).
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{SwaggerPrefix: "docs", SwaggerEnabled: "true"})
	w := serve(r, http.MethodGet, "/docs/doc.json")

	if w.Code == http.StatusNotFound && w.Body.String() == notFoundBody {
		t.Error("GET /docs/doc.json: got NoRoute, want the swagger handler")
	}
}

// A call as in gateway tests: compiles, swagger is off until SwaggerEnabled is set.
func TestInitRouter_GatewayStyleCall(t *testing.T) {
	base := &baseConfig.BaseConfig{Name: "gateway"}

	r := newRouter(t, base, config.HttpConfig{SwaggerPrefix: "docs"})
	assertNotFound(t, serve(r, http.MethodGet, "/docs/index.html"), "without SwaggerEnabled")

	r = newRouter(t, base, config.HttpConfig{SwaggerPrefix: "docs", SwaggerEnabled: "true"})
	assertSwaggerUI(t, serve(r, http.MethodGet, "/docs/index.html"), "SwaggerEnabled=true")
}

// The prefix rule SWAGGER_PREFIX → APP_NAME → swagger is unchanged.
func TestInitRouter_SwaggerPrefix(t *testing.T) {
	cases := []struct {
		name     string
		appName  string
		prefix   string
		setting  string
		uiPath   string
		notFound []string
	}{
		{name: "off, prefix from APP_NAME", appName: "svc", notFound: []string{"/svc/index.html", "/swagger/index.html"}},
		{name: "on, prefix from APP_NAME", appName: "svc", setting: "true", uiPath: "/svc/index.html", notFound: []string{"/swagger/index.html"}},
		{name: "on, default prefix", setting: "true", uiPath: "/swagger/index.html"},
		{name: "off, no bypass prefix", appName: "svc", prefix: "docs", notFound: []string{"/docs/index.html", "/svc/index.html", "/swagger/index.html"}},
		{name: "on, SWAGGER_PREFIX wins", appName: "svc", prefix: "docs", setting: "true", uiPath: "/docs/index.html", notFound: []string{"/svc/index.html"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := &baseConfig.BaseConfig{Name: tc.appName}
			r := newRouter(t, base, config.HttpConfig{SwaggerPrefix: tc.prefix, SwaggerEnabled: tc.setting})

			if tc.uiPath != "" {
				assertSwaggerUI(t, serve(r, http.MethodGet, tc.uiPath), tc.uiPath)
			}

			for _, path := range tc.notFound {
				assertNotFound(t, serve(r, http.MethodGet, path), path)
			}
		})
	}
}

// With SWAGGER_PREFIX set InitRouter(nil, …) does not panic: the decision does not read baseConfig.
func TestInitRouter_NilBaseConfigWithPrefix(t *testing.T) {
	cases := map[string]bool{"": false, "true": true, "yes": false}

	for setting, on := range cases {
		r := newRouterNoPanic(t, nil, config.HttpConfig{SwaggerPrefix: "docs", SwaggerEnabled: setting})

		w := serve(r, http.MethodGet, "/docs/index.html")
		if on {
			assertSwaggerUI(t, w, "SWAGGER_ENABLED="+setting)

			continue
		}

		assertNotFound(t, w, "SWAGGER_ENABLED="+setting)
	}
}

// With swagger off the rest of InitRouter behaves as before, the handler timeout included.
func TestInitRouter_SwaggerOffKeepsRouterBehaviour(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{SwaggerPrefix: "docs", HandlerTimeout: 1})
	r.GET("/docs/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/boom", func(*gin.Context) { panic("x") })
	r.GET("/slow", func(c *gin.Context) {
		waitDone(c.Request.Context(), 2*time.Second)
		c.String(http.StatusOK, "late")
	})

	if w := serve(r, http.MethodGet, "/docs/x"); w.Code != http.StatusOK || w.Body.String() != "ok" {
		t.Errorf("GET /docs/x = %d %q, want 200 ok", w.Code, w.Body.String())
	}

	if w := serve(r, http.MethodGet, "/metrics"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "# HELP") {
		t.Errorf("GET /metrics = %d, want 200 with prometheus metrics", w.Code)
	}

	if w := serve(r, http.MethodPost, "/docs/x"); w.Code != http.StatusMethodNotAllowed || !strings.Contains(w.Body.String(), `"METHOD_NOT_ALLOWED"`) {
		t.Errorf("POST /docs/x = %d %q, want 405 METHOD_NOT_ALLOWED", w.Code, w.Body.String())
	}

	assertNotFound(t, serve(r, http.MethodGet, "/no-such-path-xyz"), "GET /no-such-path-xyz")

	if w := serve(r, http.MethodGet, "/boom"); w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), `"success":false`) {
		t.Errorf("GET /boom = %d %q, want 500 from ErrorHandler", w.Code, w.Body.String())
	}

	assertTimeoutAnswer(t, serve(r, http.MethodGet, "/slow"), "GET /slow")
}

// With swagger off the service owns /<prefix>/*any.
func TestInitRouter_SwaggerOffServiceOwnsPrefix(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{SwaggerPrefix: "docs"})
	r.GET("/docs/*any", func(c *gin.Context) { c.String(http.StatusOK, "service") })

	if w := serve(r, http.MethodGet, "/docs/index.html"); w.Code != http.StatusOK || w.Body.String() != "service" {
		t.Errorf("GET /docs/index.html = %d %q, want the service handler", w.Code, w.Body.String())
	}
}

// With swagger on a service route under the prefix clashes, as before.
func TestInitRouter_SwaggerOnPrefixClashPanics(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{SwaggerPrefix: "docs", SwaggerEnabled: "true"})

	defer func() {
		if recover() == nil {
			t.Error("GET /docs/x registered without a gin route conflict, want panic as before")
		}
	}()

	r.GET("/docs/x", func(*gin.Context) {})
}

// Several routers in one process answer by their own config.
func TestInitRouter_SeveralRoutersInOneProcess(t *testing.T) {
	base := &baseConfig.BaseConfig{Name: "svc"}
	off := newRouter(t, base, config.HttpConfig{SwaggerPrefix: "docs"})
	on := newRouter(t, base, config.HttpConfig{SwaggerPrefix: "docs", SwaggerEnabled: "true"})
	offExplicit := newRouter(t, base, config.HttpConfig{SwaggerPrefix: "docs", SwaggerEnabled: "false"})

	assertNotFound(t, serve(off, http.MethodGet, "/docs/index.html"), "unset")
	assertSwaggerUI(t, serve(on, http.MethodGet, "/docs/index.html"), "true")
	assertNotFound(t, serve(offExplicit, http.MethodGet, "/docs/index.html"), "false")
}

// One INFO line, WARN only for unrecognized, only the setting value.
func TestInitRouter_SwaggerStartupLog(t *testing.T) {
	cases := []struct {
		setting string
		info    string
		warn    string
	}{
		{setting: "", info: `swagger disabled: SWAGGER_ENABLED=""`},
		{setting: "false", info: `swagger disabled: SWAGGER_ENABLED="false"`},
		{setting: "true", info: `swagger enabled: SWAGGER_ENABLED="true"`},
		{
			setting: "yes",
			info:    `swagger disabled: SWAGGER_ENABLED="yes"`,
			warn:    `unrecognized SWAGGER_ENABLED="yes", swagger disabled; use true/1 or false/0`,
		},
		{setting: " True ", info: `swagger enabled: SWAGGER_ENABLED=" True "`},
	}

	for _, tc := range cases {
		buf := captureLog(t)
		newRouter(t, &baseConfig.BaseConfig{Name: "svc", AppEnv: "production"}, config.HttpConfig{SwaggerPrefix: "docs", SwaggerEnabled: tc.setting})

		info, warn := splitSwaggerLog(t, tc.setting, buf)

		if len(info) != 1 || !strings.HasSuffix(info[0], "[INFO] "+tc.info) {
			t.Errorf("SWAGGER_ENABLED=%q: INFO lines = %q, want exactly one ending with %q", tc.setting, info, tc.info)
		}

		assertWarn(t, tc.setting, warn, tc.warn)

		// Only the setting value is logged: no APP_ENV, no HttpConfig dump.
		for _, leak := range []string{"production", "APP_ENV", "SWAGGER_PREFIX", "swagger_prefix", "swagger_enabled"} {
			assertNoLeak(t, tc.setting, buf, leak)
		}
	}
}

// The setting is a string, any value reads without error.
func TestHttpConfig_SwaggerEnabledFromEnv(t *testing.T) {
	cases := []struct {
		name  string
		value string
		set   bool
		want  []string
	}{
		{name: "unrecognized", value: "yes", set: true, want: []string{"yes"}},
		{name: "true", value: "true", set: true, want: []string{"true"}},
		{name: "zero", value: "0", set: true, want: []string{"0"}},
		{name: "padded", value: " True ", set: true, want: []string{" True ", "True"}},
		{name: "empty", value: "", set: true, want: []string{""}},
		{name: "unset", want: []string{""}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SWAGGER_PREFIX", "docs")
			setSwaggerEnv(t, tc.value, tc.set)

			cfg := config.HttpConfig{}
			if err := baseConfig.InitConfig(&cfg); err != nil {
				t.Fatalf("InitConfig: %v", err)
			}

			if !slices.Contains(tc.want, cfg.SwaggerEnabled) {
				t.Errorf("SwaggerEnabled = %q, want one of %q", cfg.SwaggerEnabled, tc.want)
			}

			if tc.value != "" {
				return
			}

			// A service updated without env changes loses swagger.
			r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, cfg)
			assertNotFound(t, serve(r, http.MethodGet, "/docs/index.html"), "env SWAGGER_ENABLED unset/empty")
		})
	}
}

// The logger.Dump key matches neighbouring fields.
func TestHttpConfig_SwaggerEnabledJSON(t *testing.T) {
	data, err := json.Marshal(config.HttpConfig{SwaggerEnabled: "true"})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	if !strings.Contains(string(data), `"swagger_enabled":"true"`) {
		t.Errorf("json = %s, want \"swagger_enabled\":\"true\"", data)
	}
}

func newRouter(t *testing.T, base *baseConfig.BaseConfig, cfg config.HttpConfig) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	return InitRouter(base, &cfg)
}

// newRouterNoPanic fails the test if InitRouter panics.
func newRouterNoPanic(t *testing.T, base *baseConfig.BaseConfig, cfg config.HttpConfig) *gin.Engine {
	t.Helper()

	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("SWAGGER_ENABLED=%q: InitRouter(%v, …) panicked: %v", cfg.SwaggerEnabled, base, p)
		}
	}()

	return newRouter(t, base, cfg)
}

func serve(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))

	return w
}

func assertNotFound(t *testing.T, w *httptest.ResponseRecorder, what string) {
	t.Helper()

	if w.Code != http.StatusNotFound || w.Body.String() != notFoundBody {
		t.Errorf("%s = %d %q, want 404 %s", what, w.Code, w.Body.String(), notFoundBody)
	}
}

func assertSwaggerUI(t *testing.T, w *httptest.ResponseRecorder, what string) {
	t.Helper()

	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
		t.Errorf("%s = %d %q, want 200 text/html (Swagger UI)", what, w.Code, w.Header().Get("Content-Type"))
	}
}

func assertSwaggerEnabled(t *testing.T, tc swaggerEnabledCase, setting string) {
	t.Helper()

	enabled, recognized := swaggerEnabled(setting)
	if enabled != tc.enabled || recognized != tc.recognized {
		t.Errorf("%s: swaggerEnabled(%q) = (%v, %v), want (%v, %v)",
			tc.name, setting, enabled, recognized, tc.enabled, tc.recognized)
	}
}

// splitSwaggerLog returns the INFO and WARN lines about SWAGGER_ENABLED; any other such line fails the test.
func splitSwaggerLog(t *testing.T, setting string, buf *bytes.Buffer) (info, warn []string) {
	t.Helper()

	for _, line := range strings.Split(buf.String(), "\n") {
		switch {
		case !strings.Contains(line, "SWAGGER_ENABLED"):
		case strings.Contains(line, "[INFO]"):
			info = append(info, line)
		case strings.Contains(line, "[WARN]"):
			warn = append(warn, line)
		default:
			t.Errorf("SWAGGER_ENABLED=%q: unexpected log line %q", setting, line)
		}
	}

	return info, warn
}

func assertNoLeak(t *testing.T, setting string, buf *bytes.Buffer, leak string) {
	t.Helper()

	if strings.Contains(buf.String(), leak) {
		t.Errorf("SWAGGER_ENABLED=%q: startup log contains %q:\n%s", setting, leak, buf.String())
	}
}

func assertWarn(t *testing.T, setting string, warn []string, want string) {
	t.Helper()

	if want == "" {
		if len(warn) != 0 {
			t.Errorf("SWAGGER_ENABLED=%q: WARN lines = %q, want none", setting, warn)
		}

		return
	}

	if len(warn) != 1 || !strings.HasSuffix(warn[0], "[WARN] "+want) {
		t.Errorf("SWAGGER_ENABLED=%q: WARN lines = %q, want exactly one ending with %q", setting, warn, want)
	}
}

// captureLog redirects logger output to a buffer for the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	buf := &bytes.Buffer{}
	level := logger.GetLevel()
	logger.SetLogger(stdlog.New(buf, "", 0))
	logger.SetLevel(logger.LevelInfo)

	t.Cleanup(func() {
		logger.SetLogger(stdlog.New(os.Stdout, "", 0))
		logger.SetLevel(level)
	})

	return buf
}

func setSwaggerEnv(t *testing.T, value string, set bool) {
	t.Helper()

	if set {
		t.Setenv("SWAGGER_ENABLED", value)

		return
	}

	t.Setenv("SWAGGER_ENABLED", "")
	if err := os.Unsetenv("SWAGGER_ENABLED"); err != nil {
		t.Fatalf("unset SWAGGER_ENABLED: %v", err)
	}
}
