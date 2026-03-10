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


# Использование ошибок на доменном слое

Доменный слой **не должен знать про HTTP**.\
Сервисы возвращают обычные `error`, но при бизнес-ошибках используют
`AppException`.

Контроллер передает любую ошибку в `ErrorResponse`, где она
автоматически преобразуется в HTTP ответ.

------------------------------------------------------------------------

# Пример: Validation ошибка

### Service

``` go
func (s *TariffService) Create(ctx context.Context, dto CreateTariffDto) error {

    if dto.Name == "" {
        return &exception.NewValidationException(map[string]any{
		"name":  "Не указано имя",
	    },
	    false)
    }

    return nil
}
```

------------------------------------------------------------------------

# Пример: Not Found

### Service

``` go
func (s *TariffService) GetById(ctx context.Context, id uint) (*Tariff, error) {

    tariff, err := s.repo.GetById(ctx, id)
    if err != nil {
        return nil, err
    }

    if tariff == nil {
        return nil, &exception.NewNotFoundException(errors.New("tariff not found"), false)
        }
    }

    return tariff, nil
}
```

------------------------------------------------------------------------

# Пример: Forbidden

``` go
return &exception.NewForbiddenException(errors.New("access denied"), false) 
```

------------------------------------------------------------------------

# Пример: Internal ошибка

``` go
return &exception.AppException{
    Err: err,
    Kind: exception.ErrorKindInternal,
    TrackInSentry: true,
}
```

------------------------------------------------------------------------

# Controller

Контроллер не анализирует тип ошибки.

``` go
func (h *Handler) GetTariff(c *gin.Context) {

    result, err := h.service.GetTariff(c.Request.Context(), id)
    if err != nil {
        response.ErrorResponse(c, err)
        return
    }

    response.Success(c, result)
}
```

------------------------------------------------------------------------

# Что происходит дальше

Service → AppException → Controller → ErrorResponse() → HttpException →
JSON response

------------------------------------------------------------------------

# Пример HTTP ответа

``` json
{
  "status": 404,
  "error": "not_found",
  "message": "tariff not found",
  "details": {
    "id": 42
  }
}
```

------------------------------------------------------------------------

# Главное правило

**Domain / Service слой** - возвращает `AppException` - не знает про
HTTP

**Controller** - просто передает ошибку в `ErrorResponse` - не содержит
логики обработки ошибок
