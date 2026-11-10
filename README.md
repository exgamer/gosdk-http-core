# gosdk-http-core

`gosdk-http-core` — HTTP kernel для Go-приложений, построенных на базе `gosdk-core`.
Пакет инкапсулирует инициализацию HTTP-сервера, роутера и middleware и подключается к приложению как **kernel**.

Основная цель — дать единый и предсказуемый способ поднятия HTTP (REST) слоя поверх core SDK.

---

## 📦 Возможности

- 🌐 HTTP kernel для `gosdk-core`
- 🚀 Инициализация HTTP сервера
- 🧭 Поддержка роутера (Gin)
- 🧩 Регистрация middleware
- ⚙️ Конфигурация через config
- ♻️ Корректный shutdown сервера
- 🧠 Интеграция с DI контейнером

---

## 🚀 Установка

```bash
go get github.com/exgamer/gosdk-http-core
```

---

[Что доступно в DI из коробки](pkg/di/DI_FUNCTIONS_README.MD)

---

[Структуры описывающие HTTP ответы](pkg/structures/HTTP_RESPONSES.MD)

---

[Возможности в режиме отладки](DEBUG_MODE.MD)

---

[Возможности логирования HTTP](pkg/logger/README.md)

---

[HTTP exceptions](pkg/exception/README.md)

---
## SWAGGER

### Генерация документации сваггера:
```go
go install github.com/swaggo/swag/cmd/swag@latest
```

```go
swag init -g main.go -o docs --parseDependency --parseInternal --parseDepth 1
```

### Включение

Swagger по умолчанию выключен на любом окружении, `APP_ENV` на это не влияет. Включает переменная `SWAGGER_ENABLED`:

| `SWAGGER_ENABLED` | Swagger |
|---|---|
| `true`, `1` (регистр и пробелы по краям не важны) | включён |
| не задана, пустая, `false`, `0` | выключен |
| любое другое значение (`yes`, `on`, опечатка) | выключен, в лог - `[WARN]` |

При старте `InitRouter` пишет в лог `[INFO] swagger enabled: SWAGGER_ENABLED="true"` или
`swagger disabled: SWAGGER_ENABLED=""`.

Включённый Swagger доступен по путям (префикс - `SWAGGER_PREFIX`, если пуст - `APP_NAME`, если пуст и он - `swagger`):

http://0.0.0.0:8090/rest-template/index.html

http://0.0.0.0:8090/rest-template/doc.json

Выключенный - маршрута нет, `index.html`, `doc.json` и статика UI отвечают 404
`{"code":"PAGE_NOT_FOUND","message":"404 page not found"}`.

На проде SDK документацию аутентификацией не закрывает: `SWAGGER_ENABLED=true` там - только вместе с закрытием пути
`/<prefix>/` на ingress/шлюзе.

---

## HTTP_ERROR_DETAILS

Паника в обработчике или middleware под `InitRouter`: ответ 500 без деталей, полная ошибка - в лог и Sentry.

```json
{"message": "internal server error", "details": [], "success": false, "service_code": 0}
```

В лог - одна запись `[ERROR]`: `[request_id,METHOD,path,status] panic: <текст>` и стек (переводы строк логгер
`gosdk-core` пишет как `\n`, запись остаётся одной строкой). Путь - без query, заголовки и тело запроса не пишутся.
Статус - тот, что получил клиент: 500, после `HANDLER_TIMEOUT` - 503. В Sentry - одно событие, как раньше.

Текст паники и стек в ответе включает переменная `HTTP_ERROR_DETAILS` - только для локальной отладки:

| `HTTP_ERROR_DETAILS` | Текст паники и стек в ответе 500 |
|---|---|
| `true`, `1` (регистр и пробелы по краям не важны) | есть: `message` - текст паники, `details` - стек |
| не задана, пустая, `false`, `0` | нет |
| любое другое значение (`yes`, `on`, опечатка) | нет, в лог - `[WARN]` |

На проде `HTTP_ERROR_DETAILS` не задавать: SDK окружение не проверяет, с `true` клиент получит текст паники и стек
(пути файлов, внутренние данные).

