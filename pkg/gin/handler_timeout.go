package gin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/exgamer/gosdk-core/pkg/logger"

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
		if skipHandlerTimeout(c) {
			c.Next()

			return
		}

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

// noTimeoutRoutes Маршруты «METHOD шаблон» без HANDLER_TIMEOUT (WithoutHandlerTimeout).
var noTimeoutRoutes sync.Map

// WithoutHandlerTimeout Маршрут method path (полный шаблон с префиксом групп, как в router.Routes(): "/api/v1/files/:id") выполняется без
// HANDLER_TIMEOUT: ответ не буферизуется, контекст по таймауту не отменяется. Для потоков (SSE) и больших файлов;
// срок записи такому маршруту задаёт WriteDeadline. Вызывать при регистрации маршрутов, до запуска сервера.
func WithoutHandlerTimeout(method, path string) {
	noTimeoutRoutes.Store(strings.ToUpper(method)+" "+path, struct{}{})
}

func skipHandlerTimeout(c *gin.Context) bool {
	route := c.FullPath()
	if route == "" {
		return false
	}

	_, ok := noTimeoutRoutes.Load(c.Request.Method + " " + route)

	return ok
}

// WriteDeadline Middleware маршрута: срок записи ответа — d от начала обработки вместо SERVER_WRITE_TIMEOUT_SEC
// (0 и меньше — без срока). Для потоков (SSE) и больших файлов. Под HANDLER_TIMEOUT не действует (ответ
// буферизуется) — маршрут исключают через WithoutHandlerTimeout; тогда в лог пишется предупреждение.
func WriteDeadline(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		var deadline time.Time
		if d > 0 {
			deadline = time.Now().Add(d)
		}

		if err := http.NewResponseController(c.Writer).SetWriteDeadline(deadline); err != nil {
			msg := fmt.Sprintf("write deadline for %s %s not set: %v", c.Request.Method, c.FullPath(), err)
			if errors.Is(err, http.ErrNotSupported) {
				msg += " (under HANDLER_TIMEOUT — exclude the route with WithoutHandlerTimeout)"
			}

			logger.Warning(c.Request.Context(), msg)
		}

		c.Next()
	}
}

// CheckTimeoutExemptions Пишет WARN по каждому маршруту WithoutHandlerTimeout, которого нет в роутере (путь без
// префикса группы, опечатка): такой маршрут остался бы под HANDLER_TIMEOUT. HttpKernel вызывает её при старте.
func CheckTimeoutExemptions(router *gin.Engine) {
	routes := make(map[string]struct{})
	for _, r := range router.Routes() {
		routes[r.Method+" "+r.Path] = struct{}{}
	}

	noTimeoutRoutes.Range(func(key, _ any) bool {
		if _, ok := routes[key.(string)]; !ok {
			logger.Warning(context.Background(), fmt.Sprintf("WithoutHandlerTimeout(%q): no such route in the router — "+
				"the path must be the full template with the group prefix, as in router.Routes()", key))
		}

		return true
	})
}
