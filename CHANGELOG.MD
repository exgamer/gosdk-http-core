# Changelog

## v1.5.0

Swagger-тип тела ошибки без обёртки. Сигнатуры не изменились.

### Добавлено
- `structures.RawErrorResponse` — тело ошибки ответов `response.*Raw` (`ErrorResponseRaw`, `ErrorResponseWithStatusRaw`,
  `BadRequestRaw` и т. п.): `{status, request_id, error, message, hostname, details}` без `success`/`data`.

## v1.4.1

Метрики больше не читают тело запроса. Сигнатуры не изменились.

### Исправлено
- `InitRouter`: middleware метрик (`go-gin-prometheus`) читал тело запроса целиком (`io.ReadAll`) до обработчиков —
  в память без лимита размера и до дедлайнов и проверок сервиса. Теперь `DisableBodyReading: true`: размер запроса в
  метрике берётся из `Content-Length`, тело достаётся обработчику непрочитанным. Имена и метки метрик прежние.

## v1.4.0

Проверка параметров пути запроса. Сигнатуры не изменились.

### Добавлено
- `validators.ValidateRequestPath(c, request)` — биндинг и проверка параметров пути (`/stations/:code`) в поля с
  тегом `uri`: правила — тег `binding` и `CustomValidationRules`, сообщения — `CustomValidationMessage` /
  `ValidationMessage`, как у `ValidateRequestQuery`. Ошибка — 422 с ключом по полю (snake_case), значение параметра
  в ответ не попадает; ошибка разбора числа — «Invalid value, expected number» по имени параметра. Query-параметры
  с тем же именем значение из пути не подменяют.

## v1.3.0

Типы ответов для Swagger на 401, 413, 415, 500 и 503 и успешный ответ 202. Сигнатуры не изменились.

### Добавлено
- `structures.UnauthorizedErrorResponse` (401), `PayloadTooLargeErrorResponse` (413),
  `UnsupportedMediaTypeErrorResponse` (415), `InternalServerErrorResponse` (500 — ошибка обработчика в общей
  обёртке), `ServiceUnavailableErrorResponse` (503). Поля совпадают с телом `response.Formatted`; `details` —
  `map[string]any`. `InternalServerResponse` по-прежнему описывает только ответ на панику.
- `response.SuccessAccepted` — 202 в общей обёртке, `response.RawAccepted` — 202 без обёртки.

## v1.2.0

Паника больше не отдаёт клиенту текст и стек: ответ 500 без деталей, полная ошибка - в лог и Sentry. HttpInfo и
AppInfo лежат в контексте под ключами своего типа. Требует `gosdk-core` v1.1.0. Сигнатуры не изменились.

### Добавлено
- `config.HttpConfig.ErrorDetails`, env `HTTP_ERROR_DETAILS`: текст паники и стек в ответе 500. Включают `true` или
  `1` (регистр и пробелы по краям не важны). Не задана, `false`, `0` и любое другое значение - выключено. Только для
  локальной отладки, на проде не задавать.
- `InitRouter` при старте пишет строку `[INFO]`: `error details enabled: HTTP_ERROR_DETAILS="..."` или
  `error details disabled: HTTP_ERROR_DETAILS="..."`. Нераспознанное значение - дополнительно `[WARN]`:
  `unrecognized HTTP_ERROR_DETAILS="...", error details disabled; use true/1 or false/0`.
- Лог паники: одна запись `[ERROR]` `[request_id,METHOD,path,status] panic: <текст>` со стеком. Путь - без query,
  заголовки и тело запроса не пишутся. Статус - тот, что получил клиент (500, после `HANDLER_TIMEOUT` - 503). Без
  `RequestInfoMiddleware` метод и путь берутся из запроса, `request_id` пустой. Раньше текст паники в лог SDK не попадал.
- `gin.WithHttpInfo(ctx, info)` - кладёт HttpInfo в контекст; нужен тестам и коду, который собирает контекст запроса
  без `RequestInfoMiddleware`.

### Исправлено
- Событие Sentry при панике получает статус ответа клиенту: 500 (раньше - 200), после таймаута обработчика - 503.
- Замечания `golangci-lint`: `errcheck` (`CaptureToSentry` - результат `errorreporter.Capture` отброшен явно: это та же
  ошибка с пометкой «отправлена», а не ошибка отправки), `SA1029` (строковые ключи контекста). Sentry работает как
  раньше - одно событие на панику.
