package gin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/config"
)

// ClientIP берётся из X-Forwarded-For только от частных сетей, loopback и 100.64.0.0/10; от публичного адреса —
// адрес соединения.
func TestInitRouter_TrustedProxiesPrivateOnly(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{MetricsEnabled: "false"})
	r.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

	cases := []struct {
		remote string
		want   string
	}{
		{remote: "203.0.113.5:1234", want: "203.0.113.5"},
		{remote: "10.1.2.3:1234", want: "198.51.100.7"},
		{remote: "127.0.0.1:1234", want: "198.51.100.7"},
		{remote: "[::1]:1234", want: "198.51.100.7"},
		{remote: "100.64.0.9:1234", want: "198.51.100.7"},
		{remote: "100.127.255.1:1234", want: "198.51.100.7"},
		{remote: "100.128.0.1:1234", want: "100.128.0.1"},
		{remote: "[fd00::1]:1234", want: "198.51.100.7"},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/ip", nil)
		req.RemoteAddr = tc.remote
		req.Header.Set("X-Forwarded-For", "198.51.100.7")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if got := w.Body.String(); got != tc.want {
			t.Errorf("remote %s: ClientIP = %q, want %q", tc.remote, got, tc.want)
		}
	}
}

// Каждый ответ роутера, включая 404, — с X-Content-Type-Options: nosniff.
func TestInitRouter_NoSniff(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{MetricsEnabled: "false", HandlerTimeout: 5})
	r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	for _, path := range []string{"/ok", "/missing"} {
		if got := serve(r, http.MethodGet, path).Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("GET %s: X-Content-Type-Options = %q, want nosniff", path, got)
		}
	}
}

// Метка url метрик — путь только при явном METRICS_URL_LABEL=path.
func TestMetricsPathLabel(t *testing.T) {
	cases := map[string]bool{"": false, "route": false, " Route ": false, "path": true, "PATH": true, "url": false}

	for value, want := range cases {
		if got := metricsPathLabel(value); got != want {
			t.Errorf("metricsPathLabel(%q) = %v, want %v", value, got, want)
		}
	}
}

// Ответ таймаута — тело ошибки SDK с входящим Request-Id и именем сервиса.
func TestInitRouter_TimeoutAnswerSdkFormat(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{MetricsEnabled: "false", HandlerTimeout: 1})
	r.GET("/slow", func(c *gin.Context) {
		<-c.Request.Context().Done()
		time.Sleep(10 * time.Millisecond)
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	req.Header.Set("Request-Id", "req-abc-123")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var got struct {
		Success bool `json:"success"`
		Data    struct {
			Status        int    `json:"status"`
			CorrelationID string `json:"correlation_id"`
			Hostname      string `json:"hostname"`
		} `json:"data"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}

	if w.Code != http.StatusServiceUnavailable || got.Success || got.Data.Status != http.StatusServiceUnavailable ||
		got.Data.CorrelationID != "req-abc-123" || got.Data.Hostname != "svc" {
		t.Fatalf("timeout answer = %d %s", w.Code, w.Body.String())
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q", w.Header().Get("Content-Type"))
	}
}

// ErrorDetailsEnabled — только при HTTP_ERROR_DETAILS=true у роутера InitRouter.
func TestErrorDetailsEnabled(t *testing.T) {
	for setting, want := range map[string]bool{"": false, "false": false, "true": true} {
		r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{MetricsEnabled: "false", ErrorDetails: setting})

		var got bool

		r.GET("/d", func(c *gin.Context) { got = ErrorDetailsEnabled(c) })
		serve(r, http.MethodGet, "/d")

		if got != want {
			t.Errorf("HTTP_ERROR_DETAILS=%q: ErrorDetailsEnabled = %v, want %v", setting, got, want)
		}
	}
}

// Iin — чувствительный ключ; MaskJSONKeys — на любой глубине, исходное значение не меняется.
func TestMaskJSONKeys(t *testing.T) {
	if !IsSensitiveKey("Iin") {
		t.Fatal("Iin must be sensitive")
	}

	src := map[string]any{"a": []any{map[string]any{"password": "p", "x": "1"}}, "iin": "123", "n": "v"}
	out := MaskJSONKeys(src, "***").(map[string]any)

	inner := out["a"].([]any)[0].(map[string]any)
	if inner["password"] != "***" || inner["x"] != "1" || out["iin"] != "***" || out["n"] != "v" {
		t.Fatalf("masked = %v", out)
	}

	if src["iin"] != "123" || src["a"].([]any)[0].(map[string]any)["password"] != "p" {
		t.Fatalf("source changed: %v", src)
	}
}

func TestMaskBodyString(t *testing.T) {
	cases := []struct {
		in       string
		hidden   string
		contains string
	}{
		{in: `{"client_secret":"S1","grant_type":"x"}`, hidden: "S1", contains: `"grant_type":"x"`},
		{in: `client_secret=S2&grant_type=x`, hidden: "S2", contains: "client_secret=***"},
		{in: `plain text`, contains: "plain text"},
	}

	for _, tc := range cases {
		got := MaskBodyString(tc.in, "***")
		if (tc.hidden != "" && strings.Contains(got, tc.hidden)) || !strings.Contains(got, tc.contains) {
			t.Errorf("MaskBodyString(%q) = %q", tc.in, got)
		}
	}
}
