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

Документация сваггера будет доступна по следующим эндпойнтам

http://0.0.0.0:8090/rest-template/api-docs/index.html

http://0.0.0.0:8090/rest-template/api-docs/doc.json

---

## 🧠 Концепция HTTP Kernel

HTTP kernel — это kernel приложения, который:
- регистрирует HTTP-зависимости в DI
- инициализирует роутер
- поднимает HTTP сервер
- корректно завершает работу при shutdown

Kernel реализует интерфейс `KernelInterface` из `gosdk-core`.

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
HANDLER_TIMEOUT=30
SENTRY_DSN=
```

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
