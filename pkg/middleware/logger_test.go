package middleware

import (
	"bytes"
	"errors"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	corelogger "github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	"github.com/exgamer/gosdk-http-core/pkg/response"
	"github.com/gin-gonic/gin"
)

type loggerCase struct {
	name    string
	handler gin.HandlerFunc
	status  int
	tag     string
}

var loggerCases = []loggerCase{
	{
		name:    "exception 4xx",
		handler: func(c *gin.Context) { response.BadRequest(c, errors.New("bad input"), nil) },
		status:  http.StatusBadRequest,
		tag:     "[WARN]",
	},
	{
		name: "exception 4xx via ErrorResponse",
		handler: func(c *gin.Context) {
			response.ErrorResponse(c, exception.NewHttpException(http.StatusNotFound, errors.New("not found"), nil))
		},
		status: http.StatusNotFound,
		tag:    "[WARN]",
	},
	{
		name:    "exception 5xx",
		handler: func(c *gin.Context) { response.InternalServerError(c, errors.New("db down"), nil) },
		status:  http.StatusInternalServerError,
		tag:     "[ERROR]",
	},
	{
		name:    "exception without HttpException",
		handler: func(c *gin.Context) { response.ErrorResponse(c, errors.New("plain error")) },
		status:  http.StatusInternalServerError,
		tag:     "[ERROR]",
	},
	{
		name: "exception 4xx with status 5xx written",
		handler: func(c *gin.Context) {
			c.Set("exception", exception.NewHttpException(http.StatusConflict, errors.New("conflict"), nil))
			c.AbortWithStatus(http.StatusServiceUnavailable)
		},
		status: http.StatusServiceUnavailable,
		tag:    "[ERROR]",
	},
	{
		name:    "status 4xx without exception",
		handler: func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) },
		status:  http.StatusUnauthorized,
		tag:     "[WARN]",
	},
	{
		name:    "status 5xx without exception",
		handler: func(c *gin.Context) { c.AbortWithStatus(http.StatusBadGateway) },
		status:  http.StatusBadGateway,
		tag:     "[ERROR]",
	},
	{
		name: "gin error with status 4xx",
		handler: func(c *gin.Context) {
			_ = c.Error(errors.New("bind failed"))
			c.AbortWithStatus(http.StatusBadRequest)
		},
		status: http.StatusBadRequest,
		tag:    "[ERROR]",
	},
	{
		name:    "success",
		handler: func(c *gin.Context) { c.Status(http.StatusOK) },
		status:  http.StatusOK,
		tag:     "[INFO]",
	},
}

var visibleByTag = map[string]map[corelogger.Level]bool{
	"[INFO]":  {corelogger.LevelDebug: true, corelogger.LevelInfo: true, corelogger.LevelError: false},
	"[WARN]":  {corelogger.LevelDebug: true, corelogger.LevelInfo: true, corelogger.LevelError: false},
	"[ERROR]": {corelogger.LevelDebug: true, corelogger.LevelInfo: true, corelogger.LevelError: true},
}

func TestLoggerMiddleware_LevelByStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, lc := range loggerCases {
		for level, visible := range visibleByTag[lc.tag] {
			t.Run(lc.name+"/"+level.String(), func(t *testing.T) {
				out := captureLog(t, level)

				status := serve(lc.handler)

				if status != lc.status {
					t.Fatalf("status = %d, want %d", status, lc.status)
				}

				assertRequestLine(t, out.String(), lc.tag, visible)
			})
		}
	}
}

func TestLoggerMiddleware_OneLinePerRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, lc := range loggerCases {
		t.Run(lc.name, func(t *testing.T) {
			out := captureLog(t, corelogger.LevelDebug)

			serve(lc.handler)

			lines := strings.Split(strings.TrimSpace(out.String()), "\n")
			if len(lines) != 1 {
				t.Fatalf("want one log line, got %d:\n%s", len(lines), out.String())
			}

			if !strings.Contains(lines[0], lc.tag) {
				t.Fatalf("want %s, got %q", lc.tag, lines[0])
			}
		})
	}
}

func serve(handler gin.HandlerFunc) int {
	engine := gin.New()
	engine.Use(RequestInfoMiddleware(nil), LoggerMiddleware(), FormattedResponseMiddleware())
	engine.GET("/test", handler)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/test", nil))

	return rec.Code
}

func captureLog(t *testing.T, level corelogger.Level) *bytes.Buffer {
	t.Helper()

	prevLevel := corelogger.GetLevel()
	out := &bytes.Buffer{}

	corelogger.SetLogger(stdlog.New(out, "", 0))
	corelogger.SetLevel(level)

	t.Cleanup(func() {
		corelogger.SetLogger(stdlog.New(os.Stdout, "", 0))
		corelogger.SetLevel(prevLevel)
	})

	return out
}

func assertRequestLine(t *testing.T, out string, tag string, visible bool) {
	t.Helper()

	hasLine := strings.Contains(out, tag) && strings.Contains(out, ",GET,/test,")

	if visible && !hasLine {
		t.Fatalf("want %s line for the request, got %q", tag, out)
	}

	if !visible && out != "" {
		t.Fatalf("want no log output, got %q", out)
	}
}
