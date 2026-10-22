package gin

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
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

const (
	wantTimeoutBody        = `{"code": -1, "msg":"http: Handler timeout"}`
	wantTimeoutContentType = "text/plain; charset=utf-8"
	wantStreamContentType  = "text/event-stream;charset=utf-8"
	fastRequests           = 20
)

// lateHandlerResult is what the timed-out handler saw in its gin.Context.
type lateHandlerResult struct {
	ctxErr     error
	paramAfter string
	keyAfter   any
}

// A handler still running after HANDLER_TIMEOUT keeps its own gin.Context: requests served meanwhile
// do not reuse it, the timeout answer keeps its contract. Run with -race.
func TestInitRouter_TimedOutHandlerKeepsItsContext(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 1})

	release := make(chan struct{})
	late := make(chan lateHandlerResult, 1)

	r.GET("/slow/:id", func(c *gin.Context) {
		c.Set("id", c.Param("id"))
		waitDone(c.Request.Context(), 3*time.Second)

		// Works with its context after the deadline, as a handler finishing a DB call does.
		c.Set("status", c.Writer.Status())
		c.Header("X-Late", "1")
		<-release

		key, _ := c.Get("id")
		late <- lateHandlerResult{ctxErr: c.Request.Context().Err(), paramAfter: c.Param("id"), keyAfter: key}
		c.JSON(http.StatusInternalServerError, gin.H{"late": true})
	})
	r.GET("/fast/:id", func(c *gin.Context) {
		c.Set("id", c.Param("id"))
		c.String(http.StatusOK, c.Param("id"))
	})

	slow := newSignalRecorder()
	served := make(chan struct{})

	go func() {
		defer close(served)
		r.ServeHTTP(slow, httptest.NewRequest(http.MethodGet, "/slow/1", nil))
	}()

	select {
	case <-slow.written:
	case <-time.After(3 * time.Second):
		t.Fatal("GET /slow/1: no timeout answer within 3s with HandlerTimeout 1s")
	}

	assertFastRequests(t, r)
	close(release)
	<-served

	got := <-late
	if !errors.Is(got.ctxErr, context.DeadlineExceeded) {
		t.Errorf("timed-out handler: ctx.Err() = %v, want context.DeadlineExceeded", got.ctxErr)
	}

	if got.paramAfter != "1" || got.keyAfter != "1" {
		t.Errorf("timed-out handler: Param(id) = %q, Get(id) = %v after other requests, want 1 and 1",
			got.paramAfter, got.keyAfter)
	}

	assertTimeoutAnswer(t, slow.ResponseRecorder, "GET /slow/1")
}

// Over the network the timeout answer comes at the deadline, not when the handler returns,
// and the client's next request does not wait for that handler.
func TestInitRouter_TimeoutAnswerReachesClientAtDeadline(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 1})
	r.GET("/slow", func(c *gin.Context) {
		time.Sleep(2 * time.Second) // ignores ctx, as a handler stuck in a call without ctx does
		c.String(http.StatusOK, "late")
	})
	r.GET("/fast", func(c *gin.Context) { c.String(http.StatusOK, "fast") })

	srv := httptest.NewServer(r)
	defer srv.Close()

	start := time.Now()
	code, body, header := get(t, srv.URL+"/slow")

	if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
		t.Errorf("GET /slow answered in %v, want about 1s (HandlerTimeout), not after the handler", elapsed)
	}

	if code != http.StatusServiceUnavailable || body != wantTimeoutBody || header.Get("Content-Type") != wantTimeoutContentType {
		t.Errorf("GET /slow = %d %q %q, want 503 %s %s", code, header.Get("Content-Type"), body, wantTimeoutContentType, wantTimeoutBody)
	}

	start = time.Now()
	if code, body, _ = get(t, srv.URL+"/fast"); code != http.StatusOK || body != "fast" {
		t.Errorf("GET /fast = %d %q, want 200 fast", code, body)
	}

	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("GET /fast after a timeout answered in %v, want no wait for the timed-out handler", elapsed)
	}
}

