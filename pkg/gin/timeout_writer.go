package gin

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/gin-gonic/gin"
)

const noWritten = -1

// timeoutWriter Буфер ответа обработчиков под handlerTimeout. В origin пишет под mu ровно один раз:
// либо ответ обработчика после его завершения (complete), либо ответ таймаута по дедлайну (timeout).
// Исключения — потоковый ответ после первого Flush и Hijack: дальше обработчик пишет в origin сам,
// ответ таймаута не пишется. Работа с origin в обход буфера — тоже под mu.
// Состояние таймаута наступает с дедлайном, а не с запуском горутины context.AfterFunc: обработчик,
// увидевший отмену контекста, видит и статус ответа таймаута (лог и Sentry паники — со статусом клиента).
// Отмена контекста без дедлайна (клиент оборвал запрос) таймаутом не считается: ответ — обработчика,
// без ответа обработчика — статус clientClosedStatus.
type timeoutWriter struct {
	origin gin.ResponseWriter
	header http.Header
	body   bytes.Buffer
	ctxErr func() error // Err() контекста с таймаутом обработчика

	mu          sync.Mutex
	status      int
	size        int
	wroteHeader bool
	timedOut    bool
	answered    bool
	streaming   bool
	hijacked    bool
	done        bool
}

var _ gin.ResponseWriter = (*timeoutWriter)(nil)

// newTimeoutWriter ctx — контекст с таймаутом обработчика.
func newTimeoutWriter(ctx context.Context, origin gin.ResponseWriter) *timeoutWriter {
	return &timeoutWriter{
		origin: origin,
		header: make(http.Header),
		ctxErr: ctx.Err,
		status: origin.Status(),
		size:   noWritten,
	}
}

// timeout Отдаёт ответ таймаута, если наступил дедлайн, а ответ ещё не отдан и не начат.
// Вызывается из горутины context.AfterFunc — и при обрыве запроса клиентом: тогда ничего не делает.
func (w *timeoutWriter) timeout(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.done || w.hijacked || w.streaming || !w.deadlineExceeded() {
		return
	}

	w.markTimedOut()
	w.writeTimeout(ctx)
}

// expire Переводит буфер в состояние таймаута, если дедлайн наступил, а горутина context.AfterFunc
// ещё не отработала; вызывается под mu в начале методов обработчика.
func (w *timeoutWriter) expire() {
	if w.timedOut || w.done || w.hijacked || w.streaming {
		return
	}

	if w.deadlineExceeded() {
		w.markTimedOut()
	}
}

// deadlineExceeded Контекст завершён дедлайном (таймаут обработчика или дедлайн родителя), а не отменой.
func (w *timeoutWriter) deadlineExceeded() bool {
	return errors.Is(w.ctxErr(), context.DeadlineExceeded)
}

// clientClosed Клиент оборвал запрос (context.Canceled), а обработчик ещё ничего не написал:
// статус ответа — clientClosedStatus; вызывается под mu.
func (w *timeoutWriter) clientClosed() bool {
	return !w.timedOut && !w.wroteHeader && w.size == noWritten && errors.Is(w.ctxErr(), context.Canceled)
}

// markTimedOut Статус ответа — статус таймаута: его получит клиент; вызывается под mu.
func (w *timeoutWriter) markTimedOut() {
	w.timedOut = true
	w.status = timeoutStatus
}

// writeTimeout Пишет в origin ответ таймаута один раз; вызывается под mu.
func (w *timeoutWriter) writeTimeout(ctx context.Context) {
	if w.answered {
		return
	}

	w.answered = true

	// Content-Length и Flush: клиент получает ответ целиком сразу, не дожидаясь конца обработчика.
	// Connection: close: соединение занято обработчиком до его конца, следующий запрос клиента
	// не должен ждать его в этом соединении.
	w.origin.Header().Set("Content-Type", timeoutContentType)
	w.origin.Header().Set("Content-Length", strconv.Itoa(len(timeoutBody)))
	w.origin.Header().Set("Connection", "close")
	w.origin.WriteHeader(timeoutStatus)

	if _, err := w.origin.WriteString(timeoutBody); err != nil {
		logger.Warning(ctx, fmt.Sprintf("handler timeout: write timeout response: %v", err))

		return
	}

	w.origin.Flush()
}

