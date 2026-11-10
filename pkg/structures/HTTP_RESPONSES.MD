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
- RawErrorResponse — тело ошибки без обёртки, ответы `response.*Raw` (любой статус)

Ошибки (кроме ответа на панику) отдаются в общей обёртке `{"success": false, "data": {status, error, message,
request_id, hostname, details}}`. `InternalServerResponse` — тело ответа на панику, для обычных 500 в Swagger —
`InternalServerErrorResponse`. Ответы `response.*Raw` (`ErrorResponseRaw`, `BadRequestRaw` и т. п.) отдают то же тело
`data` без обёртки — в Swagger `RawErrorResponse`.
