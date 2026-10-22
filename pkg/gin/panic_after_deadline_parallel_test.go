package gin

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/config"
)

// Parallel panics after the deadline: each client gets the timeout answer, one event and one 503 record per panic.
// Run with -race.
func TestInitRouter_ParallelPanicsAfterDeadline(t *testing.T) {
	const requests = 10

	reporter := withPanicReporter(t)
	buf := captureLog(t)

	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 1})
	r.GET("/panic", func(c *gin.Context) {
		waitDone(c.Request.Context(), 3*time.Second)
		panic(panicSecret)
	})

	results := make([]*httptest.ResponseRecorder, requests)

	var wg sync.WaitGroup
	for i := range requests {
		wg.Go(func() {
			results[i] = serve(r, http.MethodGet, "/panic")
		})
	}
	wg.Wait()

	for i, w := range results {
		assertTimeoutAnswer(t, w, "request "+strconv.Itoa(i))
	}

	if n := len(reporter.all()); n != requests {
		t.Errorf("events = %d, want %d", n, requests)
	}

	if n := strings.Count(buf.String(), "[ERROR] [,GET,/panic,503] panic: "); n != requests {
		t.Errorf("panic records with status 503 = %d, want %d; log = %q", n, requests, buf.String())
	}
}
