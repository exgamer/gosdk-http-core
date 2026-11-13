package gin

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Ответ таймаута — тот же, что отдавал github.com/vearne/gin-timeout v0.2.3.
const (
	timeoutStatus      = http.StatusServiceUnavailable
	timeoutBody        = `{"code": -1, "msg":"http: Handler timeout"}`
	timeoutContentType = "text/plain; charset=utf-8"
)

// clientClosedStatus Статус запроса, оборванного клиентом до ответа, если обработчик ничего не написал
// (код nginx 499 Client Closed Request, в net/http его нет): не ошибка сервера — в логе WARN, в метриках не 5xx.
const clientClosedStatus = 499

// handlerTimeout Ограничивает время обработчиков после себя: по дедлайну отменяет контекст запроса
// и сразу отдаёт клиенту ответ таймаута; что обработчик напишет после, клиенту не уходит.
// Ответ таймаута — только по дедлайну (context.DeadlineExceeded). Обрыв клиентом (context.Canceled контекста
// запроса) таймаутом не считается: статус — тот, что выставил обработчик, а если он ничего не написал — 499.
// Потоковый ответ (после первого Flush) идёт клиенту напрямую: его таймаут только отменяет контекст.
// Цепочка выполняется в горутине запроса: gin.Context не возвращается в пул, пока обработчик работает,
// и не делится с другими запросами.
func handlerTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		request := c.Request
		tw := newTimeoutWriter(ctx, c.Writer)
		stop := context.AfterFunc(ctx, func() { tw.timeout(ctx) })

		defer func() {
			stop()
			tw.close()
			c.Writer = tw.origin
			c.Request = request
		}()

		c.Request = request.WithContext(ctx)
		c.Writer = tw
		c.Next()
		tw.complete(ctx)
	}
}
