package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corelogger "github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/gin-gonic/gin"
)

// serveBody POST /test с телом body; handler получает прочитанное обработчиком тело.
func serveBody(t *testing.T, body []byte, query string, header http.Header) (got []byte) {
	t.Helper()

	engine := gin.New()
	engine.Use(RequestInfoMiddleware(nil), LoggerMiddleware(), FormattedResponseMiddleware())
	engine.POST("/test", func(c *gin.Context) {
		got, _ = io.ReadAll(c.Request.Body)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test"+query, bytes.NewReader(body))
	for k, v := range header {
		req.Header[k] = v
	}

	engine.ServeHTTP(httptest.NewRecorder(), req)

	return got
}

// Тело больше 1 МиБ доходит до обработчика целиком: и без debug (тело не читается), и в debug (в лог — первые 1 МиБ).
func TestLoggerMiddleware_FullBodyReachesHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := bytes.Repeat([]byte("a"), 3<<20)

	for _, level := range []corelogger.Level{corelogger.LevelInfo, corelogger.LevelDebug} {
		t.Run(level.String(), func(t *testing.T) {
			captureLog(t, level)

			if got := serveBody(t, body, "", nil); len(got) != len(body) {
				t.Fatalf("handler got %d bytes, want %d", len(got), len(body))
			}
		})
	}
}

// Debug-лог маскирует чувствительные заголовки и параметры query (code, state, *token*, X-Api-Key, SENSITIVE_KEYS).
func TestLoggerMiddleware_DebugMasksSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SENSITIVE_KEYS", "otp")

	out := captureLog(t, corelogger.LevelDebug)

	serveBody(t, []byte(`{}`), "?code=C0DE&state=ST4TE&refresh_token=RT&otp=1234&page=7",
		http.Header{"X-Api-Key": {"AK"}, "Authorization": {"Bearer B"}, "Accept": {"application/json"}})

	log := out.String()
	for _, secret := range []string{"C0DE", "ST4TE", "RT", "1234", "AK", "Bearer B"} {
		if strings.Contains(log, secret) {
			t.Fatalf("secret %q in debug log: %s", secret, log)
		}
	}

	if !strings.Contains(log, `"page":["7"]`) || !strings.Contains(log, "application/json") {
		t.Fatalf("ordinary query/header must stay: %s", log)
	}
}
