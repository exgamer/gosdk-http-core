# HTTP Formatted Logger (wrapper over global stdlib logger)

Этот модуль даёт **FormattedInfo/FormattedError** (HTTP-формат) поверх вашего **глобального логгера** на `log.Logger`.


## HTTP-форматтер

## Использование

### Форматированный HTTP лог

```go
logger.FormattedInfo(
    "catalog-service-go",
    "GET",
    "/v1/items",
    200,
    "req-123",
    "ok",
)
```

### Лог из `AppInfo/HttpInfo`

```go
httpfmt.FormattedLogWithAppInfo(appInfo, httpInfo, "validation failed")
httpfmt.FormattedErrorWithAppInfo(appInfo, httpInfo, "db error")
```

---

## Почему так лучше

- нет зависимости вида `package logger` + импорт `.../logger` (путаница и циклы)
- один backend логгера на весь проект
- форматирование HTTP-логов живёт отдельно и легко меняется
- легко тестировать (подмена на buffer)

---

## Лицензия

MIT
