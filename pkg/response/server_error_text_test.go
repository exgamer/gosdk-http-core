package response

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/exgamer/gosdk-core/pkg/debug"
	exception2 "github.com/exgamer/gosdk-core/pkg/exception"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
)

const internalSecret = "dial tcp db.internal:5432: user app_rw"

const intendedText = "Сервис временно недоступен, повторите позже"

// 5xx без HTTP_ERROR_DETAILS: текст ошибки уходит клиенту, только если статус задан сервисом явно и ошибка — простое
// сообщение без вложенной причины; иначе — текст статуса. details — null в обоих случаях.
func TestFormatted_ServerErrorText(t *testing.T) {
	dbErr := errors.New(internalSecret)

	cases := []struct {
		name    string
		handler gin.HandlerFunc
		status  int
		message string // пусто — текст статуса
	}{
		{"явный 503, простое сообщение", func(c *gin.Context) {
			ErrorResponseUntrackableSentry(c, http.StatusServiceUnavailable, errors.New(intendedText), nil)
		}, http.StatusServiceUnavailable, intendedText},
		{"явный 503 Raw, простое сообщение", func(c *gin.Context) {
			ErrorResponseUntrackableSentryRaw(c, http.StatusServiceUnavailable, errors.New(intendedText), nil)
		}, http.StatusServiceUnavailable, intendedText},
		{"ErrorResponseWithStatus 503, простое сообщение, context не уходит", func(c *gin.Context) {
			ErrorResponseWithStatus(c, http.StatusServiceUnavailable, errors.New(intendedText), map[string]any{"dsn": internalSecret})
		}, http.StatusServiceUnavailable, intendedText},
		{"ErrorResponseWithStatusRaw 502, простое сообщение", func(c *gin.Context) {
			ErrorResponseWithStatusRaw(c, http.StatusBadGateway, errors.New(intendedText), nil)
		}, http.StatusBadGateway, intendedText},
		{"NewHttpException 503, простое сообщение", func(c *gin.Context) {
			ErrorResponse(c, exception.NewHttpException(http.StatusServiceUnavailable, errors.New(intendedText), nil))
		}, http.StatusServiceUnavailable, intendedText},
		{"NewUntrackableHttpException 503 Raw, простое сообщение", func(c *gin.Context) {
			ErrorResponseRaw(c, exception.NewUntrackableHttpException(http.StatusServiceUnavailable, errors.New(intendedText), nil))
		}, http.StatusServiceUnavailable, intendedText},
		{"явный 500, причина через %w", func(c *gin.Context) {
			ErrorResponseWithStatus(c, http.StatusInternalServerError, fmt.Errorf("x: %w", dbErr), nil)
		}, http.StatusInternalServerError, ""},
		{"явный 503 Raw, причина через %w", func(c *gin.Context) {
			ErrorResponseUntrackableSentryRaw(c, http.StatusServiceUnavailable, fmt.Errorf("x: %w", dbErr), nil)
		}, http.StatusServiceUnavailable, ""},
		{"явный 503, ошибка своего типа без причины (драйвер БД)", func(c *gin.Context) {
			ErrorResponseWithStatus(c, http.StatusServiceUnavailable, driverError{}, nil)
		}, http.StatusServiceUnavailable, ""},
		{"явный 503, errors.Join", func(c *gin.Context) {
			ErrorResponseWithStatus(c, http.StatusServiceUnavailable, errors.Join(errors.New("x"), dbErr), nil)
		}, http.StatusServiceUnavailable, ""},
		{"явный 503, AppException с причиной", func(c *gin.Context) {
			ErrorResponseWithStatus(c, http.StatusServiceUnavailable, exception2.NewAppException(fmt.Errorf("x: %w", dbErr), nil, false), nil)
		}, http.StatusServiceUnavailable, ""},
		{"ErrorResponse, простая ошибка → 500", func(c *gin.Context) { ErrorResponse(c, errors.New(internalSecret)) }, http.StatusInternalServerError, ""},
		{"ErrorResponseRaw, простая ошибка → 500", func(c *gin.Context) { ErrorResponseRaw(c, errors.New(internalSecret)) }, http.StatusInternalServerError, ""},
		{"ErrorResponse, AppException → 500 по виду", func(c *gin.Context) {
			ErrorResponse(c, exception2.NewAppException(errors.New(internalSecret), map[string]any{"dsn": internalSecret}, true))
		}, http.StatusInternalServerError, ""},
		{"InternalServerError, простая ошибка", func(c *gin.Context) {
			InternalServerError(c, errors.New("db host=db.internal user=app_rw"), map[string]any{"dsn": internalSecret})
		}, http.StatusInternalServerError, ""},
		{"InternalServerErrorRaw, простая ошибка", func(c *gin.Context) {
			InternalServerErrorRaw(c, errors.New("db host=db.internal user=app_rw"), nil)
		}, http.StatusInternalServerError, ""},
		{"NewInternalServerErrorException, простая ошибка", func(c *gin.Context) {
			ErrorResponse(c, exception.NewInternalServerErrorException(errors.New(internalSecret), nil))
		}, http.StatusInternalServerError, ""},
		{"HttpException литералом, простая ошибка", func(c *gin.Context) {
			ErrorResponse(c, &exception.HttpException{Code: http.StatusServiceUnavailable, Err: errors.New(internalSecret)})
		}, http.StatusServiceUnavailable, ""},
		{"явный 503 без ошибки", func(c *gin.Context) {
			ErrorResponseWithStatus(c, http.StatusServiceUnavailable, nil, nil)
		}, http.StatusServiceUnavailable, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := formatted(tc.handler)
			body := w.Body.String()

			want := tc.message
			if want == "" {
				want = http.StatusText(tc.status)
			}

			if w.Code != tc.status || strings.Contains(body, "db.internal") || !strings.Contains(body, `"message":"`+want+`"`) ||
				!strings.Contains(body, `"details":null`) {
				t.Errorf("%d %s, want %d message %q without details", w.Code, body, tc.status, want)
			}
		})
	}
}

// Паника — константный текст без текста паники и details, как раньше.
func TestFormatted_PanicHidesText(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin2.InitRouter(&baseConfig.BaseConfig{Name: "svc"}, &config.HttpConfig{MetricsEnabled: "false"})
	r.GET("/x", func(c *gin.Context) { panic(internalSecret) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "db.internal") {
		t.Fatalf("panic: %d %s", w.Code, w.Body.String())
	}
}

// С HTTP_ERROR_DETAILS=true — текст и details при явном статусе и причине через %w, как раньше.
func TestFormatted_ExplicitServerErrorWithErrorDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin2.InitRouter(&baseConfig.BaseConfig{Name: "svc"}, &config.HttpConfig{ErrorDetails: "true", MetricsEnabled: "false"})
	r.Use(func(c *gin.Context) {
		c.Next()
		Formatted(c)
	})
	r.GET("/x", func(c *gin.Context) {
		ErrorResponseWithStatusRaw(c, http.StatusServiceUnavailable, fmt.Errorf("x: %w", errors.New(internalSecret)), map[string]any{"k": "v"})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	if !strings.Contains(w.Body.String(), internalSecret) || !strings.Contains(w.Body.String(), `"k":"v"`) {
		t.Fatalf("with details body = %s", w.Body.String())
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

// driverError Ошибка своего типа без вложенной причины (как *pgconn.PgError): не публичный текст
type driverError struct{}

func (driverError) Error() string { return "FATAL: password authentication failed for user billing" }
