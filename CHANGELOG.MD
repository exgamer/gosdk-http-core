# Changelog

## v1.11.0

Безопасное поведение по умолчанию: меняются ответ 5xx, разбор тела `ValidateRequestBody`, ответ таймаута, доверие
`X-Forwarded-For` и метка метрик по умолчанию. Сигнатуры не изменились, новых переменных окружения нет; прежнее
поведение метрик — `METRICS_URL_LABEL=path`, текст ошибки 5xx — `HTTP_ERROR_DETAILS=true` (только локально).

### Изменено поведение
- ⚠️ Клиенты, читающие `request_id` в теле ошибок, переходят на `correlation_id`; Swagger сервисов (`docs/docs.go`)
  перегенерировать (`swag init`): структуры `structures.*` описывают `correlation_id`.
- Id запроса стал сквозным id корреляции. Вход: `X-Correlation-Id` → `X-Request-Id` (ingress-nginx) → `Request-Id`
  (прежний) → новый UUID; раньше — только `Request-Id`. Ответ — заголовок `X-Correlation-Id` (раньше заголовка id в
  ответе не было), `Access-Control-Expose-Headers: X-Correlation-Id` (новый), `X-Correlation-Id` — в базовых
  `Access-Control-Allow-Headers`. Поле тела ошибок (`response.*`, ответ `HANDLER_TIMEOUT`, структуры Swagger
  `structures.*`) и данных события Sentry — `correlation_id` вместо `request_id`. Формат логов `[id,METHOD,path,status]`
  не меняется — внутри тот же id. `HttpInfo.RequestId` и `constants.RequestIdHeaderName` — Deprecated, `RequestId`
  заполняется тем же id.
- Ответ 5xx через `response.ErrorResponse`/`InternalServerError`/`ErrorResponseWithStatus` (и raw-варианты) отдавал
  клиенту текст внутренней ошибки (адрес и пользователь БД, SQL) и `details`. Теперь `message` — текст статуса
  (`Internal Server Error`, `Service Unavailable`), `details` — `null`; полная ошибка — в лог и Sentry, как раньше.
  С `HTTP_ERROR_DETAILS=true` — как раньше. 4xx не меняются.
- `ValidateRequestBody`: без `Content-Type` тело разбиралось из query и формы (`binding.Default`), поля `json:"-"`
  заполнялись по имени поля. Теперь без `Content-Type` и для `application/*+json` (`merge-patch+json`) — JSON (пустое
  тело без `Content-Type` — пустой объект); форма (`x-www-form-urlencoded`, `multipart/form-data`) — только у запроса с
  тегами `form`, иначе 415; прочие неизвестные типы — 415. `GET`, XML, YAML, TOML, protobuf, msgpack — как раньше.
- Тело `ValidateRequestBody` (кроме формы) читалось без предела. Теперь больше 10 МиБ — 413 `request body too large`
  (не 422). Загрузки файлов и роутер целиком предел не получают.
- Поля `json:"-"` без тега `form` больше не заполняются из query и формы (`ValidateRequestBody`, `ValidateRequestQuery`).
- Ответ `HANDLER_TIMEOUT` — 503 в формате ошибок SDK (`application/json`, `{"success":false,"data":{"status":503,
  "error":"service_unavailable","message":"Service Unavailable","correlation_id":...,"hostname":...,"details":null}}`) вместо
  `text/plain` `{"code": -1, "msg":"http: Handler timeout"}`.
