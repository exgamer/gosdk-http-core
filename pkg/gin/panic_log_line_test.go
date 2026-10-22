package gin

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/config"
)

const forgedLogLine = "2026-10-22 00:00:00.000 [INFO] fake"

// A panic text with \n and the stack stay one physical line with literal \n; no forged [INFO] line.
func TestInitRouter_PanicRecordIsOneLine(t *testing.T) {
	buf := servePanicLogLine(t, "/p", "x\n"+forgedLogLine)

	lines := panicLogLines(buf)
	if len(lines) != 1 {
		t.Fatalf("want one log line, got %d:\n%s", len(lines), buf.String())
	}

	line := lines[0]
	if !strings.Contains(line, "[ERROR] [") || !strings.Contains(line, `,GET,/p,500] panic: x\n`+forgedLogLine+`\n`) {
		t.Errorf("log line = %q, want the panic text with a literal \\n and the stack after a literal \\n", line)
	}

	if n := strings.Count(buf.String(), "[INFO] fake"); n != 1 {
		t.Errorf(`"[INFO] fake" occurs %d times, want once inside the [ERROR] line`, n)
	}
}

// \r and ESC in the panic text and in the path reach the log as literals.
func TestInitRouter_PanicRecordEscapesControl(t *testing.T) {
	buf := servePanicLogLine(t, "/y%0D", "a\rb\x1b[31mc")
	out := buf.String()

	if strings.ContainsAny(out, "\r\x1b") {
		t.Errorf("log = %q, want no raw \\r and ESC bytes", out)
	}

	for _, want := range []string{`,GET,/y\r,500] `, `panic: a\rb\x1b[31mc\n`} {
		if !strings.Contains(out, want) {
			t.Errorf("log = %q, want %q", out, want)
		}
	}
}

// servePanicLogLine serves target through InitRouter with HttpInfo in the context, every request panics
// with value; returns the log of the request only.
func servePanicLogLine(t *testing.T, target, value string) *bytes.Buffer {
	t.Helper()

	withPanicReporter(t)
	buf := captureLog(t)

	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{})
	r.Use(withPanicHTTPInfo(), func(*gin.Context) { panic(value) })
	r.GET("/p", func(c *gin.Context) { c.Status(http.StatusOK) })

	buf.Reset() // startup log of InitRouter
	serve(r, http.MethodGet, target)

	return buf
}

func panicLogLines(buf *bytes.Buffer) []string {
	out := strings.TrimSpace(buf.String())
	if out == "" {
		return nil
	}

	return strings.Split(out, "\n")
}
