package gin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/exgamer/gosdk-core/pkg/errorreporter"
	"github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/gin-gonic/gin"
	goerrors "github.com/go-errors/errors"

	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
)

const panicSecret = "secret /path/file.go"

// panicLeakMarkers must never reach the client when error details are off.
var panicLeakMarkers = []string{"secret", ".go", "postgres://", "goroutine", "gin_helper", "runtime/"}

// panicEvent is one event the reporter received.
type panicEvent struct {
	err  error
	opts errorreporter.Options
}

// panicReporter records events; the name differs from the GitHub-only fakeReporter of package gin_test.
type panicReporter struct {
	mu     sync.Mutex
	events []panicEvent
}

func (r *panicReporter) Capture(_ context.Context, err error, opts errorreporter.Options) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, panicEvent{err: err, opts: opts})
}

func (r *panicReporter) all() []panicEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]panicEvent(nil), r.events...)
}

// Error details off: the body is built from constants, whatever the panic value is.
func TestInitRouter_PanicBodyWithoutDetails(t *testing.T) {
	values := map[string]any{
		"string":         panicSecret,
		"dsn error":      fmt.Errorf("dsn postgres://u:p@h/db"),
		"error":          errors.New("secret"),
		"struct":         struct{ Password string }{"secret"},
		"int":            42,
		"panic(nil)":     nil,
		"wrapped string": fmt.Sprintf("goroutine 1 [running]: %s", panicSecret),
	}
	settings := []string{"", "   ", "false", "0", " False ", "yes", "on", "t", "enabled"}

	withPanicReporter(t)
	captureLog(t)

	for _, setting := range settings {
		for name, value := range values {
			r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{ErrorDetails: setting})
			r.GET("/p", func(*gin.Context) { panic(value) })

			w := serve(r, http.MethodGet, "/p")
			assertPanicBodyWithoutDetails(t, w, fmt.Sprintf("HTTP_ERROR_DETAILS=%q, panic %s", setting, name))

			for _, leak := range []string{"42", "Password"} {
				if strings.Contains(w.Body.String(), leak) {
					t.Errorf("HTTP_ERROR_DETAILS=%q, panic %s: body %q contains %q", setting, name, w.Body.String(), leak)
				}
			}
		}
	}
}

// A panic in a service middleware after the recovery gets the same answer.
func TestInitRouter_MiddlewarePanicWithoutDetails(t *testing.T) {
	withPanicReporter(t)
	captureLog(t)

	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{})
	r.Use(func(*gin.Context) { panic("secret mw") })
	r.GET("/p", func(c *gin.Context) { c.String(http.StatusOK, "handler ran") })

	// The body is the panic answer only: the handler after the panicking middleware does not run.
	assertPanicBodyWithoutDetails(t, serve(r, http.MethodGet, "/p"), "middleware panic")
}

// A handler that already answered keeps its status (gin behaviour); nothing internal is added to the body.
func TestInitRouter_PanicAfterPartialAnswer(t *testing.T) {
	withPanicReporter(t)
	buf := captureLog(t)

	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{})
	r.GET("/p", func(c *gin.Context) {
		c.String(http.StatusOK, "partial")
		panic(panicSecret)
	})

	w := serve(r, http.MethodGet, "/p")

	if !strings.HasPrefix(w.Body.String(), "partial") {
		t.Errorf("GET /p body = %q, want the handler's partial answer first", w.Body.String())
	}

	assertNoPanicLeak(t, strings.TrimPrefix(w.Body.String(), "partial"), "after the partial answer")
	assertPanicLogCount(t, buf, 1)
}

func TestInitRouter_PanicBodyWithDetails(t *testing.T) {
	withPanicReporter(t)
	captureLog(t)

	for _, setting := range []string{"true", "1", " TRUE ", "True"} {
		r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{ErrorDetails: setting})
		r.GET("/p", panicDetailsHandler)

		w := serve(r, http.MethodGet, "/p")
		body := decodePanicBody(t, w, "HTTP_ERROR_DETAILS="+setting)

		if body["message"] != "boom-details" || body["success"] != false || body["service_code"] != float64(0) {
			t.Errorf("HTTP_ERROR_DETAILS=%q: body = %v, want message boom-details, success false, service_code 0", setting, body)
		}

		frames, _ := body["details"].([]any)
		if len(frames) == 0 || !strings.Contains(fmt.Sprint(frames), "panicDetailsHandler") {
			t.Errorf("HTTP_ERROR_DETAILS=%q: details = %v, want stack frames with panicDetailsHandler", setting, frames)
		}
	}
}