- `InitRouter` доверял `X-Forwarded-For`/`X-Real-IP` от любого адреса: клиент подделывал `ClientIP()`. Теперь доверенные
  прокси — только частные сети, loopback и адреса подов в части CNI (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`,
  `127.0.0.0/8`, `100.64.0.0/10`, `::1`, `fc00::/7`); свой список — `router.SetTrustedProxies(...)` после `InitRouter`.
- Метка `url` метрик `ginHelpers_*` по умолчанию — шаблон маршрута (`/items/:id`), а не путь: число серий не растёт с
  id. Путь — `METRICS_URL_LABEL=path`; нераспознанное значение — шаблон и WARN при старте.
- `helpers.GetPagerRequest`: `per_page` больше 100 обрезается до 100 (`helpers.MaxPaginationPerPage`).

### Исправлено
- Пользовательские правила валидации регистрировались в общем валидаторе по имени: одно имя у разных запросов с
  разными функциями давало запросу чужое правило (зависело от порядка первых запросов, вложенные структуры — первого
  разбора). Теперь под именем — диспетчер по типу валидируемого запроса: каждый запрос проверяется своей функцией;
  запрос, который пользуется тегом, не объявив правило, получает функцию первого зарегистрировавшего, как раньше.
- Чтение тела шло под блокировкой правил: медленный клиент задерживал регистрацию правил новых запросов. Теперь тело
  читается до блокировки.
- CORS: `Request-Id` (заголовок SDK) не был в `Access-Control-Allow-Headers`, `Vary: Origin` затирал выставленный
  раньше `Vary`. Теперь `X-Correlation-Id` и `Request-Id` в базовом списке, `Vary` дополняется, ответ — с
  `Access-Control-Expose-Headers: X-Correlation-Id`.
- Debug-лог тела запроса писал секреты открытым текстом. Теперь ключи JSON на любой глубине и параметры формы
  маскируются по правилам заголовков и query; multipart и JSON, который не разбирается (обрезан по 1 МиБ), — только
  размер. Заголовок и ключ `Iin` маскируются (ИИН).
- Debug-поле ответа (`DebugCollector`) отдавало клиенту тела, заголовки и ответы исходящих запросов с секретами.
  Теперь чувствительные ключи и секреты в строковых `body` (JSON, форма) — `***`.

### Добавлено
- Ответы роутера `InitRouter` — с `X-Content-Type-Options: nosniff`.
- WARN при старте `HttpKernel`, если `HANDLER_TIMEOUT` не меньше срока записи сервера (`SERVER_WRITE_TIMEOUT_SEC`).
- `gin.ErrorDetailsEnabled(c)`, `gin.MaskJSONKeys`, `gin.MaskBodyString`, `helpers.MaxPaginationPerPage`.
- Сквозной CorrelationID: `RequestInfoMiddleware` и `HANDLER_TIMEOUT` кладут id корреляции запроса в контекст
  (`sdkctx.CorrelationID(c.Request.Context())`, gosdk-core) и в `HttpInfo.CorrelationId`; `gin.IncomingCorrelationID`,
  `constants.CorrelationIdHeaderName` (`X-Correlation-Id`), `constants.XRequestIdHeaderName`. Требуется gosdk-core v1.3.0.

### Сервис за `httputil.ReverseProxy` (gateway)
- Ответ апстрима на http-core приходит со своим `X-Correlation-Id`, а `ReverseProxy` добавляет заголовки апстрима к уже
  выставленным middleware прокси: клиент получает заголовок дважды. В `ModifyResponse` удалите заголовок апстрима
  (`resp.Header.Del("X-Correlation-Id")`) или не ставьте свой в ответ прокси. См. README, «Id корреляции».

## v1.10.0

Меняется ответ 422 на ошибку собственного `UnmarshalJSON` поля тела. Сигнатуры и переменные окружения не изменились.

### Исправлено
- `ValidateRequestBody` отвечал на ошибку собственного `UnmarshalJSON` поля (дата, enum, деньги) 422 `incorrect
  request body` без ключа поля: сервисам приходилось ставить ключ самим. Теперь — 422 `validation error` с ключом
  поля по пути JSON, как у ошибки типа (`VALIDATION_KEYS=json` — `period.endDay`, не задана — `period_end_day`), и
  текстом ошибки `UnmarshalJSON`. Поле ищется и во вложенных структурах, срезах структур и встроенных структурах.
  Валидное тело, ошибки типа и синтаксиса, данные после JSON-значения, пустое тело и тела не-JSON — как раньше.

## v1.9.0

Меняется CORS при `CORS_ALLOWED_ORIGINS=*` и разбор JSON-тела с данными после объекта. Сигнатуры и переменные
окружения не изменились; с явным списком origins ответ CORS прежний.

### Исправлено
- CORS при `CORS_ALLOWED_ORIGINS=*` отражал любой Origin вместе с `Access-Control-Allow-Credentials: true`: любой сайт
  мог делать запросы с cookie пользователя и читать ответ. Теперь при `*` Origin отражается (`Vary: Origin`) без
  `Access-Control-Allow-Credentials`; cookie и credentials — только для origins из явного списка. Origin `null` с
  credentials не разрешается, в том числе из явного списка. Запросы с `Authorization: Bearer` работают как раньше.
- `ValidateRequestBody` принимал JSON-тело с данными после первого значения (`{"a":1}{"a":2}`, `{"a":1}garbage`) по
  первому объекту. Теперь такое тело — 422 `incorrect request body`. Пробелы в конце, неизвестные поля, ошибки типов и
  правил — как раньше.

## v1.8.0

Меняется ответ на прерванный запрос без ответа и приём входящего `Request-Id`. Сигнатуры не изменились; сервисы,
которые прерывают запрос через `response.ErrorResponse`/`response.*`, работают как раньше.

### Исправлено
- `response.Formatted` (`FormattedResponseMiddleware`) отвечал `{"success": true, "data": null}` на запрос, прерванный
  без ответа в контексте: голый `c.Abort()` — со статусом 200, `c.AbortWithStatus(401)` — 401 с телом успеха. Теперь
  `AbortWithStatus(4xx/5xx)` — тело ошибки SDK с этим статусом, голый `c.Abort()` — 500 и ERROR в логе «request
  aborted without response» (`response.ErrAbortedWithoutResponse`), `AbortWithStatus(<400)` (CORS preflight 204) —
  без тела, как есть. Лог и Sentry по таким запросам — как раньше.
- Входящий `Request-Id` принимался любым и попадал в логи, ответы и исходящие запросы. Теперь принимается только
  1–128 символов `[A-Za-z0-9._:/+=-]` (UUID, ULID, hex trace id, base64), иначе — сгенерированный UUID, в том числе
  в заголовке запроса.

### Добавлено
- `gin.IsValidRequestId(id)` — проверка формата входящего `Request-Id`.

## v1.7.1

Зависимость `gosdk-core` поднята до v1.2.0. API пакета не изменился.

## v1.7.0

Исправления безопасности и порчи ответов, новые необязательные настройки. Обратно совместимо: сигнатуры не изменились,
без новых переменных поведение для клиентов прежнее.

### Исправлено
- `LoggerMiddleware` читал тело каждого запроса и подменял его первыми 1 МиБ: обработчик получал обрезанное тело
  больше 1 МиБ. Теперь тело читается только при уровне логов debug (в лог — до 1 МиБ), обработчик получает его целиком.
- `response.Formatted` дописывал JSON-конверт к телу, которое хендлер записал сам (файл, текст, `c.JSON`): ответ
  портился. Теперь записанное тело не трогается.
- Секреты в Sentry и debug-логе: маскировались только `Authorization`, `Cookie`, `token`, `access_token`. Теперь
  ещё `Proxy-Authorization`, `Set-Cookie`, `X-Api-Key`, `X-Auth-Token`, `password`, `refresh_token`, `id_token`,
  `client_secret`, `code`, `state` (OAuth/OIDC), ключи с окончанием `-token`/`_token`/`-secret`/`_secret`/`-key`/`_key`
  (кроме `Idempotency-Key`) и с `password`/`apikey` в имени; `SENSITIVE_KEYS` добавляет ключи, `SENSITIVE_KEYS_ALLOW` —
  снимает маскирование.
- `metrics.NewCollector` второй раз в процессе (второе приложение, тесты) падал с паникой регистрации: теперь берётся
  уже зарегистрированная гистограмма.

### Добавлено
- `response.NoContent(c, status)` — ответ без тела (204 по умолчанию) через `FormattedResponseMiddleware`.
- `gin.WithoutHandlerTimeout(method, path)` — маршрут без `HANDLER_TIMEOUT` (потоки, большие файлы; путь — полный
  шаблон с префиксом групп); `gin.CheckTimeoutExemptions(router)` — WARN о пути, которого нет в роутере (вызывает
  `HttpKernel.Start`); `gin.WriteDeadline(d)` — свой срок записи ответа маршрута.
- `gin.IsSensitiveKey`, `gin.MaskHeaders`, `gin.MaskQuery` — общий список чувствительных ключей; env `SENSITIVE_KEYS`.
- env `VALIDATION_KEYS=json` — ключи ошибок 422 по тегу источника (тело — `json`, query — `form`, путь — `uri`) и полный путь вложенного поля
  (`items[2].component_ids`); без неё ключи прежние (`component_i_ds`).
- env `METRICS_ENABLED=false` — без метрик `ginHelpers_*` и `/metrics`; env `METRICS_URL_LABEL=route` — метка `url`
  по шаблону маршрута.

## v1.6.1

Обрыв запроса клиентом под `HANDLER_TIMEOUT` больше не считается таймаутом. Сигнатуры не изменились, ответ по
дедлайну прежний.

### Исправлено
- `InitRouter` с `HANDLER_TIMEOUT`: ответ таймаута (503 `{"code": -1, "msg":"http: Handler timeout"}`, лог `[ERROR]`)
  отдавался на любую отмену контекста запроса, в том числе когда клиент закрыл соединение до дедлайна: обрыв клиентом
  попадал в лог как ошибка сервера и в метрики как 5xx. Теперь ответ таймаута — только по дедлайну
  (`context.DeadlineExceeded`). При обрыве клиентом (`context.Canceled`) статус — тот, что выставил обработчик, а если
  обработчик ничего не написал — 499 Client Closed Request: `LoggerMiddleware` пишет его `[WARN]`, в метриках — 499.

## v1.6.0

Настраиваемые дополнительные заголовки CORS. Обратно совместимо: сигнатуры не изменились, без новой переменной ответ
CORS прежний.

### Добавлено
- `config.HttpConfig.CorsExtraAllowedHeaders` (env `CORS_EXTRA_ALLOWED_HEADERS`) — дополнительные заголовки для
  `Access-Control-Allow-Headers` через запятую (например, `X-CSRF-Token` для SPA на другом домене с CSRF-защитой).
  Добавляются к базовому списку `Origin, Content-Type, Accept, Authorization, X-Request-Id`; пустые элементы и повторы
  (без учёта регистра) пропускаются. Не задана — заголовок ответа тот же, что в v1.5.0. Заголовки CORS по-прежнему
  отдаются только разрешённым origins из `CORS_ALLOWED_ORIGINS`.

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
