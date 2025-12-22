# HTTP Module (Gin)

HTTP-модуль предоставляет готовую интеграцию **Gin** в модульную архитектуру `App`
с поддержкой lifecycle, middleware, Sentry и graceful shutdown.

Модуль предназначен для использования в сервисах,
построенных на `gosdk-http-core` и `Application Core`.

---

## Возможности

- Инициализация `gin.Engine`
- Подключение middleware (logger, recovery, rate limit и др.)
- Интеграция с Sentry
- Централизованный `App` context
- Graceful shutdown HTTP-сервера
- Расширение через `PrepareComponentsFunc`

---

## Структура пакета

```
pkg/
 ├── app/
 │   └── modules/
 │       └── http.go
 ├── config/
 │   ├── http_config.go
 │   └── http_info.go
 ├── constants/
 ├── gin/
 ├── middleware/
 ├── helpers/
 └── structures/
```

---

## Подключение модуля

### Регистрация

```go
appInstance.RegisterModule(&modules.HttpModule{
    PrepareComponentsFunc: func(app *app.App, module *modules.HttpModule) error {
        router := module.Router

        router.GET("/health", func(c *gin.Context) {
            c.JSON(200, gin.H{"status": "ok"})
        })

        return nil
    },
})
```

---

### Запуск

```go
if err := appInstance.RunModule("http"); err != nil {
    log.Fatal(err)
}
```

---

### Ожидание завершения

```go
appInstance.WaitForShutdown()
```

---

## HttpModule

```go
type HttpModule struct {
    HttpConfig *config.HttpConfig
    Router     *gin.Engine
    Server     *http.Server

    PrepareComponentsFunc func(app *app.App, module *HttpModule) error
}
```

---

## Жизненный цикл

### Register(app)

Вызывается при `RegisterModule`.

Внутри:

- загрузка `HttpConfig`
- инициализация Sentry (если указан DSN)
- создание `gin.Engine`
- подключение middleware
- вызов `PrepareComponentsFunc`

⚠️ **Важно:**  
`Register()` не должен запускать сервер или блокировать поток.

---

### Start(app)

Вызывается при `RunModule("http")`.

Внутри:

- запуск HTTP-сервера в goroutine
- обработка ошибок сервера
- при фатальной ошибке вызывается `app.Fail(err)`

Пример логики:

```go
go func() {
    if err := m.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        app.Fail(err)
    }
}()
```

---

### Stop(ctx)

Вызывается при graceful shutdown приложения.

Внутри:

- `Server.Shutdown(ctx)`
- `sentry.Flush()`

Пример:

```go
func (m *HttpModule) Stop(ctx context.Context) error {
    err := m.Server.Shutdown(ctx)
    sentry.Flush(2 * time.Second)
    return err
}
```

---

## Конфигурация

Используется `HttpConfig`, загружаемый из env / config-файлов.

Основные параметры:

- `SERVER_ADDRESS`
- `SENTRY_DSN`
- `READ_TIMEOUT`
- `WRITE_TIMEOUT`
- `IDLE_TIMEOUT`

---

## Middleware

Пакет `middleware` содержит готовые middleware:

- Logger
- Recovery
- Request info
- Rate limiter
- Форматированные ответы

Все middleware подключаются в `ginHelper.InitRouter()`.

---

## Error handling

Модуль использует:

- централизованные структуры ошибок
- единый формат HTTP-ответов
- middleware для panic recovery
- Sentry для критических ошибок

---

## Рекомендации

### Делать

- регистрировать роуты через `PrepareComponentsFunc`
- использовать `module.Router`
- обрабатывать ошибки через `app.Fail(err)`

### Не делать

- запускать сервер в `Register()`
- блокировать `Start()`
- использовать `panic` или `log.Fatal`

---

## Пример использования

```go
httpModule := &modules.HttpModule{
    PrepareComponentsFunc: func(app *app.App, module *modules.HttpModule) error {
        api := module.Router.Group("/api")

        api.GET("/ping", func(c *gin.Context) {
            c.JSON(200, gin.H{"pong": true})
        })

        return nil
    },
}

appInstance.RegisterModule(httpModule)
appInstance.RunModule("http")
appInstance.WaitForShutdown()
```

---

## Итог

HTTP-модуль обеспечивает:

- чистую интеграцию Gin
- единый lifecycle
- безопасный shutdown
- расширяемость
- готовность к production