// One [ERROR] record and one event per panic; the record holds the panic and its stack, not the request data.
func TestInitRouter_PanicLogAndEvent(t *testing.T) {
	for _, setting := range []string{"", "true"} {
		reporter := withPanicReporter(t)
		buf := captureLog(t)

		r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{ErrorDetails: setting})
		r.Use(withPanicHTTPInfo())
		r.GET("/p", panicDetailsHandler)

		req := httptest.NewRequest(http.MethodGet, "/p", nil)
		req.Header.Set(constants.RequestIdHeaderName, "rid-1")
		serveRequest(r, req)

		out := buf.String()
		if !strings.Contains(out, "[ERROR] [rid-1,GET,/p,500] panic: boom-details") || !strings.Contains(out, "panicDetailsHandler") {
			t.Errorf("HTTP_ERROR_DETAILS=%q: log = %q, want [ERROR] [rid-1,GET,/p,500] panic: boom-details with the stack", setting, out)
		}

		assertPanicLogCount(t, buf, 1)

		events := reporter.all()
		if len(events) != 1 || events[0].opts.Level != errorreporter.LevelError || !strings.Contains(events[0].err.Error(), "boom-details") {
			t.Errorf("HTTP_ERROR_DETAILS=%q: events = %+v, want one LevelError event with boom-details", setting, events)
		}
	}
}

// Headers, query and body reach neither the panic record (at any log level) nor Sentry unmasked.
func TestInitRouter_PanicLogWithoutRequestData(t *testing.T) {
	for _, level := range []logger.Level{logger.LevelInfo, logger.LevelDebug} {
		reporter := withPanicReporter(t)
		buf := captureLog(t)
		logger.SetLevel(level)

		r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{})
		r.Use(withPanicHTTPInfo())
		r.POST("/p", func(*gin.Context) { panic(panicSecret) })

		req := httptest.NewRequest(http.MethodPost, "/p?token=th5-query&access_token=th5-at", strings.NewReader(`{"password":"th5-pass"}`))
		req.Header.Set("Authorization", "Bearer th5-auth")
		req.Header.Set("Cookie", "s=th5-cookie")
		serveRequest(r, req)

		if strings.Contains(buf.String(), "th5-") {
			t.Errorf("level %v: log contains request data:\n%s", level, buf.String())
		}

		if !strings.Contains(buf.String(), ",POST,/p,500] panic: ") {
			t.Errorf("level %v: log = %q, want the path without query", level, buf.String())
		}

		assertPanicEventMasked(t, reporter)
	}
}

// Without RequestInfoMiddleware and reporter: method and path from the request, empty request_id, no panic.
func TestErrorHandler_WithoutHttpInfo(t *testing.T) {
	withPanicReporter(t)
	buf := captureLog(t)
	muteGinErrorWriter(t)

	r := gin.New()
	r.Use(gin.CustomRecovery(ErrorHandler))
	r.GET("/p", func(*gin.Context) { panic(panicSecret) })

	target := "/p?token=th9-query" // not inline: the stack in the log shows the source line of the call
	assertPanicBodyWithoutDetails(t, serve(r, http.MethodGet, target), "ErrorHandler without HttpInfo")

	if !strings.Contains(buf.String(), "[ERROR] [,GET,/p,500] panic: "+panicSecret) || strings.Contains(buf.String(), "th9-") {
		t.Errorf("log = %q, want [ERROR] [,GET,/p,500] without query", buf.String())
	}
}