// complete Переносит в origin ответ обработчика, если он уложился в срок; иначе отдаёт ответ таймаута,
// если горутина context.AfterFunc не успела его отдать.
func (w *timeoutWriter) complete(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.done || w.hijacked || w.streaming {
		w.done = true

		return
	}

	w.expire()
	w.done = true

	if w.timedOut {
		w.writeTimeout(ctx)

		return
	}

	if w.clientClosed() {
		w.status = clientClosedStatus
	}

	w.writeBuffered()

	if w.body.Len() == 0 {
		return
	}

	if _, err := w.origin.Write(w.body.Bytes()); err != nil {
		logger.Warning(ctx, fmt.Sprintf("handler timeout: write response: %v", err))
	}
}

// writeBuffered Переносит в origin заголовки и статус обработчика; вызывается под mu.
// Тело остаётся в body: его пишет вызывающий.
func (w *timeoutWriter) writeBuffered() {
	for key, values := range w.header {
		w.origin.Header()[key] = values
	}

	w.origin.WriteHeader(w.status)

	if w.size != noWritten {
		w.origin.WriteHeaderNow()
	}
}

// close После close ни timeout, ни complete в origin не пишут.
func (w *timeoutWriter) close() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.done = true
}

func (w *timeoutWriter) Header() http.Header {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.streaming {
		return w.origin.Header()
	}

	return w.header
}

// WriteHeader Как у gin: код после отправки заголовков (WriteHeaderNow, Write) не меняется.
func (w *timeoutWriter) WriteHeader(code int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.expire()

	if w.streaming {
		w.origin.WriteHeader(code)

		return
	}

	if w.timedOut || code <= 0 || w.size != noWritten {
		return
	}

	w.status = code
	w.wroteHeader = true
}

func (w *timeoutWriter) WriteHeaderNow() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.expire()

	if w.streaming {
		w.origin.WriteHeaderNow()

		return
	}

	if w.size == noWritten {
		w.size = 0
	}
}

// Write После таймаута отбрасывает данные, как gin-timeout: (0, nil). Тело при статусе без тела
// (1xx, 204, 304) отклоняет с http.ErrBodyNotAllowed, как net/http без таймаута.
func (w *timeoutWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.expire()

	if w.streaming {
		return w.origin.Write(data)
	}

	if w.timedOut {
		return 0, nil
	}

	if w.size == noWritten {
		w.size = 0
	}

	if !bodyAllowed(w.status) {
		return 0, http.ErrBodyNotAllowed
	}

	n, err := w.body.Write(data)
	w.size += n

	return n, err
}

func (w *timeoutWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *timeoutWriter) Status() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.expire()

	if w.streaming {
		return w.origin.Status()
	}

	if w.clientClosed() {
		return clientClosedStatus
	}

	return w.status
}

func (w *timeoutWriter) Size() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.streaming {
		return w.origin.Size()
	}

	return w.size
}

func (w *timeoutWriter) Written() bool {
	return w.Size() != noWritten
}

// Flush Начинает потоковый ответ: буфер обработчика уходит клиенту, дальше обработчик пишет в origin
// напрямую, как без таймаута. Таймаут такого ответа только отменяет контекст: 503 начатый поток не подменяет.
func (w *timeoutWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.expire()

	if w.done || w.timedOut || w.hijacked {
		return
	}

	if !w.streaming {
		w.startStreaming()
	}

	w.origin.Flush()
}

// startStreaming Переносит в origin заголовки, статус и буфер обработчика; вызывается под mu.
func (w *timeoutWriter) startStreaming() {
	w.streaming = true
	w.writeBuffered()

	if w.body.Len() == 0 {
		return
	}

	// Ошибка записи в соединение не теряется: net/http сохраняет её и вернёт на следующих записях обработчика.
	if _, err := w.origin.Write(w.body.Bytes()); err != nil {
		return
	}

	w.body.Reset()
}

// Hijack Соединение переходит обработчику; ни ответ таймаута, ни буфер в него не пишутся.
func (w *timeoutWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.expire()

	if w.done || w.timedOut {
		return nil, nil, http.ErrHandlerTimeout
	}

	conn, rw, err := w.origin.Hijack()
	if err == nil {
		w.hijacked = true
	}

	return conn, rw, err
}

// CloseNotify Устарело в net/http, оставлено для интерфейса gin.ResponseWriter.
func (w *timeoutWriter) CloseNotify() <-chan bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.origin.CloseNotify()
}

func (w *timeoutWriter) Pusher() http.Pusher {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.origin.Pusher()
}

// bodyAllowed Как net/http: у 1xx, 204 и 304 тела нет.
func bodyAllowed(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == http.StatusNoContent, status == http.StatusNotModified:
		return false
	}

	return true
}
