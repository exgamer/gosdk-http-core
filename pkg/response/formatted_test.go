package response

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