- `HANDLER_TIMEOUT > 0`, паника после дедлайна: лог и событие Sentry паники могли показать статус 500, хотя клиент
  получил 503 - состояние таймаута зависело от того, успела ли отработать фоновая горутина. Теперь оно наступает с
  дедлайном: статус в цепочке, `Flush` и `Hijack` после дедлайна ведут себя как после ответа таймаута.

### Изменено
- `gosdk-core` поднят до v1.1.0: AppInfo в контексте под собственным ключом, `context.WithAppInfo`.
- `constants.HttpInfoKey` помечен `Deprecated`: ключом контекста больше не служит.

### ⚠️ Изменения поведения
- Ответ 500 при панике под `InitRouter` и в `gin.ErrorHandler`: `message` - `"internal server error"`, `details` -
  `[]`. Раньше в `message` был текст паники, в `details` - стек. Для отладки - `HTTP_ERROR_DETAILS=true` локально;
  на проде не задавать: SDK окружение не проверяет, включённое значение видно только в строке `[INFO]` при старте.
  `gin.ErrorHandler` (роутер без `InitRouter`) настройку не читает - детали всегда выключены.
- Паника в middleware прерывает цепочку: ответ - `AbortWithStatusJSON`, как у `gin.Recovery`. Раньше следующий
  обработчик цепочки выполнялся (в том числе обработчик за упавшим middleware авторизации), и его тело дописывалось
  после ответа 500.
- `InitRouter` больше не пишет в stderr дамп gin `[Recovery]` (в debug-режиме gin - с заголовками запроса, `Cookie`
  открытым текстом; при обрыве соединения клиентом - в любом режиме). Паника - одной записью `[ERROR]` в лог SDK.
  Кто разбирал панику по stderr - смотрите лог `[ERROR]` и Sentry.
- `HANDLER_TIMEOUT > 0`, запрос не уложился в таймаут: статус ответа внутри цепочки (`c.Writer.Status()` в
  `LoggerMiddleware`, `SentryMiddleware`, `MetricsMiddleware`, лог паники) после дедлайна всегда 503, даже если
  обработчик успел выставить свой код до дедлайна. Раньше - код обработчика. Лог, Sentry, метрики и ответ клиенту
  теперь совпадают. Метка `status` в метриках у запросов, превысивших таймаут, теперь 503 - как реально получил
  клиент; доля 5xx может вырасти - проверьте дашборды и алерты.
- HttpInfo лежит в контексте под ключом своего типа: `ctx.Value("http_info")` и `ctx.Value(constants.HttpInfoKey)`
  вернут `nil`. Читать - `gin.GetHttpInfoFromContext(ctx)`; в тестах класть - `gin.WithHttpInfo(ctx, info)`, а не
  `context.WithValue(ctx, "http_info", ...)`.
- Kernel кладёт AppInfo в контекст запроса через `context.WithAppInfo` из `gosdk-core` v1.1.0; нет AppInfo в контексте
  приложения - контекст запроса не дополняется. Код, читающий `ctx.Value("app_info")` напрямую, получит `nil` -
  читать `context.GetAppInfoFromContext(ctx)`.
- Поднимать вместе с `gosdk-core` v1.1.0 (`go get` поднимет его сам по `go.mod`). `gosdk-core` v1.1.0 со старым
  `gosdk-http-core` (≤ v1.1.0) теряет AppInfo: логи запросов и события Sentry - без имени сервиса.

## v1.1.0

Swagger включается переменной `SWAGGER_ENABLED` и по умолчанию выключен. Минимальная версия Go - 1.27.3.
Сигнатуры не изменились.

### Добавлено
- `config.HttpConfig.SwaggerEnabled`, env `SWAGGER_ENABLED`: включают `true` или `1` (регистр и пробелы по краям
  не важны). Не задана, `false`, `0` и любое другое значение - Swagger выключен.
- `InitRouter` при старте пишет строку `[INFO]`: `swagger enabled: SWAGGER_ENABLED="..."` или
  `swagger disabled: SWAGGER_ENABLED="..."`. Нераспознанное значение (`yes`, `on`, опечатка) - дополнительно
  `[WARN]`: `unrecognized SWAGGER_ENABLED="...", swagger disabled; use true/1 or false/0`.

