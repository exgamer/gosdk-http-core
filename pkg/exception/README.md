# Errors & Responses

Пакет задаёт единый способ работы с ошибками и HTTP-ответами.

Идея простая:
- хендлеры не формируют JSON
- хендлеры кладут в gin.Context либо data, либо error
- один middleware формирует ответ (FormattedResponse)

---

## Успех

```go
helpers.SuccessResponse(c, data)          // 200
helpers.SuccessCreatedResponse(c, data)   // 201
helpers.SuccessDeletedResponse(c, nil)    // 204
```

---

## Ошибка

Базовый вариант (принимает error):

```go
helpers.ErrorResponse(c, err)
```

Если err AppException (пакет gosdk-core) → станет 500 Internal Server Error и будет учтен context и error

С указанием статуса:

```go
helpers.ErrorResponseWithStatus(c, http.StatusBadRequest, err, ctx)
```

Готовые шорткаты:

```go
helpers.BadRequest(c, err, ctx)
helpers.NotFound(c, err, ctx)
helpers.InternalServerError(c, err, ctx)
helpers.Unauthorized(c, err, ctx)
helpers.Forbidden(c, err, ctx)
```

Текстовые:

```go
helpers.BadRequestMsg(c, "invalid id", nil)
helpers.NotFoundMsg(c, "not found", nil)
```

---

## Пример хендлера

```go
func (h *CityHandler) View() gin.HandlerFunc {
    return func(c *gin.Context) {
        id, err := validators.GetIntQueryParam(c, "id")
        if err != nil {
            helpers.BadRequest(c, err, nil)
            return
        }

        item, err := h.cityService.GetById(c.Request.Context(), uint(id))
        if err != nil {
            helpers.InternalServerError(c, err, nil)
            return
        }

        if item == nil {
            helpers.NotFoundResponse(c, errors.New("Not Found"), nil)

            return
        }

        helpers.SuccessResponse(c, factories.OneResponse(item))
    }
}
```

---

## Формирование JSON

Нужен один middleware:

```go
func ResponseMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()
        helpers.FormattedResponse(c)
    }
}
```

Он:
- если есть exception → отдаёт error JSON
- иначе → отдаёт success JSON

---

## Правила

- В context кладём только error, не struct.
- Статус выставляется один раз (ErrorResponse).
- Логирование / Sentry / метрики читают exception через errors.As.
- JSON формируется в одном месте (FormattedResponse).
