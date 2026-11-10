# Готовые структуры для описания ответов на разные HTTP статусы (можно использовать в тч для документации сваггера)

- HttpResponse
- BadRequestErrorResponse
- ForbiddenErrorResponse
- InternalServerResponse
- NotFoundErrorResponse
- ValidationErrorResponse
- UnauthorizedErrorResponse — 401
- PayloadTooLargeErrorResponse — 413
- UnsupportedMediaTypeErrorResponse — 415
- InternalServerErrorResponse — 500, ошибка обработчика в общей обёртке
- ServiceUnavailableErrorResponse — 503

Ошибки (кроме ответа на панику) отдаются в общей обёртке `{"success": false, "data": {status, error, message,
request_id, hostname, details}}`. `InternalServerResponse` — тело ответа на панику, для обычных 500 в Swagger —
`InternalServerErrorResponse`.
