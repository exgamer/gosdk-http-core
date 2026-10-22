package app

import (
	"bytes"
	"context"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	coreConfig "github.com/exgamer/gosdk-core/pkg/config"
	coreContext "github.com/exgamer/gosdk-core/pkg/context"
	corelogger "github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
	"github.com/exgamer/gosdk-http-core/pkg/middleware"
)

type requestKey struct{}

// The app context is cancelled on SIGTERM before Shutdown waits for
// in-flight requests: a request must keep its own context and only get
// AppInfo from the app.
func TestWithAppInfo_KeepsTheRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ai := &coreConfig.AppInfo{ServiceName: "svc"}
	appCtx, stopApp := context.WithCancel(coreContext.WithAppInfo(context.Background(), ai))
	stopApp() // SIGTERM already received

	engine := gin.New()
	engine.Use(withAppInfo(func() context.Context { return appCtx }))

	var appInfo *coreConfig.AppInfo
	var own any
	var ctxErr error

	engine.GET("/", func(c *gin.Context) {
		ctx := c.Request.Context()
		appInfo, own, ctxErr = coreContext.GetAppInfoFromContext(ctx), ctx.Value(requestKey{}), ctx.Err()
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), requestKey{}, "own"))
	engine.ServeHTTP(httptest.NewRecorder(), req)

	if appInfo != ai {
		t.Errorf("AppInfo = %+v, want the app's AppInfo added to the request context", appInfo)
	}

	if own != "own" {
		t.Errorf("request value = %v, want the request's own context kept", own)
	}

	if ctxErr != nil {
		t.Errorf("request context err = %v, want the app's cancellation not to reach the request", ctxErr)
	}
}

// A client that left cancels its request.
func TestWithAppInfo_ClientDisconnectCancels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(withAppInfo(context.Background))

	var ctxErr error

	engine.GET("/", func(c *gin.Context) { ctxErr = c.Request.Context().Err() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))

	if ctxErr == nil {
		t.Error("request context not cancelled although the client left")
	}
}

// An app context without AppInfo leaves the request without it.
func TestWithAppInfo_NoAppInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(withAppInfo(context.Background))

	var appInfo *coreConfig.AppInfo
	called := false

	engine.GET("/", func(c *gin.Context) {
		called = true
		appInfo = coreContext.GetAppInfoFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !called || rec.Code != http.StatusOK {
		t.Fatalf("handler called = %v, status = %d, want the request served with 200", called, rec.Code)
	}

	if appInfo != nil {
		t.Errorf("AppInfo = %+v, want nil", appInfo)
	}
}

// AppInfo put into the app context under the string key "app_info" does not reach the request.
func TestWithAppInfo_StringKeyIgnored(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ai := &coreConfig.AppInfo{ServiceName: "svc"}
	appCtx := context.WithValue(context.Background(), "app_info", ai) //nolint:staticcheck // SA1029: проверяем, что строковый ключ не даёт AppInfo запросу

	engine := gin.New()
	engine.Use(withAppInfo(func() context.Context { return appCtx }))

	var appInfo *coreConfig.AppInfo
	called := false

	engine.GET("/", func(c *gin.Context) {
		called = true
		appInfo = coreContext.GetAppInfoFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !called || rec.Code != http.StatusOK {
		t.Fatalf("handler called = %v, status = %d, want the request served with 200", called, rec.Code)
	}

	if appInfo != nil {
		t.Errorf("AppInfo = %+v, want nil for the string key", appInfo)
	}
}

// The request log line gets the service name from AppInfo and the request id from HttpInfo.
func TestWithAppInfo_ReachesTheRequestLogLine(t *testing.T) {
	gin.SetMode(gin.TestMode)

	out := captureKernelLog(t)
	appCtx := coreContext.WithAppInfo(context.Background(), &coreConfig.AppInfo{ServiceName: "svc"})

	engine := gin.New()
	engine.Use(withAppInfo(func() context.Context { return appCtx }), middleware.RequestInfoMiddleware(nil), middleware.LoggerMiddleware())
	engine.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(constants.RequestIdHeaderName, "rid-95")
	engine.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(out.String(), "[INFO,svc]") || !strings.Contains(out.String(), "rid-95") {
		t.Errorf("log = %q, want a request line with [INFO,svc] and rid-95", out.String())
	}
}

func captureKernelLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	prevLevel := corelogger.GetLevel()
	out := &bytes.Buffer{}

	corelogger.SetLogger(stdlog.New(out, "", 0))
	corelogger.SetLevel(corelogger.LevelDebug)

	t.Cleanup(func() {
		corelogger.SetLogger(stdlog.New(os.Stdout, "", 0))
		corelogger.SetLevel(prevLevel)
	})

	return out
}

func TestNewServer_Timeouts(t *testing.T) {
	cases := []struct {
		name                          string
		cfg                           config.HttpConfig
		read, readHeader, write, idle time.Duration
	}{
		{
			name:       "unset: the values hardcoded before these settings existed",
			cfg:        config.HttpConfig{},
			read:       15 * time.Second,
			readHeader: 10 * time.Second,
			write:      30 * time.Second,
			idle:       60 * time.Second,
		},
		{
			name:       "set",
			cfg:        config.HttpConfig{ServerReadTimeoutSec: 5, ServerReadHeaderTimeoutSec: 2, ServerWriteTimeoutSec: 120, ServerIdleTimeoutSec: 90},
			read:       5 * time.Second,
			readHeader: 2 * time.Second,
			write:      120 * time.Second,
			idle:       90 * time.Second,
		},
		{
			name:       "-1 disables, others keep defaults",
			cfg:        config.HttpConfig{ServerWriteTimeoutSec: -1},
			read:       15 * time.Second,
			readHeader: 10 * time.Second,
			write:      0,
			idle:       60 * time.Second,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.cfg.ServerAddress = "127.0.0.1:0"
			handler := http.NewServeMux()
			srv := newServer(&tc.cfg, handler)

			if srv.Addr != "127.0.0.1:0" || srv.Handler != handler {
				t.Errorf("address/handler not passed through: %q %v", srv.Addr, srv.Handler)
			}

			got := [4]time.Duration{srv.ReadTimeout, srv.ReadHeaderTimeout, srv.WriteTimeout, srv.IdleTimeout}
			want := [4]time.Duration{tc.read, tc.readHeader, tc.write, tc.idle}

			if got != want {
				t.Errorf("timeouts (read, readHeader, write, idle) = %v, want %v", got, want)
			}
		})
	}
}