При старте `InitRouter` пишет в лог `[INFO] error details disabled: HTTP_ERROR_DETAILS=""` или
`error details enabled: HTTP_ERROR_DETAILS="true"` - включённую настройку видно в логе старта. Нераспознанное
значение - дополнительно `[WARN] unrecognized HTTP_ERROR_DETAILS="yes", error details disabled; use true/1 or false/0`.

Роутер без `InitRouter`, где паники ловит `ErrorHandler` пакета `pkg/gin`, - ответ всегда без деталей, настройка не
читается. Подключать через `gin.CustomRecoveryWithWriter(nil, ginHelper.ErrorHandler)` (`ginHelper` - импорт
`pkg/gin` SDK, `gin` - `github.com/gin-gonic/gin`), а не `gin.CustomRecovery`: тот
пишет в stderr дамп gin `[Recovery]` с заголовками запроса: `Authorization` gin маскирует, а `Cookie` уходит открытым
текстом (в debug-режиме gin и при обрыве соединения клиентом).

---

## 🧠 Концепция HTTP Kernel

HTTP kernel — это kernel приложения, который:
- регистрирует HTTP-зависимости в DI
- инициализирует роутер
- поднимает HTTP сервер
- корректно завершает работу при shutdown

Kernel реализует интерфейс `KernelInterface` из `gosdk-core`.

### Контекст запроса

Kernel дополняет собственный контекст запроса данными приложения (AppInfo), а не заменяет его:

- **уход клиента отменяет запрос**: `c.Request.Context()` отменяется, и вызовы БД/Redis/HTTP с этим контекстом прерываются;
- **остановка сервиса (SIGTERM) текущие запросы не обрывает**: `Shutdown` даёт им доработать.

Запись, которая должна завершиться даже если клиент ушёл (платёж, колбэк внешней системы), отвяжите от отмены явно:

```go
ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
defer cancel()
```

Данные запроса (`HttpInfo`), которые кладёт `RequestInfoMiddleware`, читаются только через
`gin.GetHttpInfoFromContext(ctx)`; в тестах положить их в контекст - `gin.WithHttpInfo(ctx, info)`. AppInfo - через
`GetAppInfoFromContext` / `WithAppInfo` пакета `context` из `gosdk-core`. Чтение по строковым ключам
(`ctx.Value("http_info")`, `ctx.Value("app_info")`) возвращает `nil`.

---

## 🔌 Регистрация HTTP Kernel

```go
httpKernel := httpkernel.NewHttpKernel()

app.RegisterKernel(httpKernel)
```

---

## ⚙️ Конфигурация

HTTP kernel использует конфигурацию из `pkg/config`.

Пример env-переменных:

```env
SERVER_ADDRESS=0.0.0.0:8090
SWAGGER_PREFIX=rest-template
SWAGGER_ENABLED=
HTTP_ERROR_DETAILS=
HANDLER_TIMEOUT=30
SENTRY_DSN=
```

