package gin_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/exgamer/gosdk-core/pkg/errorreporter"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	ginhelper "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/exgamer/gosdk-http-core/pkg/middleware"
	"github.com/gin-gonic/gin"
)

type capturedEvent struct {
	err  error
	opts errorreporter.Options
}

type fakeReporter struct {
	mu     sync.Mutex
	events []capturedEvent
}

func (f *fakeReporter) Capture(_ context.Context, err error, opts errorreporter.Options) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, capturedEvent{err: err, opts: opts})
}

func (f *fakeReporter) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

func (f *fakeReporter) last() capturedEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.events[len(f.events)-1]
}

func withFakeReporter(t *testing.T) *fakeReporter {
	t.Helper()
	f := &fakeReporter{}
	errorreporter.SetReporter(f)
	return f
}

// TestHttp_TrackedException_ReachesReporter — обычная ошибка (SentryMiddleware путь):
// хендлер выставляет "exception" в контекст, middleware должен вызвать errorreporter.Capture.
func TestHttp_TrackedException_ReachesReporter(t *testing.T) {
	f := withFakeReporter(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.SentryMiddleware())
	router.GET("/boom", func(c *gin.Context) {
		ex := exception.NewInternalServerErrorException(errors.New("db is down"), map[string]any{"table": "cities"})
		c.Set("exception", ex)
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/boom?token=secret&city=1", nil)
	req.Header.Set("Authorization", "Bearer supersecret")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if got := f.count(); got != 1 {
		t.Fatalf("expected exactly 1 captured event, got %d", got)
	}

	ev := f.last()
	if ev.opts.Level != errorreporter.LevelError {
		t.Fatalf("expected LevelError for 500, got %v", ev.opts.Level)
	}

	extra, ok := ev.opts.Extra["header"].(map[string]any)
	if !ok {
		t.Fatal("expected 'header' extra to be present as map[string]any")
	}
	if extra["header_Authorization"] != "*****" {
		t.Fatalf("expected Authorization header to be masked, got %v", extra["header_Authorization"])
	}

	query, ok := ev.opts.Extra["query"].(map[string]any)
	if !ok {
		t.Fatal("expected 'query' extra to be present")
	}
	if query["query_token"] != "*****" {
		t.Fatalf("expected token query param to be masked, got %v", query["query_token"])
	}

	errData, ok := ev.opts.Extra["error"].(map[string]any)
	if !ok {
		t.Fatal("expected 'error' extra to be present")
	}
	if errData["status"] != http.StatusInternalServerError {
		t.Fatalf("expected status 500 in error context, got %v", errData["status"])
	}
}

// TestHttp_4xx_MapsToWarningLevel — 4xx должен маппиться в LevelWarning, не Error.
func TestHttp_4xx_MapsToWarningLevel(t *testing.T) {
	f := withFakeReporter(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.SentryMiddleware())
	router.GET("/bad", func(c *gin.Context) {
		ex := exception.NewHttpException(http.StatusUnprocessableEntity, errors.New("validation failed"), nil)
		c.Set("exception", ex)
		c.Status(http.StatusUnprocessableEntity)
	})

	req := httptest.NewRequest(http.MethodGet, "/bad", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if got := f.count(); got != 1 {
		t.Fatalf("expected exactly 1 captured event, got %d", got)
	}
	if f.last().opts.Level != errorreporter.LevelWarning {
		t.Fatalf("expected LevelWarning for 422, got %v", f.last().opts.Level)
	}
}

// TestHttp_UntrackedException_DoesNotReachReporter — TrackInSentry=false должен полностью
// подавить отправку.
func TestHttp_UntrackedException_DoesNotReachReporter(t *testing.T) {
	f := withFakeReporter(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.SentryMiddleware())
	router.GET("/silent", func(c *gin.Context) {
		ex := exception.NewUntrackableHttpException(http.StatusNotFound, errors.New("not found"), nil)
		c.Set("exception", ex)
		c.Status(http.StatusNotFound)
	})

	req := httptest.NewRequest(http.MethodGet, "/silent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if got := f.count(); got != 0 {
		t.Fatalf("expected 0 captured events for untracked exception, got %d", got)
	}
}

// TestHttp_Panic_ReachesReporter — паника в хендлере должна долететь до reporter-а
// через тот же путь, что и в бою: gin.CustomRecovery(ErrorHandler).
func TestHttp_Panic_ReachesReporter(t *testing.T) {
	f := withFakeReporter(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(gin.CustomRecovery(ginhelper.ErrorHandler))
	router.GET("/panic", func(c *gin.Context) {
		panic("nil pointer somewhere")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 response after recovered panic, got %d", w.Code)
	}
	if got := f.count(); got != 1 {
		t.Fatalf("expected exactly 1 captured event from panic, got %d", got)
	}
	if f.last().opts.Level != errorreporter.LevelError {
		t.Fatalf("expected LevelError for panic, got %v", f.last().opts.Level)
	}
}

// TestHttp_AlreadyReportedError_DedupedAtTransport — ошибка, уже отрепорченная в
// "domain"-слое через errorreporter.CaptureError, не должна отправиться повторно,
// когда долетает до HTTP-транспорта. Это ключевая гарантия "единого механизма".
func TestHttp_AlreadyReportedError_DedupedAtTransport(t *testing.T) {
	f := withFakeReporter(t)
	gin.SetMode(gin.TestMode)

	baseErr := errors.New("upstream timeout")
	// имитируем то, что уже произошло в domain-слое до транспорта
	reportedErr := errorreporter.CaptureError(context.Background(), baseErr, map[string]string{"component": "city_service"})
	if f.count() != 1 {
		t.Fatalf("expected 1 event after domain-level CaptureError, got %d", f.count())
	}

	router := gin.New()
	router.Use(middleware.SentryMiddleware())
	router.GET("/dup", func(c *gin.Context) {
		ex := exception.NewInternalServerErrorException(reportedErr, nil)
		c.Set("exception", ex)
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/dup", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if got := f.count(); got != 1 {
		t.Fatalf("expected dedup to keep count at 1, got %d", got)
	}
}