// A preflight answered with AbortWithStatus(204) keeps 204 when a response middleware writes a body after it,
// as gin does without HANDLER_TIMEOUT, and the rejected body is not logged as a write failure.
func TestInitRouter_NoContentStaysWithoutWarning(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 30})
	r.Use(func(c *gin.Context) {
		c.Next()
		c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":null}`))
	})
	r.OPTIONS("/preflight", func(c *gin.Context) { c.AbortWithStatus(http.StatusNoContent) })

	out := captureLog(t)
	w := serve(r, http.MethodOptions, "/preflight")

	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Errorf("OPTIONS /preflight = %d %q, want 204 without body", w.Code, w.Body.String())
	}

	if strings.Contains(out.String(), "handler timeout") {
		t.Errorf("OPTIONS /preflight: unexpected log %q", out.String())
	}
}

// A streamed response reaches the client event by event, with the handler's status and headers,
// before the handler returns.
func TestInitRouter_StreamReachesClientBeforeHandlerEnds(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 30})
	release := make(chan struct{})
	r.GET("/stream", func(c *gin.Context) {
		c.Status(http.StatusCreated)
		c.SSEvent("msg", "first")
		c.Writer.Flush()
		<-release
		c.SSEvent("msg", "second")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) }) // srv.Close waits for the handler

	resp, body := openStream(t, srv.URL+"/stream")
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusCreated || resp.Header.Get("Content-Type") != wantStreamContentType {
		t.Errorf("GET /stream = %d %q, want 201 %s", resp.StatusCode, resp.Header.Get("Content-Type"), wantStreamContentType)
	}

	if line := readLine(t, body, "data:"); line != "data:first" {
		t.Errorf("GET /stream: first event %q while the handler runs, want data:first", line)
	}

	releaseOnce.Do(func() { close(release) })

	if line := readLine(t, body, "data:"); line != "data:second" {
		t.Errorf("GET /stream: second event %q, want data:second", line)
	}
}

// HANDLER_TIMEOUT on a started stream cancels the handler's context; the stream is not replaced by the 503 answer.
func TestInitRouter_TimedOutStreamKeepsStream(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 1})
	ctxErr := make(chan error, 1)
	r.GET("/stream", func(c *gin.Context) {
		c.Status(http.StatusCreated)
		c.SSEvent("msg", "first")
		c.Writer.Flush()
		waitDone(c.Request.Context(), 3*time.Second)
		ctxErr <- c.Request.Context().Err()
		c.SSEvent("msg", "after")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	code, body, header := get(t, srv.URL+"/stream")

	if code != http.StatusCreated || header.Get("Content-Type") != wantStreamContentType {
		t.Errorf("GET /stream = %d %q, want 201 %s", code, header.Get("Content-Type"), wantStreamContentType)
	}

	if want := "event:msg\ndata:first\n\nevent:msg\ndata:after\n\n"; body != want {
		t.Errorf("GET /stream body = %q, want %q without the timeout answer", body, want)
	}

	if err := <-ctxErr; !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("GET /stream: handler ctx.Err() = %v, want context.DeadlineExceeded", err)
	}
}

// A connection hijacked before the deadline belongs to the handler: the timeout answer is not written into it.
func TestInitRouter_HijackedConnectionGetsNoTimeoutAnswer(t *testing.T) {
	const raw = "HTTP/1.1 200 OK\r\nContent-Length: 6\r\nConnection: close\r\n\r\nhijack"

	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 1})
	handlerErr := make(chan error, 1)
	r.GET("/ws", func(c *gin.Context) {
		handlerErr <- writeHijacked(c, raw)
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	got := rawGet(t, srv.Listener.Addr().String(), "/ws")

	if err := <-handlerErr; err != nil {
		t.Fatalf("GET /ws: handler: %v", err)
	}

	if got != raw {
		t.Errorf("GET /ws: connection got %q, want only the handler's %q", got, raw)
	}
}

// Hijack after the timeout answer is refused with http.ErrHandlerTimeout.
func TestInitRouter_HijackAfterTimeoutRefused(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 1})
	answered := make(chan struct{})
	hijackErr := make(chan error, 1)
	r.GET("/ws", func(c *gin.Context) {
		<-answered
		_, _, err := c.Writer.Hijack()
		hijackErr <- err
	})

	w := newSignalRecorder()
	served := make(chan struct{})

	go func() {
		defer close(served)
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ws", nil))
	}()

	select {
	case <-w.written:
	case <-time.After(3 * time.Second):
		t.Fatal("GET /ws: no timeout answer within 3s with HandlerTimeout 1s")
	}

	close(answered)
	<-served

	if err := <-hijackErr; !errors.Is(err, http.ErrHandlerTimeout) {
		t.Errorf("GET /ws: Hijack after the timeout answer = %v, want http.ErrHandlerTimeout", err)
	}

	assertTimeoutAnswer(t, w.ResponseRecorder, "GET /ws")
}

