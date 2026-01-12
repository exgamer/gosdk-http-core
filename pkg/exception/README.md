# Errors & Responses

Пакет задаёт единый способ работы с ошибками и HTTP-ответами.

Идея простая:
- хендлеры не формируют JSON
- хендлеры кладут в gin.Context либо data, либо error
- один middleware формирует ответ (FormattedResponse)

---

## Успех

```go
response.Success(c, data)          // 200
response.SuccessCreated(c, data)   // 201
response.SuccessDeletedR(c, nil)    // 204
```

---

## Ошибка

Базовый вариант (принимает error):

```go
response.Error(c, err)
```

Если err AppException (пакет gosdk-core) → станет 500 Internal Server Error и будет учтен context и error

С указанием статуса:

```go
helpers.ErrorResponseWithStatus(c, http.StatusBadRequest, err, ctx)
```

Готовые шорткаты:

```go
response.BadRequest(c, err, ctx)
response.NotFound(c, err, ctx)
response.InternalServerError(c, err, ctx)
response.Unauthorized(c, err, ctx)
response.Forbidden(c, err, ctx)
```

---

## Пример хендлера

```go
func (h *CityHandler) View() gin.HandlerFunc {
    return func(c *gin.Context) {
        id, err := validators.GetIntQueryParam(c, "id")
        if err != nil {
        response.BadRequest(c, err, nil)
            return
        }

        item, err := h.cityService.GetById(c.Request.Context(), uint(id))
        if err != nil {
        response.InternalServerError(c, err, nil)
            return
        }

        if item == nil {
        response.NotFoundResponse(c, errors.New("Not Found"), nil)

            return
        }

        response.SuccessResponse(c, factories.OneResponse(item))
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
		response.Formatted(c)
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