// The exported ErrorHandler never shows details: not after another router enabled them, not from env.
func TestErrorHandler_GatewayStyleWithoutDetails(t *testing.T) {
	var _ gin.RecoveryFunc = ErrorHandler

	reporter := withPanicReporter(t)
	captureLog(t)
	muteGinErrorWriter(t)
	t.Setenv("HTTP_ERROR_DETAILS", "true")

	withDetails := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{ErrorDetails: "true"})
	withDetails.GET("/p", panicDetailsHandler)

	if body := decodePanicBody(t, serve(withDetails, http.MethodGet, "/p"), "InitRouter true"); body["message"] != "boom-details" {
		t.Errorf("InitRouter with ErrorDetails true: message = %v, want boom-details", body["message"])
	}

	recoveries := map[string]gin.HandlerFunc{
		"CustomRecovery":               gin.CustomRecovery(ErrorHandler),
		"CustomRecoveryWithWriter nil": gin.CustomRecoveryWithWriter(nil, ErrorHandler),
	}

	for name, recovery := range recoveries {
		r := gin.New()
		r.Use(recovery)
		r.GET("/p", panicDetailsHandler)

		assertPanicBodyWithoutDetails(t, serve(r, http.MethodGet, "/p"), name)
	}

	if n := len(reporter.all()); n != 3 {
		t.Errorf("events = %d, want 3: one per panic", n)
	}
}

// Without a reporter (no-op) the answer and the log record are still there.
func TestInitRouter_PanicWithoutReporter(t *testing.T) {
	errorreporter.SetReporter(silentReporter{})
	t.Cleanup(func() { errorreporter.SetReporter(silentReporter{}) })
	buf := captureLog(t)

	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{})
	r.GET("/p", func(*gin.Context) { panic(panicSecret) })

	assertPanicBodyWithoutDetails(t, serve(r, http.MethodGet, "/p"), "no reporter")
	assertPanicLogCount(t, buf, 1)
}

func TestInitRouter_TwoPanicsTwoRecords(t *testing.T) {
	reporter := withPanicReporter(t)
	buf := captureLog(t)

	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{})
	r.GET("/p", func(*gin.Context) { panic(panicSecret) })

	serve(r, http.MethodGet, "/p")
	serve(r, http.MethodGet, "/p")

	assertPanicLogCount(t, buf, 2)

	if n := len(reporter.all()); n != 2 {
		t.Errorf("events = %d, want 2", n)
	}
}

// A panic after HANDLER_TIMEOUT keeps the timeout answer even with details on; the record has status 503.
func TestInitRouter_PanicAfterDeadlineLogAndEvent(t *testing.T) {
	for _, setting := range []string{"", "true"} {
		reporter := withPanicReporter(t)
		buf := captureLog(t)

		r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 1, ErrorDetails: setting})
		r.GET("/panic", func(c *gin.Context) {
			waitDone(c.Request.Context(), 3*time.Second)
			panic(panicSecret)
		})

		assertTimeoutAnswer(t, serve(r, http.MethodGet, "/panic"), "HTTP_ERROR_DETAILS="+setting)

		if !strings.Contains(buf.String(), "[ERROR] [,GET,/panic,503] panic: ") {
			t.Errorf("HTTP_ERROR_DETAILS=%q: log = %q, want the panic record with status 503", setting, buf.String())
		}

		if n := len(reporter.all()); n != 1 {
			t.Errorf("HTTP_ERROR_DETAILS=%q: events = %d, want 1", setting, n)
		}
	}
}

// A panic after the deadline, before the timeout goroutine runs: the record and the event carry 503,
// the status the client gets. Deterministic counterpart of TestInitRouter_PanicAfterDeadlineLogAndEvent.
func TestRespondPanic_DeadlineBeforeTimeoutGoroutine(t *testing.T) {
	reporter := withPanicReporter(t)
	buf := captureLog(t)
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/panic", nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tw := newTimeoutWriter(c.Writer, ctx.Done())
	c.Writer = tw

	respondPanic(c, goerrors.New(panicSecret), true)
	tw.complete(ctx)

	assertTimeoutAnswer(t, w, "panic after the deadline")

	if !strings.Contains(buf.String(), "[ERROR] [,GET,/panic,503] panic: ") {
		t.Errorf("log = %q, want the panic record with status 503", buf.String())
	}

	events := reporter.all()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}

	if data, _ := events[0].opts.Extra["error"].(map[string]any); data["status"] != http.StatusServiceUnavailable {
		t.Errorf("event status = %v, want 503", data["status"])
	}
}