### Исправлено
- `InitRouter` с `HANDLER_TIMEOUT > 0`: обработчик, который не уложился в таймаут, больше не делит `gin.Context`
  с другими запросами. Раньше `github.com/vearne/gin-timeout` v0.2.3 отвечал по таймауту и возвращал контекст
  в пул gin, пока обработчик продолжал с ним работать: гонка данных (`go test -race`), чужие параметры пути
  у обработчика. Таймаут теперь в самом SDK, зависимость `github.com/vearne/gin-timeout` удалена.
- Ответ таймаута прежний: 503, тело `{"code": -1, "msg":"http: Handler timeout"}`, `Content-Type:
  text/plain; charset=utf-8`, приходит клиенту по истечении `HANDLER_TIMEOUT`; контекст запроса обработчика
  отменяется по дедлайну. Ответ обработчика после дедлайна клиенту не уходит, кроме уже начатого потокового
  ответа (см. ниже). Запросы, уложившиеся в таймаут, отвечают как gin без `HANDLER_TIMEOUT`; отличия от прежнего -
  в «⚠️ Изменения поведения».

### Изменено
- `go.mod`: `go 1.26.0` → `go 1.27.3`. Причина - уязвимости стандартной библиотеки go1.26.0
  (`govulncheck`), исправлены в новых версиях Go. На go1.27.3 `govulncheck` уязвимостей стандартной библиотеки не находит.
- `gosdk-core` поднят до v1.0.29 (переход на Go 1.27.3, API не изменился).

### ⚠️ Изменения поведения
- Swagger по умолчанию выключен на любом окружении - dev, stage и прод. `APP_ENV` на это не влияет. После
  обновления у сервиса без `SWAGGER_ENABLED` пропадают `/<prefix>/index.html`, `/<prefix>/doc.json` и статика UI:
  ответ 404 `{"code":"PAGE_NOT_FOUND","message":"404 page not found"}`.
- Вернуть Swagger - `SWAGGER_ENABLED=true` (или `1`) в env сервиса, пересборка не нужна.
- На проде включать только вместе с закрытием пути `/<prefix>/` на ingress/шлюзе: SDK документацию
  аутентификацией не закрывает, со `SWAGGER_ENABLED=true` схема всех эндпойнтов доступна любому.
- Тесты сервиса, которые ждут Swagger от `InitRouter` (200 на `/<prefix>/index.html`, конфликт маршрута с
  префиксом), - добавить `SwaggerEnabled: "true"` в `config.HttpConfig`. При выключенном Swagger свой маршрут на
  `/<prefix>/...` регистрируется без конфликта.
- `HANDLER_TIMEOUT > 0`, запрос не уложился в таймаут: в ответе таймаута добавился заголовок `Connection: close`, HTTP/1.1
  соединение закрывается после завершения обработчика, следующий запрос клиент шлёт в новом соединении.
- Такой запрос в метриках Prometheus (`ginHelpers_request_duration_seconds`, `code="503"`) учитывается после
  завершения обработчика: длительность - время работы обработчика, а не `HANDLER_TIMEOUT`. Код ответа в
  метриках прежний - 503.
- `HANDLER_TIMEOUT > 0`, код ответа после отправки заголовков (`AbortWithStatus`, `WriteHeaderNow`, первая запись
  тела) больше не меняется - как в gin без таймаута. Пример: preflight `CorsMiddleware` (`AbortWithStatus(204)`) под
  `FormattedResponseMiddleware` раньше отвечал 200 и `{"success":true,"data":null}`, теперь 204 без тела. Запись
  тела при коде 1xx, 204, 304 возвращает обработчику `http.ErrBodyNotAllowed`, как в gin без таймаута.
- `HANDLER_TIMEOUT > 0`, потоковый ответ (`c.Stream`, `c.SSEvent` + `c.Writer.Flush()`): после первого `Flush` ответ
  идёт клиенту сразу, со статусом и заголовками обработчика (`Content-Type: text/event-stream`), как без таймаута.
  Раньше события приходили сразу, но со статусом 200 и `Content-Type: text/plain`. Таймаут у начатого потока только
  отменяет контекст обработчика: 503 в поток не пишется, что обработчик успеет записать, уходит клиенту.
- Потребителю нужен Go ≥ 1.27.3: после `go get` этой версии в его `go.mod` станет `go 1.27.3`.
  С `GOTOOLCHAIN=auto` toolchain скачается сам; с `GOTOOLCHAIN=local` на Go < 1.27.3 или в образе
  без доступа к `proxy.golang.org` сборка упадёт - образ сборки `golang:1.27.3`.

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