// A handler panicking after the deadline does not replace the timeout answer with the recovery 500.
func TestInitRouter_PanicAfterDeadlineKeepsTimeoutAnswer(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 1})
	r.GET("/panic", func(c *gin.Context) {
		waitDone(c.Request.Context(), 3*time.Second)
		panic("late failure")
	})

	assertTimeoutAnswer(t, serve(r, http.MethodGet, "/panic"), "GET /panic")
}

// A request canceled by the client before the deadline gets the timeout answer, as with gin-timeout,
// and the handler sees context.Canceled.
func TestInitRouter_ClientCancelGetsTimeoutAnswer(t *testing.T) {
	r := newRouter(t, &baseConfig.BaseConfig{Name: "svc"}, config.HttpConfig{HandlerTimeout: 1})
	started := make(chan struct{})
	ctxErr := make(chan error, 1)
	r.GET("/slow", func(c *gin.Context) {
		close(started)
		waitDone(c.Request.Context(), 3*time.Second)
		ctxErr <- c.Request.Context().Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-started
		cancel()
	}()

	start := time.Now()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/slow", nil).WithContext(ctx))

	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("GET /slow canceled by the client answered in %v, want right after the cancel", elapsed)
	}

	if err := <-ctxErr; !errors.Is(err, context.Canceled) {
		t.Errorf("GET /slow: handler ctx.Err() = %v, want context.Canceled", err)
	}

	assertTimeoutAnswer(t, w, "GET /slow")
}

// A handler that answers after the deadline but before the timeout goroutine runs gets the timeout answer,
// and the late timeout goroutine does not write a second one.
func TestTimeoutWriter_AnswerAfterDeadlineGetsTimeoutAnswer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tw := newTimeoutWriter(c.Writer, ctx.Done())
	tw.Header().Set("X-Late", "1")
	tw.WriteHeader(http.StatusOK)

	if _, err := tw.WriteString("late"); err != nil {
		t.Fatalf("WriteString before complete: %v", err)
	}

	tw.complete(ctx)
	tw.timeout(ctx)

	assertTimeoutAnswer(t, w, "answer after the deadline")
}

// After the deadline, before the timeout goroutine runs, the handler already sees the timeout status:
// what it writes is dropped, streaming does not start, the client gets the timeout answer.
func TestTimeoutWriter_DeadlineBeforeTimeoutGoroutine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ctx, cancel := context.WithCancel(context.Background())
	tw := newTimeoutWriter(c.Writer, ctx.Done())
	tw.WriteHeader(http.StatusCreated)
	cancel()

	if got := tw.Status(); got != http.StatusServiceUnavailable {
		t.Errorf("Status() after the deadline = %d, want 503", got)
	}

	tw.WriteHeader(http.StatusInternalServerError)
	tw.Flush()

	if n, err := tw.WriteString("late"); n != 0 || err != nil {
		t.Errorf("WriteString after the deadline = (%d, %v), want (0, nil)", n, err)
	}

	if got := tw.Status(); got != http.StatusServiceUnavailable {
		t.Errorf("Status() after WriteHeader(500) = %d, want 503", got)
	}

	if w.Code != http.StatusOK || w.Body.Len() != 0 {
		t.Errorf("origin before complete = %d %q, want nothing written", w.Code, w.Body.String())
	}

	tw.complete(ctx)
	tw.timeout(ctx)

	assertTimeoutAnswer(t, w, "deadline before the timeout goroutine")
}