// A panic before HANDLER_TIMEOUT answers 500 without details, the record has status 500.
func TestInitRouter_PanicBeforeDeadline(t *testing.T) {
	reporter := withPanicReporter(t)
	buf := captureLog(t)

	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 1})
	r.GET("/panic", func(*gin.Context) { panic(panicSecret) })

	assertPanicBodyWithoutDetails(t, serve(r, http.MethodGet, "/panic"), "panic before the deadline")

	if !strings.Contains(buf.String(), "[ERROR] [,GET,/panic,500] panic: ") {
		t.Errorf("log = %q, want the panic record with status 500", buf.String())
	}

	if n := len(reporter.all()); n != 1 {
		t.Errorf("events = %d, want 1", n)
	}
}

// One INFO line with the raw value; WARN only for unrecognized; the value is quoted.
func TestInitRouter_ErrorDetailsStartupLog(t *testing.T) {
	cases := []struct {
		setting string
		info    string
		warn    string
	}{
		{setting: "", info: `error details disabled: HTTP_ERROR_DETAILS=""`},
		{setting: "true", info: `error details enabled: HTTP_ERROR_DETAILS="true"`},
		{setting: " TRUE ", info: `error details enabled: HTTP_ERROR_DETAILS=" TRUE "`},
		{setting: "1", info: `error details enabled: HTTP_ERROR_DETAILS="1"`},
		{setting: "0", info: `error details disabled: HTTP_ERROR_DETAILS="0"`},
		{setting: "false", info: `error details disabled: HTTP_ERROR_DETAILS="false"`},
		{setting: " False ", info: `error details disabled: HTTP_ERROR_DETAILS=" False "`},
		{
			setting: "yes",
			info:    `error details disabled: HTTP_ERROR_DETAILS="yes"`,
			warn:    `unrecognized HTTP_ERROR_DETAILS="yes", error details disabled; use true/1 or false/0`,
		},
		{
			setting: "on",
			info:    `error details disabled: HTTP_ERROR_DETAILS="on"`,
			warn:    `unrecognized HTTP_ERROR_DETAILS="on", error details disabled; use true/1 or false/0`,
		},
		{
			setting: "true\n[ERROR] fake",
			info:    `error details disabled: HTTP_ERROR_DETAILS="true\n[ERROR] fake"`,
			warn:    `unrecognized HTTP_ERROR_DETAILS="true\n[ERROR] fake", error details disabled; use true/1 or false/0`,
		},
	}

	for _, tc := range cases {
		buf := captureLog(t)
		newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{ErrorDetails: tc.setting})

		info, warn := errorDetailsLog(buf)

		if len(info) != 1 || !strings.HasSuffix(info[0], "[INFO] "+tc.info) {
			t.Errorf("HTTP_ERROR_DETAILS=%q: INFO lines = %q, want exactly one ending with %q", tc.setting, info, tc.info)
		}

		assertWarn(t, tc.setting, warn, tc.warn)

		for _, line := range strings.Split(buf.String(), "\n") {
			if strings.HasPrefix(line, "[ERROR] fake") {
				t.Errorf("HTTP_ERROR_DETAILS=%q: forged log line %q", tc.setting, line)
			}
		}
	}
}

// The setting is logged once at start, not on every panic.
func TestInitRouter_ErrorDetailsLoggedOnce(t *testing.T) {
	withPanicReporter(t)
	buf := captureLog(t)

	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{})
	r.GET("/p", func(*gin.Context) { panic(panicSecret) })

	for range 3 {
		serve(r, http.MethodGet, "/p")
	}

	if n := strings.Count(buf.String(), "HTTP_ERROR_DETAILS"); n != 1 {
		t.Errorf("HTTP_ERROR_DETAILS mentioned %d times in the log, want 1 (at start)", n)
	}
}

