package response

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/exgamer/gosdk-http-core/pkg/exception"
	"github.com/gin-gonic/gin"
)

func formatted(handler gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Next()
		Formatted(c)
	})
	r.GET("/x", handler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	return w
}

// A handler that wrote its own answer (file, text) keeps it: no JSON wrapper appended.
func TestFormatted_HandlerWroteItself(t *testing.T) {
	w := formatted(func(c *gin.Context) { c.String(http.StatusOK, "plain file content") })

	if w.Body.String() != "plain file content" {
		t.Fatalf("body = %q, want only the handler's text", w.Body.String())
	}
}

// NoContent: an empty answer with the given status (204 by default), no JSON body.
func TestFormatted_NoContent(t *testing.T) {
	for status, want := range map[int]int{0: http.StatusNoContent, http.StatusOK: http.StatusOK} {
		w := formatted(func(c *gin.Context) { NoContent(c, status) })

		if w.Code != want || w.Body.Len() != 0 {
			t.Fatalf("NoContent(%d) = %d %q, want %d without body", status, w.Code, w.Body.String(), want)
		}
	}
}

// Unchanged: Success data goes out as the JSON wrapper.
func TestFormatted_SuccessUnchanged(t *testing.T) {
	w := formatted(func(c *gin.Context) { Success(c, map[string]int{"a": 1}) })

	if w.Code != http.StatusOK || w.Body.String() != `{"success":true,"data":{"a":1}}` {
		t.Fatalf("Success = %d %q", w.Code, w.Body.String())
	}
}

// Bare c.Abort() without a response — a service error: 500 error body, not 200 success.
func TestFormatted_BareAbortIs500(t *testing.T) {
	w := formatted(func(c *gin.Context) { c.Abort() })

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), `"status":500`) ||
		strings.Contains(w.Body.String(), `"success":true`) {
		t.Fatalf("bare Abort = %d %q, want 500 error body", w.Code, w.Body.String())
	}
}

// AbortWithStatus(4xx) without a response — the SDK error body with that status, not success:true.
func TestFormatted_AbortWithErrorStatus(t *testing.T) {
	w := formatted(func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) })

	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), `"status":401`) ||
		strings.Contains(w.Body.String(), `"success":true`) {
		t.Fatalf("AbortWithStatus(401) = %d %q, want 401 error body", w.Code, w.Body.String())
	}
}

// AbortWithStatus(<400) — the middleware answered itself (CORS preflight 204): left as is, no body.
func TestFormatted_AbortWithStatusBelow400Unchanged(t *testing.T) {
	w := formatted(func(c *gin.Context) { c.AbortWithStatus(http.StatusNoContent) })

	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Fatalf("AbortWithStatus(204) = %d %q, want 204 without body", w.Code, w.Body.String())
	}
}

// Unchanged: ErrorResponse aborts with its exception — its status and message.
func TestFormatted_ErrorResponseUnchanged(t *testing.T) {
	w := formatted(func(c *gin.Context) {
		ErrorResponse(c, exception.NewUntrackableHttpException(http.StatusForbidden, errors.New("no access"), nil))
	})

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), `"message":"no access"`) {
		t.Fatalf("ErrorResponse = %d %q", w.Code, w.Body.String())
	}
}

// Unchanged: a response set before Abort (Success, NoContent) is written as before.
func TestFormatted_AnswerThenAbortUnchanged(t *testing.T) {
	w := formatted(func(c *gin.Context) {
		Success(c, map[string]int{"a": 1})
		c.Abort()
	})
	if w.Code != http.StatusOK || w.Body.String() != `{"success":true,"data":{"a":1}}` {
		t.Fatalf("Success+Abort = %d %q", w.Code, w.Body.String())
	}

	w = formatted(func(c *gin.Context) {
		NoContent(c, 0)
		c.Abort()
	})
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Fatalf("NoContent+Abort = %d %q", w.Code, w.Body.String())
	}
}