func openStream(t *testing.T, url string) (*http.Response, *bufio.Reader) {
	t.Helper()

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}

	return resp, bufio.NewReader(resp.Body)
}

func closeBody(t *testing.T, resp *http.Response) {
	t.Helper()

	if err := resp.Body.Close(); err != nil {
		t.Errorf("close body: %v", err)
	}
}

// readLine reads the stream up to the next line starting with prefix.
func readLine(t *testing.T, body *bufio.Reader, prefix string) string {
	t.Helper()

	for {
		line, err := body.ReadString('\n')
		if err != nil {
			t.Fatalf("read stream: %v", err)
		}

		if line = strings.TrimSpace(line); strings.HasPrefix(line, prefix) {
			return line
		}
	}
}

// writeHijacked takes over the connection, answers raw, keeps it past the deadline and closes it.
func writeHijacked(c *gin.Context, raw string) error {
	conn, rw, err := c.Writer.Hijack()
	if err != nil {
		return err
	}

	if _, err := rw.WriteString(raw); err != nil {
		return err
	}

	if err := rw.Flush(); err != nil {
		return err
	}

	waitDone(c.Request.Context(), 3*time.Second)
	time.Sleep(100 * time.Millisecond) // the timeout goroutine has run by now

	return conn.Close()
}

// rawGet sends GET path over a new connection and returns everything read until the server closes it.
func rawGet(t *testing.T, addr, path string) string {
	t.Helper()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close connection: %v", err)
		}
	}()

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}

	if _, err := conn.Write([]byte("GET " + path + " HTTP/1.1\r\nHost: test\r\n\r\n")); err != nil {
		t.Fatalf("write request: %v", err)
	}

	got, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(got)
}

func get(t *testing.T, url string) (int, string, http.Header) {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("GET %s: read body: %v", url, err)
	}

	if err := resp.Body.Close(); err != nil {
		t.Fatalf("GET %s: close body: %v", url, err)
	}

	return resp.StatusCode, string(body), resp.Header
}

// waitDone waits for ctx or limit, whichever comes first.
func waitDone(ctx context.Context, limit time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(limit):
	}
}

func assertFastRequests(t *testing.T, r *gin.Engine) {
	t.Helper()

	for i := 2; i < fastRequests+2; i++ {
		id := strconv.Itoa(i)

		if w := serve(r, http.MethodGet, "/fast/"+id); w.Code != http.StatusOK || w.Body.String() != id {
			t.Errorf("GET /fast/%s while a timed-out handler runs = %d %q, want 200 %q", id, w.Code, w.Body.String(), id)
		}
	}
}

func assertTimeoutAnswer(t *testing.T, w *httptest.ResponseRecorder, what string) {
	t.Helper()

	if w.Code != http.StatusServiceUnavailable || w.Body.String() != wantTimeoutBody {
		t.Errorf("%s = %d %q, want 503 %s", what, w.Code, w.Body.String(), wantTimeoutBody)
	}

	if ct := w.Header().Get("Content-Type"); ct != wantTimeoutContentType {
		t.Errorf("%s: Content-Type = %q, want %q", what, ct, wantTimeoutContentType)
	}

	if w.Header().Get("X-Late") != "" {
		t.Errorf("%s: header X-Late set after the deadline reached the client", what)
	}
}

// signalRecorder closes written on the first body write: the answer is sent while ServeHTTP may still run.
type signalRecorder struct {
	*httptest.ResponseRecorder
	once    sync.Once
	written chan struct{}
}

func newSignalRecorder() *signalRecorder {
	return &signalRecorder{ResponseRecorder: httptest.NewRecorder(), written: make(chan struct{})}
}

func (s *signalRecorder) Write(b []byte) (int, error) {
	n, err := s.ResponseRecorder.Write(b)
	s.once.Do(func() { close(s.written) })

	return n, err
}

func (s *signalRecorder) WriteString(str string) (int, error) {
	return s.Write([]byte(str))
}
