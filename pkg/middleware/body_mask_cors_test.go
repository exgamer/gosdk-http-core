package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corelogger "github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/gin-gonic/gin"
)

// Debug-лог тела: ключи JSON на любой глубине и параметры формы маскируются, заголовок Iin — тоже.
func TestLoggerMiddleware_DebugMasksBodyAndIin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	out := captureLog(t, corelogger.LevelDebug)

	serveBody(t, []byte(`{"user":{"password":"PW1","iin":"880101300123","name":"Ann"},"items":[{"client_secret":"CS1"}]}`), "",
		http.Header{"Iin": {"990101300456"}, "Content-Type": {"application/json"}})

	serveBody(t, []byte(`grant_type=x&client_secret=CS2`), "",
		http.Header{"Content-Type": {"application/x-www-form-urlencoded"}})

	log := out.String()
	for _, secret := range []string{"PW1", "880101300123", "CS1", "990101300456", "CS2"} {
		if strings.Contains(log, secret) {
			t.Fatalf("secret %q in debug log: %s", secret, log)
		}
	}

	if !strings.Contains(log, "Ann") || !strings.Contains(log, "grant_type=x") {
		t.Fatalf("ordinary fields must stay: %s", log)
	}
}

// JSON, обрезанный по пределу лога или битый, и multipart в debug-лог сырым текстом не попадают.
func TestLoggerMiddleware_DebugUnparsedBodyNotRaw(t *testing.T) {
	gin.SetMode(gin.TestMode)

	out := captureLog(t, corelogger.LevelDebug)

	big := append([]byte(`{"password":"PW2","pad":"`), bytes.Repeat([]byte("a"), logBodyLimit)...)
	serveBody(t, append(big, []byte(`"}`)...), "", http.Header{"Content-Type": {"application/json"}})
	serveBody(t, []byte("--b\r\nContent-Disposition: form-data; name=\"password\"\r\n\r\nPW3\r\n--b--\r\n"), "",
		http.Header{"Content-Type": {"multipart/form-data; boundary=b"}})

	log := out.String()
	if strings.Contains(log, "PW2") || strings.Contains(log, "PW3") {
		t.Fatalf("raw body in debug log: %.300s", log)
	}

	if !strings.Contains(log, "<unparsed JSON body") || !strings.Contains(log, "<multipart body") {
		t.Fatalf("want size placeholders: %.300s", log)
	}
}

// CORS: Vary не затирает выставленное раньше значение, Request-Id разрешён и доступен скрипту.
func TestCors_VaryAddedAndRequestIdExposed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Add("Vary", "Accept-Encoding")
		c.Next()
	})
	r.Use(corsHandler(&config.HttpConfig{CorsAllowedOrigins: corsTestOrigin}))
	r.Any("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := corsTestDo(r, http.MethodGet, corsTestOrigin)

	vary := strings.Join(w.Header().Values("Vary"), ",")
	if !strings.Contains(vary, "Accept-Encoding") || !strings.Contains(vary, "Origin") {
		t.Fatalf("Vary = %q, want Accept-Encoding and Origin", vary)
	}

	if got := w.Header().Get("Access-Control-Expose-Headers"); got != "X-Correlation-Id" {
		t.Fatalf("Expose-Headers = %q, want X-Correlation-Id", got)
	}

	if got := w.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, ", X-Correlation-Id") || !strings.Contains(got, ", Request-Id") {
		t.Fatalf("Allow-Headers = %q, want X-Correlation-Id and Request-Id", got)
	}
}

// Повторный Vary: Origin не дублируется.
func TestAddVary_NoDuplicate(t *testing.T) {
	h := http.Header{"Vary": {"Accept-Encoding, origin"}}
	addVary(h, "Origin")

	if got := h.Values("Vary"); len(got) != 1 {
		t.Fatalf("Vary = %v, want unchanged", got)
	}

	rec := httptest.NewRecorder()
	addVary(rec.Header(), "Origin")

	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary = %q, want Origin", got)
	}
}
