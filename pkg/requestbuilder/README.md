# requestbuilder — README

Пакет `requestbuilder` — удобный построитель HTTP-запросов с:
- единым API для GET/POST/PUT/PATCH
- установкой заголовков, query params, таймаута
- удобной установкой JSON/XML body
- парсингом ответа в `Response[T]`
- логированием (через `FormattedLogWithAppInfo`)
- записью debug-стейтментов (через `debug.GetDebugFromContext`)

---

## Быстрый старт

### Импорт

```go
import "github.com/exgamer/gosdk-http-core/pkg/requestbuilder"
```

### Контракт ответа

```go
type Response[T any] struct {
    Success bool `json:"success"`
    Data    T    `json:"data"`
}
```

---

## Примеры

### GET с query params

```go
rb := requestbuilder.NewGetHttpRequestBuilder[Product](ctx, url).
    SetQueryParams(map[string]string{"id": "123"})

resp, err := rb.GetResult()
```

### POST JSON

```go
rb := requestbuilder.NewPostHttpRequestBuilder[CreateOrderResponse](ctx, url).
    SetJSONBody(payload)

resp, err := rb.GetResult()
```

### Отключить логи

```go
rb.SetLogsEnabled(false)
```

---

## Рекомендации

- Проверяйте `resp.Result.Success`
- Для `5xx` билдер вернёт ошибку
- Используйте `SetJSONBody` вместо ручной сериализации
