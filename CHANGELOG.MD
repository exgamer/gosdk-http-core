# Changelog

## v1.0.34

### Добавлено
- Таймауты HTTP-сервера из env: `SERVER_READ_TIMEOUT_SEC`, `SERVER_READ_HEADER_TIMEOUT_SEC`, `SERVER_WRITE_TIMEOUT_SEC`, `SERVER_IDLE_TIMEOUT_SEC` (секунды, `-1` — без таймаута). Не заданы — прежние значения 15 / 10 / 30 / 60, поведение не меняется.

### Изменено
- `HttpKernel` дополняет собственный контекст запроса AppInfo, а не подменяет его контекстом приложения. Раньше по SIGTERM контекст приложения отменялся до того, как `Shutdown` дожидался текущих запросов, и они падали с `context canceled` (500 на каждом деплое).
- Как следствие, контекст запроса теперь отменяется, когда клиент уходит. Записи, которые должны завершиться в любом случае, оборачивайте в `context.WithoutCancel(ctx)` — перед обновлением проверьте такие места в сервисе.

## v1.0.33

### Добавлено
- Типы ошибок: `unauthorized` (401), `method_not_allowed` (405), `request_timeout` (408), `conflict` (409), `payload_too_large` (413), `too_many_requests` (429), `bad_gateway` (502), `service_unavailable` (503), `gateway_timeout` (504), `client_error` (прочие 4xx).

### Изменено
- Поле `error` в ответах с ошибкой для перечисленных статусов: раньше все они получали `internal_server_error`. Если клиенты сравнивают `error` со строкой `internal_server_error` для этих статусов — обновить. Для 400, 403, 404, 406, 422, 500 значения прежние.
