package gin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/constants"
)

// The timeout body is built only at the timeout, from the request's correlation id and the service name;
// Content-Length matches the body.
func TestHandlerTimeout_TimeoutBodyBuiltAtTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(handlerTimeout(20*time.Millisecond, "svc-a"))
	r.GET("/slow", func(c *gin.Context) {
		<-c.Request.Context().Done()
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	req.Header.Set(constants.CorrelationIdHeaderName, "corr-t")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}

	var got struct {
		Data struct {
			CorrelationID string `json:"correlation_id"`
			Hostname      string `json:"hostname"`
		} `json:"data"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}

	if got.Data.CorrelationID != "corr-t" || got.Data.Hostname != "svc-a" {
		t.Fatalf("correlation_id = %q, hostname = %q, want corr-t and svc-a", got.Data.CorrelationID, got.Data.Hostname)
	}

	if cl := w.Header().Get("Content-Length"); cl != strconv.Itoa(w.Body.Len()) {
		t.Fatalf("Content-Length = %q, body %d bytes", cl, w.Body.Len())
	}
}
