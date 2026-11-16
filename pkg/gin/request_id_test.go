package gin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/exgamer/gosdk-http-core/pkg/constants"
	"github.com/gin-gonic/gin"
)

func requestIdOf(t *testing.T, header string, set bool) (string, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	if set {
		c.Request.Header.Set(constants.RequestIdHeaderName, header)
	}

	info := GetInstanceHttpInfo(c)

	return info.RequestId, c.Request.Header.Get(constants.RequestIdHeaderName)
}

// A well-formed incoming Request-Id (UUID, ULID, hex trace id, base64) is kept as is.
func TestRequestId_ValidKept(t *testing.T) {
	for _, id := range []string{
		"2f1c7a0e-8f6b-4c1d-9a3e-5b2d4c6e8f10",
		"01HZY3N8Q4K5V6W7X8Y9Z0ABCD",
		"4bf92f3577b34da6a3ce929d0e0e4736",
		"YWJjZA==",
		"svc.a:42/x+y-z_1",
		strings.Repeat("a", 128),
	} {
		got, hdr := requestIdOf(t, id, true)
		if got != id || hdr != id {
			t.Errorf("Request-Id %q → %q (header %q), want kept", id, got, hdr)
		}
	}
}

// Missing, empty or malformed (spaces, quotes, control characters, over 128) — a generated id replaces it,
// also in the request header passed on to outgoing calls.
func TestRequestId_InvalidReplaced(t *testing.T) {
	cases := map[string]string{
		"empty":   "",
		"space":   "abc def",
		"quote":   `abc"}`,
		"newline": "abc\nforged",
		"tab":     "abc\tx",
		"unicode": "идентификатор",
		"long":    strings.Repeat("a", 129),
	}
	for name, id := range cases {
		got, hdr := requestIdOf(t, id, true)
		if got == id || !IsValidRequestId(got) || hdr != got {
			t.Errorf("%s: Request-Id %q → %q (header %q), want a generated valid id in both", name, id, got, hdr)
		}
	}

	got, hdr := requestIdOf(t, "", false)
	if !IsValidRequestId(got) || hdr != got {
		t.Errorf("no header → %q (header %q), want a generated id", got, hdr)
	}
}
