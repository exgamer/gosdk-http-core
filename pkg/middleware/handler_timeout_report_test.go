package middleware

import (
	"bytes"
	"context"
	"errors"
	"io"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/exgamer/gosdk-core/pkg/errorreporter"
	corelogger "github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
)

// Timeout answer of handlerTimeout (pkg/gin) as the client sees it.
const (
	reportTimeoutBody   = `{"code": -1, "msg":"http: Handler timeout"}`
	reportPanicMessage  = `"message":"internal server error"`
	reportDeadlineLimit = 5 * time.Second // HANDLER_TIMEOUT is 1s; the limit only stops a broken test
	reportForgedLine    = "2026-10-22 00:00:00.000 [INFO] fake"
	reportForgedPath    = "/r/x%0A2026-10-22%2000:00:00.000%20%5BINFO%5D%20fake"
	reportPanicHeader   = "X-Test-Panic"
)

// reportEvent is one event the reporter received.
type reportEvent struct {
	err  error
	opts errorreporter.Options
}

// reportReporter records events; the name differs from fakeReporter of the GitHub copy.
type reportReporter struct {
	mu     sync.Mutex
	events []reportEvent
}

func (r *reportReporter) Capture(_ context.Context, err error, opts errorreporter.Options) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, reportEvent{err: err, opts: opts})
}

func (r *reportReporter) all() []reportEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]reportEvent(nil), r.events...)
}

type reportSilentReporter struct{}

func (reportSilentReporter) Capture(context.Context, error, errorreporter.Options) {}

// A handler that runs past HANDLER_TIMEOUT without an error — 503 to the client,
// one [ERROR] request line with status 503, no events (the timeout itself is not sent to Sentry).
func TestHandlerTimeoutReport_TimeoutWithoutError(t *testing.T) {
	reporter := withReportReporter(t)
	r := newReportRouter(t)
	buf := captureReportLog(t)
	r.GET("/slow", func(c *gin.Context) {
		waitDeadline(c.Request.Context())
	})

	w := serveReport(r, http.MethodGet, "/slow", false)

	assertReportTimeoutAnswer(t, w)
	assertOneErrorLine(t, buf, "[rid-report,GET,/slow,503] ")
	assertEvents(t, reporter, 0)
}

// A panic before the deadline — 500 without details, one LevelError event,
// one [ERROR] panic record with status 500 (LoggerMiddleware does not write after a panic).
func TestHandlerTimeoutReport_PanicBeforeDeadline(t *testing.T) {
	reporter := withReportReporter(t)
	r := newReportRouter(t)
	buf := captureReportLog(t)
	r.GET("/panic", func(*gin.Context) { panic("report boom") })

	w := serveReport(r, http.MethodGet, "/panic", false)

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), reportPanicMessage) {
		t.Errorf("GET /panic = %d %q, want 500 with %s", w.Code, w.Body.String(), reportPanicMessage)
	}

	assertOneErrorLine(t, buf, "[rid-report,GET,/panic,500] panic: report boom")
	assertEvents(t, reporter, 1)
}

// An exception 500 before the deadline — one LevelError event, one [ERROR] request line with status 500.
func TestHandlerTimeoutReport_ExceptionBeforeDeadline(t *testing.T) {
	reporter := withReportReporter(t)
	r := newReportRouter(t)
	buf := captureReportLog(t)
	r.GET("/exception", func(c *gin.Context) {
		c.Set("exception", exception.NewInternalServerErrorException(errors.New("db down"), nil))
		c.AbortWithStatus(http.StatusInternalServerError)
	})

	w := serveReport(r, http.MethodGet, "/exception", false)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("GET /exception = %d, want 500", w.Code)
	}

	assertOneErrorLine(t, buf, "[rid-report,GET,/exception,500] db down")
	assertEvents(t, reporter, 1)
}

// A panic after the deadline — the client keeps the timeout answer, one event,
// one [ERROR] panic record with status 503. Run with -race.
func TestHandlerTimeoutReport_PanicAfterDeadline(t *testing.T) {
	reporter := withReportReporter(t)
	r := newReportRouter(t)
	buf := captureReportLog(t)
	r.GET("/late-panic", func(c *gin.Context) {
		waitDeadline(c.Request.Context())
		panic("report late boom")
	})

	w := serveReport(r, http.MethodGet, "/late-panic", false)

	assertReportTimeoutAnswer(t, w)
	assertOneErrorLine(t, buf, "[rid-report,GET,/late-panic,503] panic: report late boom")
	assertEvents(t, reporter, 1)
}

// The panic record and the LoggerMiddleware line write the same path (URL.Path), without own escaping.
func TestHandlerTimeoutReport_PanicPathAsLoggerMiddleware(t *testing.T) {
	withReportReporter(t)
	r := newReportRouter(t)
	buf := captureReportLog(t)
	r.GET("/r/*any", reportCatchAllHandler) // /*any conflicts with /metrics of InitRouter

	serveReport(r, http.MethodGet, "/r/a/b?q=1", false)
	serveReport(r, http.MethodGet, "/r/a/b?q=1", true)

	out := buf.String()
	for _, want := range []string{"[INFO] [rid-report,GET,/r/a/b,200] ", "[ERROR] [rid-report,GET,/r/a/b,500] panic: "} {
		if !strings.Contains(out, want) {
			t.Errorf("log = %q, want %q", out, want)
		}
	}
}

