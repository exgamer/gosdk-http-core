# Changelog

## v1.0.33

### Добавлено
- Типы ошибок: `unauthorized` (401), `method_not_allowed` (405), `request_timeout` (408), `conflict` (409), `payload_too_large` (413), `too_many_requests` (429), `bad_gateway` (502), `service_unavailable` (503), `gateway_timeout` (504), `client_error` (прочие 4xx).

### Изменено
- Поле `error` в ответах с ошибкой для перечисленных статусов: раньше все они получали `internal_server_error`. Если клиенты сравнивают `error` со строкой `internal_server_error` для этих статусов — обновить. Для 400, 403, 404, 406, 422, 500 значения прежние.