// No gin [Recovery] dump in any gin mode: request headers do not reach stderr.
func TestInitRouter_NoGinRecoveryDump(t *testing.T) {
	for _, mode := range []string{gin.DebugMode, gin.ReleaseMode} {
		withPanicReporter(t)
		buf := captureLog(t)
		logger.SetLevel(logger.LevelDebug) // InitRouter keeps gin debug mode only on the debug log level
		dump := muteGinErrorWriter(t)
		setGinMode(t, mode)

		r := InitRouter(&baseConfig.BaseConfig{Name: "svc"}, &config.HttpConfig{})
		r.GET("/p", func(*gin.Context) { panic(panicSecret) })
		dump.Reset()

		req := httptest.NewRequest(http.MethodGet, "/p", nil)
		req.Header.Set("Cookie", "s=th11")
		req.Header.Set("Authorization", "Bearer th11a")
		serveRequest(r, req)

		if dump.Len() != 0 {
			t.Errorf("gin %s mode: gin error writer got %q, want nothing", mode, dump.String())
		}

		assertPanicLogCount(t, buf, 1)

		if strings.Contains(buf.String(), "th11") {
			t.Errorf("gin %s mode: log contains request headers:\n%s", mode, buf.String())
		}
	}
}

// A broken connection panic: gin aborts without calling the handler, nothing is dumped or reported.
func TestInitRouter_AbortHandlerPanic(t *testing.T) {
	reporter := withPanicReporter(t)
	captureLog(t)
	dump := muteGinErrorWriter(t)

	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{})
	r.GET("/p", func(*gin.Context) { panic(http.ErrAbortHandler) })

	w := serve(r, http.MethodGet, "/p")

	if w.Code == http.StatusInternalServerError || w.Body.Len() != 0 {
		t.Errorf("GET /p = %d %q, want no 500 answer", w.Code, w.Body.String())
	}

	if dump.Len() != 0 || len(reporter.all()) != 0 {
		t.Errorf("gin dump %q, events %d, want none", dump.String(), len(reporter.all()))
	}
}

// Parallel panics and plain requests: each panic answers without details, one event each. Run with -race.
func TestInitRouter_ParallelPanics(t *testing.T) {
	const requests = 50

	reporter := withPanicReporter(t)
	captureLog(t)

	off := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{})
	on := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{ErrorDetails: "true"})

	for _, r := range []*gin.Engine{off, on} {
		r.GET("/panic", panicDetailsHandler)
		r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	}

	results := make([]*httptest.ResponseRecorder, requests)

	var wg sync.WaitGroup
	for i := range requests {
		wg.Go(func() {
			r := off
			if i%4 == 0 {
				r = on
			}

			path := "/ok"
			if i%2 == 0 {
				path = "/panic"
			}

			results[i] = serve(r, http.MethodGet, path)
		})
	}
	wg.Wait()

	for i, w := range results {
		assertParallelResult(t, i, w)
	}

	if n := len(reporter.all()); n != requests/2 {
		t.Errorf("events = %d, want %d", n, requests/2)
	}
}

func panicDetailsHandler(*gin.Context) {
	panic("boom-details")
}

// withPanicHTTPInfo puts HttpInfo into the request context as RequestInfoMiddleware does.
func withPanicHTTPInfo() gin.HandlerFunc {
	return func(c *gin.Context) {
		info := GetInstanceHttpInfo(c)
		c.Request = c.Request.WithContext(WithHttpInfo(c.Request.Context(), info))
		c.Next()
	}
}

// withPanicReporter installs a recording reporter; SetReporter(nil) is a no-op, so cleanup installs a silent one.
func withPanicReporter(t *testing.T) *panicReporter {
	t.Helper()

	reporter := &panicReporter{}
	errorreporter.SetReporter(reporter)
	t.Cleanup(func() { errorreporter.SetReporter(silentReporter{}) })

	return reporter
}

type silentReporter struct{}

func (silentReporter) Capture(context.Context, error, errorreporter.Options) {}