// Log forging: a path with %0A through LoggerMiddleware and through the panic record stays one physical line
// with a literal \n; no forged [INFO] line.
func TestHandlerTimeoutReport_ForgedPathOneLine(t *testing.T) {
	cases := []struct {
		name  string
		panic bool
		tag   string
	}{
		{name: "LoggerMiddleware", panic: false, tag: "[INFO]"},
		{name: "panic record", panic: true, tag: "[ERROR]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withReportReporter(t)
			r := newReportRouter(t)
			buf := captureReportLog(t)
			r.GET("/r/*any", reportCatchAllHandler) // /*any conflicts with /metrics of InitRouter

			serveReport(r, http.MethodGet, reportForgedPath, tc.panic)

			lines := reportLines(buf)
			if len(lines) != 1 {
				t.Fatalf("want one log line, got %d:\n%s", len(lines), buf.String())
			}

			if !strings.Contains(lines[0], tc.tag) || !strings.Contains(lines[0], `,GET,/r/x\n`+reportForgedLine+",") {
				t.Errorf("log line = %q, want %s with the path and a literal \\n", lines[0], tc.tag)
			}
		})
	}
}

// newReportRouter builds the chain as in a service: InitRouter (HandlerTimeout: 1) → RequestInfoMiddleware
// → LoggerMiddleware → SentryMiddleware. The startup log of InitRouter is discarded; call captureReportLog after.
func newReportRouter(t *testing.T) *gin.Engine {
	t.Helper()

	mode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(mode) })

	corelogger.SetLogger(stdlog.New(io.Discard, "", 0))
	t.Cleanup(func() { corelogger.SetLogger(stdlog.New(os.Stdout, "", 0)) })

	r := gin2.InitRouter(&baseConfig.BaseConfig{Name: "svc"}, &config.HttpConfig{HandlerTimeout: 1})
	r.Use(RequestInfoMiddleware(nil), LoggerMiddleware(), SentryMiddleware())

	return r
}

// captureReportLog redirects the core logger to a buffer at the info level.
func captureReportLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	prevLevel := corelogger.GetLevel()
	buf := &bytes.Buffer{}

	corelogger.SetLogger(stdlog.New(buf, "", 0))
	corelogger.SetLevel(corelogger.LevelInfo)

	t.Cleanup(func() {
		corelogger.SetLogger(stdlog.New(os.Stdout, "", 0))
		corelogger.SetLevel(prevLevel)
	})

	return buf
}

// withReportReporter installs a recording reporter; SetReporter(nil) is a no-op, so cleanup installs a silent one.
func withReportReporter(t *testing.T) *reportReporter {
	t.Helper()

	reporter := &reportReporter{}
	errorreporter.SetReporter(reporter)
	t.Cleanup(func() { errorreporter.SetReporter(reportSilentReporter{}) })

	return reporter
}

// serveReport serves the request with the request id rid-report; withPanic asks reportCatchAllHandler to panic.
func serveReport(r *gin.Engine, method, target string, withPanic bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	req.Header.Set(constants.RequestIdHeaderName, "rid-report")

	if withPanic {
		req.Header.Set(reportPanicHeader, "1")
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	return w
}

func reportCatchAllHandler(c *gin.Context) {
	if c.GetHeader(reportPanicHeader) != "" {
		panic("report path boom")
	}

	c.String(http.StatusOK, "ok")
}

// waitDeadline waits for the request context to end (HANDLER_TIMEOUT); the limit only stops a broken test.
func waitDeadline(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(reportDeadlineLimit):
	}
}

func reportLines(buf *bytes.Buffer) []string {
	out := strings.TrimSpace(buf.String())
	if out == "" {
		return nil
	}

	return strings.Split(out, "\n")
}

func assertReportTimeoutAnswer(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()

	if w.Code != http.StatusServiceUnavailable || w.Body.String() != reportTimeoutBody {
		t.Errorf("answer = %d %q, want 503 %s", w.Code, w.Body.String(), reportTimeoutBody)
	}
}

// assertOneErrorLine checks that the log has exactly one line and it is [ERROR] with want.
func assertOneErrorLine(t *testing.T, buf *bytes.Buffer, want string) {
	t.Helper()

	lines := reportLines(buf)
	if len(lines) != 1 || !strings.Contains(lines[0], "[ERROR] "+want) {
		t.Errorf("log = %q, want one line [ERROR] %s", lines, want)
	}
}

// assertEvents checks the number of events; each must be LevelError.
func assertEvents(t *testing.T, reporter *reportReporter, want int) {
	t.Helper()

	events := reporter.all()
	if len(events) != want {
		t.Errorf("events = %d, want %d: %+v", len(events), want, events)
	}

	for _, event := range events {
		if event.opts.Level != errorreporter.LevelError {
			t.Errorf("event %v: level = %q, want %q", event.err, event.opts.Level, errorreporter.LevelError)
		}
	}
}
