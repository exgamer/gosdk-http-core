package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coreConstants "github.com/exgamer/gosdk-core/pkg/constants"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/gin-gonic/gin"
)

type requestKey struct{}

// The app context is cancelled on SIGTERM before Shutdown waits for
// in-flight requests: a request must keep its own context and only get
// AppInfo from the app.
func TestWithAppInfo_KeepsTheRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	appCtx, stopApp := context.WithCancel(context.WithValue(context.Background(), coreConstants.AppInfoKey, "app-info"))
	stopApp() // SIGTERM already received

	engine := gin.New()
	engine.Use(withAppInfo(func() context.Context { return appCtx }))

	var appInfo, own any
	var ctxErr error

	engine.GET("/", func(c *gin.Context) {
		ctx := c.Request.Context()
		appInfo, own, ctxErr = ctx.Value(coreConstants.AppInfoKey), ctx.Value(requestKey{}), ctx.Err()
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), requestKey{}, "own"))
	engine.ServeHTTP(httptest.NewRecorder(), req)

	if appInfo != "app-info" {
		t.Errorf("AppInfo = %v, want it added to the request context", appInfo)
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