// muteGinErrorWriter redirects gin.DefaultErrorWriter to a buffer for the test.
func muteGinErrorWriter(t *testing.T) *bytes.Buffer {
	t.Helper()

	buf := &bytes.Buffer{}
	prev := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = buf
	t.Cleanup(func() { gin.DefaultErrorWriter = prev })

	return buf
}

func setGinMode(t *testing.T, mode string) {
	t.Helper()

	prev := gin.Mode()
	gin.SetMode(mode)
	t.Cleanup(func() { gin.SetMode(prev) })
}

func serveRequest(r *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	return w
}

func decodePanicBody(t *testing.T, w *httptest.ResponseRecorder, what string) map[string]any {
	t.Helper()

	if w.Code != http.StatusInternalServerError || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Errorf("%s = %d %q, want 500 application/json", what, w.Code, w.Header().Get("Content-Type"))
	}

	body := map[string]any{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s: body %q: %v", what, w.Body.String(), err)
	}

	return body
}

func assertPanicBodyWithoutDetails(t *testing.T, w *httptest.ResponseRecorder, what string) {
	t.Helper()

	body := decodePanicBody(t, w, what)
	details, isList := body["details"].([]any)

	if len(body) != 4 || body["message"] != panicMessage || !isList || len(details) != 0 ||
		body["success"] != false || body["service_code"] != float64(0) {
		t.Errorf("%s: body = %s, want only message %q, details [], success false, service_code 0", what, w.Body.String(), panicMessage)
	}

	if !strings.Contains(w.Body.String(), `"details":[]`) {
		t.Errorf("%s: body = %s, want \"details\":[]", what, w.Body.String())
	}

	assertNoPanicLeak(t, w.Body.String(), what)
}

func assertNoPanicLeak(t *testing.T, body, what string) {
	t.Helper()

	for _, leak := range panicLeakMarkers {
		if strings.Contains(body, leak) {
			t.Errorf("%s: body %q contains %q", what, body, leak)
		}
	}
}

// assertPanicLogCount counts [ERROR] records: the logger prints one per record, whatever the line breaks.
func assertPanicLogCount(t *testing.T, buf *bytes.Buffer, want int) {
	t.Helper()

	if n := strings.Count(buf.String(), "[ERROR]"); n != want || strings.Count(buf.String(), "] panic: ") != want {
		t.Errorf("log has %d [ERROR] records, want %d panic records:\n%s", n, want, buf.String())
	}
}

func assertPanicEventMasked(t *testing.T, reporter *panicReporter) {
	t.Helper()

	events := reporter.all()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}

	headers, _ := events[0].opts.Extra["header"].(map[string]any)
	query, _ := events[0].opts.Extra["query"].(map[string]any)
	masked := []any{headers["header_Authorization"], headers["header_Cookie"], query["query_token"], query["query_access_token"]}

	for _, value := range masked {
		if value != "*****" {
			t.Errorf("event header/query = %v / %v, want Authorization, Cookie, token, access_token masked", headers, query)

			return
		}
	}
}

func assertParallelResult(t *testing.T, i int, w *httptest.ResponseRecorder) {
	t.Helper()

	switch {
	case i%2 != 0:
		if w.Code != http.StatusOK || w.Body.String() != "ok" {
			t.Errorf("request %d: GET /ok = %d %q, want 200 ok", i, w.Code, w.Body.String())
		}
	case i%4 == 0:
		if !strings.Contains(w.Body.String(), "boom-details") {
			t.Errorf("request %d: router with details answered %q", i, w.Body.String())
		}
	default:
		assertPanicBodyWithoutDetails(t, w, fmt.Sprintf("request %d", i))
	}
}

// errorDetailsLog returns the INFO and WARN lines about HTTP_ERROR_DETAILS.
func errorDetailsLog(buf *bytes.Buffer) (info, warn []string) {
	for _, line := range strings.Split(buf.String(), "\n") {
		switch {
		case !strings.Contains(line, "HTTP_ERROR_DETAILS"):
		case strings.Contains(line, "[INFO]"):
			info = append(info, line)
		case strings.Contains(line, "[WARN]"):
			warn = append(warn, line)
		}
	}

	return info, warn
}
