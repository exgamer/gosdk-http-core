package response

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/exgamer/gosdk-core/pkg/debug"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

const internalSecret = "dial tcp db.internal:5432: user app_rw"

// 5xx без HTTP_ERROR_DETAILS: текст статуса без details, внутренняя ошибка клиенту не уходит.
func TestFormatted_ServerErrorHidesInternalText(t *testing.T) {
	handlers := map[string]gin.HandlerFunc{
		"InternalServerError": func(c *gin.Context) {
			InternalServerError(c, errors.New(internalSecret), map[string]any{"dsn": internalSecret})
		},
		"ErrorResponse plain": func(c *gin.Context) { ErrorResponse(c, errors.New(internalSecret)) },
		"503": func(c *gin.Context) {
			ErrorResponseWithStatus(c, http.StatusServiceUnavailable, errors.New(internalSecret), nil)
		},
		"raw": func(c *gin.Context) { InternalServerErrorRaw(c, errors.New(internalSecret), nil) },
	}

	for name, h := range handlers {
		w := formatted(h)
		body := w.Body.String()

		if w.Code < http.StatusInternalServerError || strings.Contains(body, "db.internal") || !strings.Contains(body, `"message":"`+http.StatusText(w.Code)+`"`) ||
			!strings.Contains(body, `"details":null`) {
			t.Errorf("%s: %d %s, want status text without details", name, w.Code, body)
		}
	}
}

// 4xx — текст ошибки и details, как раньше.
func TestFormatted_ClientErrorKeepsText(t *testing.T) {
	w := formatted(func(c *gin.Context) { Conflict(c, errors.New("order already paid"), map[string]any{"order_id": 7}) })

	if !strings.Contains(w.Body.String(), `"message":"order already paid"`) || !strings.Contains(w.Body.String(), `"order_id":7`) {
		t.Fatalf("4xx body = %s", w.Body.String())
	}
}

// С HTTP_ERROR_DETAILS=true (роутер InitRouter) — текст ошибки и details в 5xx, как раньше.
func TestFormatted_ServerErrorWithErrorDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin2.InitRouter(&baseConfig.BaseConfig{Name: "svc"}, &config.HttpConfig{ErrorDetails: "true", MetricsEnabled: "false"})
	r.Use(func(c *gin.Context) {
		c.Next()
		Formatted(c)
	})
	r.GET("/x", func(c *gin.Context) {
		ErrorResponse(c, exception.NewInternalServerErrorException(errors.New(internalSecret), map[string]any{"k": "v"}))
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	if !strings.Contains(w.Body.String(), internalSecret) || !strings.Contains(w.Body.String(), `"k":"v"`) {
		t.Fatalf("with details body = %s", w.Body.String())
	}
}

// outgoingStatement Запись исходящего запроса, как у http-request-builder.
type outgoingStatement struct {
	Url      string            `json:"url"`
	Headers  map[string]string `json:"headers"`
	Body     string            `json:"body"`
	Response map[string]any    `json:"response"`
}

// Debug-поле ответа: секреты в заголовках, телах (JSON и форма) и ответах исходящих запросов — «***».
func TestFormatted_DebugMasksSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		dbg := debug.NewDebugCollector()
		dbg.AddStatement("http", time.Millisecond, []outgoingStatement{
			{Url: "https://idp/token", Headers: map[string]string{"Authorization": "Bearer AT1"}, Body: `{"client_secret":"CS1","scope":"read"}`,
				Response: map[string]any{"access_token": "AT2", "expires_in": 60}},
			{Url: "https://idp/token", Body: "grant_type=client_credentials&client_secret=CS2"},
		})
		c.Request = c.Request.WithContext(debug.WithDebugCollector(c.Request.Context(), dbg))
		c.Next()
		Formatted(c)
	})
	r.GET("/x", func(c *gin.Context) { Success(c, "ok") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	body := w.Body.String()
	for _, secret := range []string{"AT1", "CS1", "AT2", "CS2"} {
		if strings.Contains(body, secret) {
			t.Fatalf("secret %q in debug answer: %s", secret, body)
		}
	}

	if !strings.Contains(body, "scope") || !strings.Contains(body, "grant_type=client_credentials") || !strings.Contains(body, `"expires_in":60`) {
		t.Fatalf("ordinary debug data must stay: %s", body)
	}
}
