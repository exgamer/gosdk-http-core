package gin

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/exgamer/gosdk-core/pkg/errorreporter"
	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/config"
)

const statusPanicText = "status panic"

// statusReporter records the response status of each event.
type statusReporter struct {
	mu       sync.Mutex
	statuses []any
}

func (r *statusReporter) Capture(_ context.Context, _ error, opts errorreporter.Options) {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, _ := opts.Extra["error"].(map[string]any)
	r.statuses = append(r.statuses, data["status"])
}

func (r *statusReporter) all() []any {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]any(nil), r.statuses...)
}

type noopStatusReporter struct{}

func (noopStatusReporter) Capture(context.Context, error, errorreporter.Options) {}

// A panic event carries the status the client gets, not the default 200 of the writer.
func TestRespondPanic_SentryStatusMatchesResponse(t *testing.T) {
	cases := []struct {
		name    string
		router  func(t *testing.T) *gin.Engine
		handler gin.HandlerFunc
		want    int
	}{
		{
			name:    "InitRouter panic",
			router:  initRouterNoTimeout,
			handler: func(*gin.Context) { panic(statusPanicText) },
			want:    http.StatusInternalServerError,
		},
		{
			name:    "ErrorHandler panic",
			router:  gatewayRouter,
			handler: func(*gin.Context) { panic(statusPanicText) },
			want:    http.StatusInternalServerError,
		},
		{
			name:   "status set without body",
			router: initRouterNoTimeout,
			handler: func(c *gin.Context) {
				c.Status(http.StatusCreated)
				panic(statusPanicText)
			},
			want: http.StatusInternalServerError,
		},
		{
			name:   "response already sent",
			router: initRouterNoTimeout,
			handler: func(c *gin.Context) {
				c.String(http.StatusCreated, "ok")
				panic(statusPanicText)
			},
			want: http.StatusCreated,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reporter := withStatusReporter(t)
			buf := captureLog(t)

			r := tc.router(t)
			r.GET("/panic", tc.handler)

			w := serve(r, http.MethodGet, "/panic")

			if w.Code != tc.want {
				t.Fatalf("response status = %d, want %d", w.Code, tc.want)
			}

			assertEventStatuses(t, reporter, tc.want)
			assertPanicLogStatus(t, buf, tc.want)
		})
	}
}

// A panic after the handler deadline: the client gets the timeout answer, the event and the log carry 503.
func TestRespondPanic_SentryStatusAfterDeadline(t *testing.T) {
	reporter := withStatusReporter(t)
	buf := captureLog(t)

	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 1})
	r.GET("/panic", func(c *gin.Context) {
		waitDone(c.Request.Context(), 3*time.Second)
		panic(statusPanicText)
	})

	w := serve(r, http.MethodGet, "/panic")

	assertTimeoutAnswer(t, w, "panic after deadline")
	assertEventStatuses(t, reporter, http.StatusServiceUnavailable)
	assertPanicLogStatus(t, buf, http.StatusServiceUnavailable)
}

func initRouterNoTimeout(t *testing.T) *gin.Engine {
	t.Helper()

	return newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{})
}

// gatewayRouter Recovery with the public ErrorHandler, without InitRouter.
func gatewayRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(gin.CustomRecoveryWithWriter(nil, ErrorHandler))

	return r
}

func withStatusReporter(t *testing.T) *statusReporter {
	t.Helper()

	reporter := &statusReporter{}
	errorreporter.SetReporter(reporter)
	t.Cleanup(func() { errorreporter.SetReporter(noopStatusReporter{}) })

	return reporter
}

// assertEventStatuses Exactly one event per panic, with the status the client got.
func assertEventStatuses(t *testing.T, reporter *statusReporter, want int) {
	t.Helper()

	statuses := reporter.all()
	if len(statuses) != 1 {
		t.Fatalf("events = %d, want 1", len(statuses))
	}

	if statuses[0] != want {
		t.Errorf("event status = %v, want %d", statuses[0], want)
	}
}

func assertPanicLogStatus(t *testing.T, buf *bytes.Buffer, want int) {
	t.Helper()

	record := "[ERROR] [,GET,/panic," + strconv.Itoa(want) + "] panic: "

	if n := strings.Count(buf.String(), record); n != 1 {
		t.Errorf("panic records %q = %d, want 1; log = %q", record, n, buf.String())
	}
}
