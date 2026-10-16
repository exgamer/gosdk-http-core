# Changelog

## v1.0.38

Уровень лога запроса зависит от статуса; `ValidateRequestQuery` отвечает 422. Сигнатуры не изменились.
Требует `gosdk-core` v1.0.28 (`logger.Error` на уровне `ERROR`).

### Добавлено
- `logger.FormattedWarning` - форматированный HTTP-лог на уровне `WARN` (те же параметры, что у `FormattedInfo`).

### Исправлено
- `middleware.LoggerMiddleware`: ответы 4xx пишутся на уровне `WARN`, 5xx и ошибки gin (`c.Error`) - на уровне
  `ERROR`. Раньше 4xx с исключением шёл через `logger.Error`, и с `gosdk-core` v1.0.28 каждый такой ответ
  попадал бы в лог как `[ERROR]`.
- `validators.ValidateRequestQuery` вызывает `ShouldBindQuery` вместо `BindQuery`: gin больше не выставляет
  ответу код 400 и не кладёт ошибку разбора в `c.Errors`, код ответа совпадает с `status` в теле (422).
- `validators.ValidateRequestQuery`: ошибка разбора числа или флага (`page=abc`) - в `details` ключ - имя
  параметра, сообщение `Invalid value, expected number` / `Invalid value, expected boolean`. Раньше ключом было
  само значение (`abc`), сообщением - текст `strconv` со значением, в `message` - тот же текст.
- `validators.ValidateRequestBody`, `ValidateRequestQuery`: `CustomValidationRules` регистрируются в валидаторе
  gin один раз на тип запроса и имя правила, а не на каждом запросе, и не во время биндинга других запросов.
  Раньше параллельные запросы давали гонку данных, вплоть до `fatal error: concurrent map writes`.

### ⚠️ Изменения поведения
- `LoggerMiddleware`, запрос завершился исключением (`exception` в контексте) с кодом 4xx: строка `[WARN]` вместо
  `[DEBUG]` (`logger.Error` до `gosdk-core` v1.0.28 - видна только на `debug`/`trace`). Видна при `LOG_LEVEL`
  `trace`, `debug`, `info`, `warn`; при `error` и `fatal` скрыта. Исключение с кодом 4xx, но записанным
  статусом 5xx - `[ERROR]`.
- `LoggerMiddleware`, ответ 4xx без исключения и без ошибок gin (например, `c.AbortWithStatus(401)`, 404 по
  неизвестному маршруту): строка `[WARN]` вместо `[INFO]`, текст тот же (заголовки, query, тело - на `debug`).
  При `LOG_LEVEL=warn` теперь видна (раньше скрыта).
- `LoggerMiddleware`, 5xx, ошибки gin, исключения 5xx и не-`HttpException` - по-прежнему `logger.Error`: с
  `gosdk-core` v1.0.28 строка `[ERROR]` видна при `LOG_LEVEL` `info`, `warn`, `error` (раньше `[DEBUG]`, только
  на `debug`/`trace`). Без изменений: 1xx-3xx - `[INFO]`; `FormattedError`, `FormattedErrorWithHttpInfo` -
  `logger.Error`.
- `ValidateRequestQuery`: HTTP-код ответа на ошибку разбора и валидации query - 422 вместо 400 (тело прежнее,
  `status: 422`). Клиенты и тесты, которые ждут 400 на неверный query, - обновить. В `go-sdk-rest-template` -
  список `handbook/city` (`ValidateRequestQuery`).
- `ValidateRequestQuery`: в `c.Errors` нет ошибки разбора (`ErrorTypeBind`) - если middleware или обработчик
  читали её оттуда, её там больше нет. `LoggerMiddleware` такой запрос пишет `[WARN]` (исключение 422).
- `ValidateRequestQuery`: ошибка разбора числа/флага - `message` `validation error` и `details` по имени
  параметра (см. «Исправлено»). Если значение не удалось сопоставить с параметром - `details` пустой.

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