`SWAGGER_ENABLED` пустой - Swagger выключен; значения и прод - в разделе [SWAGGER](#swagger).
`HTTP_ERROR_DETAILS` пустой - ответ 500 без деталей; на проде не задавать - раздел [HTTP_ERROR_DETAILS](#http_error_details).

Таймауты HTTP-сервера, секунды. Не заданы (или `0`) — значения по умолчанию, `-1` — без таймаута:

| Переменная | По умолчанию | Что ограничивает |
|---|---|---|
| `SERVER_READ_TIMEOUT_SEC` | 15 | чтение запроса целиком, с телом |
| `SERVER_READ_HEADER_TIMEOUT_SEC` | 10 | чтение заголовков запроса |
| `SERVER_WRITE_TIMEOUT_SEC` | 30 | от конца чтения заголовков до конца записи ответа |
| `SERVER_IDLE_TIMEOUT_SEC` | 60 | простой keep-alive соединения между запросами |

Долгие ответы (выгрузки, стриминг) обрываются по `SERVER_WRITE_TIMEOUT_SEC` — для них увеличьте его или задайте `-1`.

---

## 🧩 Работа с роутером

Роутер (Gin) регистрируется в DI и доступен в бизнес-модулях.

```go
router, err := di.GetRouter(app)
if err != nil {
    return err
}

baseConfig, err := app.GetBaseConfig(a) // из `gosdk-core`
if err != nil {
    return err
}

service.Use(middleware.RequestInfoMiddleware(baseConfig))  // мидлвейр который записывает в контекст данные о приложении и запросе
service.Use(middleware.LoggerMiddleware())                 // мидлвейр который логирует запросы
service.Use(middleware.FormattedResponseMiddleware())      // мидлвейр который обрабатывает ответ от контроллера
service.Use(middleware.MetricsMiddleware())                // мидлвейр который записывает в метрики прометея данные о вызванных эндпойнтах и времени работы (расширяет эндпойнт /metrics)
service.Use(middleware.DebugMiddleware())                  // дебаг инфа в ответе от сервиса  (работает только если DEBUG=true)
service.Use(middleware.SentryMiddleware())                  // мидлвейр для отправки ошибок в сентри

router.GET("/health", func(c *gin.Context) {
    c.JSON(200, gin.H{"status": "ok"})
})
```

---

## 📤 Формат ответа: `{success, data}` и raw-ответы

По умолчанию `middleware.FormattedResponseMiddleware()` оборачивает любой ответ хендлера
в единый конверт:

```json
{
  "success": true,
  "data": { "...": "..." }
}
```

Хендлер не пишет ответ напрямую (никаких `c.JSON`) — он кладёт данные в контекст через
`response.*`, а middleware после `c.Next()` сериализует их через `response.Formatted(c)`:

```go
// success
response.Success(c, data)          // 200
response.SuccessCreated(c, data)   // 201
response.SuccessAccepted(c, data)  // 202
response.SuccessDeleted(c, data)   // 204

// error
response.BadRequest(c, err, ctx)
response.NotFound(c, err, ctx)
response.InternalServerError(c, err, ctx)
// ...и т.д., см. pkg/response/http_response_helper.go
```

Ошибки оборачиваются аналогично — `{"success": false, "data": {"status", "error", "message", "request_id", "hostname", "details"}}`.

### Raw-ответы (без конверта `success/data`)

Для эндпойнтов с внешним/фиксированным контрактом (вебхуки, колбэки платёжных систем,
прокси на сторонние API и т.п.), где обёртка `success/data` не нужна и мешает контракту,
есть raw-аналоги в `pkg/response/http_raw_response_helper.go` — по одной функции на каждую
существующую:

```go
// success (raw)
response.RawSuccess(c, data)          // 200, тело = data как есть
response.RawCreated(c, data)          // 201
response.RawAccepted(c, data)         // 202
response.RawDeleted(c, data)          // 204
response.Raw(c, data, http.StatusTeapot) // произвольный статус

// error (raw) — конверт success/data снимается,
// но структура тела ошибки (status/error/message/...) сохраняется
response.BadRequestRaw(c, err, ctx)
response.NotFoundRaw(c, err, ctx)
response.InternalServerErrorRaw(c, err, ctx)
// ...и т.д.
```

Raw-режим работает через тот же `FormattedResponseMiddleware` — переключение делается
флагом в `gin.Context`, отдельная middleware не нужна.

**Важно:** это расширение полностью аддитивное — старые `response.Success/BadRequest/...`
не изменены и продолжают оборачивать ответ как раньше. Флаг raw выставляется только внутри
новых `*Raw`-функций, поэтому существующие эндпойнты не затрагиваются.

---

## 🧱 Метрики прометея

После подключения Http ядра в приложении уже будет эедпойнт /metrics, который отдает метрики для прометея

Если подключить к эндпойнту middleware middleware.MetricsMiddleware(), в метриках появтся данные о вызовах эндпойнта, времени рабоыт и скорости


---

## ♻️ Graceful Shutdown

HTTP kernel автоматически:
- останавливает приём новых соединений
- дожидается завершения активных запросов
- завершает сервер по контексту приложения

---

## 🧪 Рекомендации

- ❌ не создавайте HTTP сервер вручную
- ✅ используйте роутер из DI
- ✅ регистрируйте роуты в бизнес-модулях
- ❌ не храните бизнес-логику в middleware

---

## 📌 Используется вместе с

- `gosdk-core`
- Gin
- internal business modules

---

## 📝 License

MIT или внутренняя лицензия компании
