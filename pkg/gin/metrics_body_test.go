package gin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/config"
)

// countingBody Тело запроса, которое считает прочитанные байты
type countingBody struct {
	r    io.Reader
	read atomic.Int64
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.read.Add(int64(n))

	return n, err
}

func (b *countingBody) Close() error { return nil }

// The metrics middleware does not read the request body: the handler gets it untouched, so body limits and
// deadlines of the service apply before the body is read.
func TestInitRouter_MetricsDoNotReadRequestBody(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{})

	body := &countingBody{r: strings.NewReader(strings.Repeat("x", 4096))}

	var readBeforeHandler int64 = -1

	var got string

	r.POST("/echo", func(c *gin.Context) {
		readBeforeHandler = body.read.Load()

		data, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Errorf("read body in handler: %v", err)
		}

		got = string(data)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/echo", nil)
	req.Body = body
	req.ContentLength = 4096

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	if readBeforeHandler != 0 {
		t.Fatalf("bytes read before the handler = %d, want 0", readBeforeHandler)
	}

	if len(got) != 4096 {
		t.Fatalf("handler read %d bytes, want 4096", len(got))
	}
}
