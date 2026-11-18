package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/config"
)

const corsTestOrigin = "https://app.example.com"

func corsTestRouter(cfg *config.HttpConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(corsHandler(cfg))
	r.Any("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	return r
}

func corsTestDo(r *gin.Engine, method, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/x", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	return w
}

// Без CORS_EXTRA_ALLOWED_HEADERS заголовок ответа тот же, что до v1.6.0.
func TestCors_NoExtraHeaders_SameAllowHeadersAsBefore(t *testing.T) {
	r := corsTestRouter(&config.HttpConfig{CorsAllowedOrigins: corsTestOrigin})

	w := corsTestDo(r, http.MethodOptions, corsTestOrigin)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}

	want := "Origin, Content-Type, Accept, Authorization, X-Correlation-Id, X-Request-Id, Request-Id"
	if got := w.Header().Get("Access-Control-Allow-Headers"); got != want {
		t.Errorf("Allow-Headers = %q, want %q", got, want)
	}
}

func TestCors_ExtraHeadersAppended(t *testing.T) {
	r := corsTestRouter(&config.HttpConfig{
		CorsAllowedOrigins:      corsTestOrigin,
		CorsExtraAllowedHeaders: "X-CSRF-Token",
	})

	w := corsTestDo(r, http.MethodOptions, corsTestOrigin)

	want := "Origin, Content-Type, Accept, Authorization, X-Correlation-Id, X-Request-Id, Request-Id, X-CSRF-Token"
	if got := w.Header().Get("Access-Control-Allow-Headers"); got != want {
		t.Errorf("Allow-Headers = %q, want %q", got, want)
	}
}

func TestCors_ExtraHeadersOnlyForAllowedOrigin(t *testing.T) {
	r := corsTestRouter(&config.HttpConfig{
		CorsAllowedOrigins:      corsTestOrigin,
		CorsExtraAllowedHeaders: "X-CSRF-Token",
	})

	for _, origin := range []string{"https://evil.example.com", ""} {
		w := corsTestDo(r, http.MethodGet, origin)

		if got := w.Header().Get("Access-Control-Allow-Headers"); got != "" {
			t.Errorf("origin %q: Allow-Headers = %q, want empty", origin, got)
		}

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("origin %q: Allow-Origin = %q, want empty", origin, got)
		}
	}
}

func TestCors_ExtraHeadersIgnoredWhenOriginsEmpty(t *testing.T) {
	r := corsTestRouter(&config.HttpConfig{CorsExtraAllowedHeaders: "X-CSRF-Token"})

	w := corsTestDo(r, http.MethodGet, corsTestOrigin)

	if got := w.Header().Get("Access-Control-Allow-Headers"); got != "" {
		t.Errorf("Allow-Headers = %q, want empty", got)
	}
}

func TestCorsAllowedHeaders(t *testing.T) {
	base := "Origin, Content-Type, Accept, Authorization, X-Correlation-Id, X-Request-Id, Request-Id"

	cases := []struct {
		name  string
		extra string
		want  string
	}{
		{"empty", "", base},
		{"spaces only", "  ,  , ", base},
		{"one", "X-CSRF-Token", base + ", X-CSRF-Token"},
		{"several with spaces", " X-CSRF-Token , X-Client-Version ", base + ", X-CSRF-Token, X-Client-Version"},
		{"duplicate of base, other case", "authorization, x-request-id", base},
		{"duplicate within extra", "X-CSRF-Token,x-csrf-token", base + ", X-CSRF-Token"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := corsAllowedHeaders(tc.extra); got != tc.want {
				t.Errorf("corsAllowedHeaders(%q) = %q, want %q", tc.extra, got, tc.want)
			}
		})
	}
}

// Явный список: origin отражается с credentials, как раньше.
func TestCors_ExplicitList_CredentialsAsBefore(t *testing.T) {
	r := corsTestRouter(&config.HttpConfig{CorsAllowedOrigins: "https://other.example.com, " + corsTestOrigin})

	w := corsTestDo(r, http.MethodGet, corsTestOrigin)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != corsTestOrigin {
		t.Errorf("Allow-Origin = %q, want %q", got, corsTestOrigin)
	}

	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q, want true", got)
	}

	if got := w.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin", got)
	}
}

// "*": origin отражается, Allow-Credentials нет, остальные заголовки прежние.
func TestCors_AnyOrigin_NoCredentials(t *testing.T) {
	r := corsTestRouter(&config.HttpConfig{CorsAllowedOrigins: "*"})

	w := corsTestDo(r, http.MethodOptions, corsTestOrigin)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != corsTestOrigin {
		t.Errorf("Allow-Origin = %q, want %q", got, corsTestOrigin)
	}

	if got := w.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin", got)
	}

	if got, ok := w.Header()["Access-Control-Allow-Credentials"]; ok {
		t.Errorf("Allow-Credentials = %q, want absent", got)
	}

	if got := w.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, PATCH, DELETE, OPTIONS" {
		t.Errorf("Allow-Methods = %q", got)
	}

	if got := w.Header().Get("Access-Control-Max-Age"); got != "86400" {
		t.Errorf("Max-Age = %q", got)
	}
}

// Origin "null" с credentials не разрешается никогда; при "*" без credentials — отражается.
func TestCors_NullOrigin(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.HttpConfig
		want string
	}{
		{"any origin without credentials", &config.HttpConfig{CorsAllowedOrigins: "*"}, "null"},
		{"explicit list with null", &config.HttpConfig{CorsAllowedOrigins: corsTestOrigin + ",null"}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := corsTestDo(corsTestRouter(tc.cfg), http.MethodGet, "null")

			if got := w.Header().Get("Access-Control-Allow-Origin"); got != tc.want {
				t.Errorf("Allow-Origin = %q, want %q", got, tc.want)
			}

			if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
				t.Errorf("Allow-Credentials = %q, want absent", got)
			}
		})
	}
}
