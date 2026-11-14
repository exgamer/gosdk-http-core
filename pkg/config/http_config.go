package config

// HttpConfig Http конфиг
type HttpConfig struct {
	SwaggerPrefix      string `mapstructure:"SWAGGER_PREFIX"       json:"swagger_prefix"`
	ServerAddress      string `mapstructure:"SERVER_ADDRESS"       json:"server_address"`
	HandlerTimeout     int    `mapstructure:"HANDLER_TIMEOUT"      json:"handler_timeout"`
	CorsAllowedOrigins string `mapstructure:"CORS_ALLOWED_ORIGINS" json:"cors_allowed_origins"`

	// CorsExtraAllowedHeaders Дополнительные заголовки для Access-Control-Allow-Headers (через запятую), например
	// X-CSRF-Token. Добавляются к базовому списку; не задано — ответ CORS прежний.
	CorsExtraAllowedHeaders string `mapstructure:"CORS_EXTRA_ALLOWED_HEADERS" json:"cors_extra_allowed_headers"`

	// SwaggerEnabled Включают true/1 (без учёта регистра, пробелы по краям обрезаются);
	// не задана, false/0 и любое другое значение — Swagger выключен.
	SwaggerEnabled string `mapstructure:"SWAGGER_ENABLED" json:"swagger_enabled"`

	// ErrorDetails Полная ошибка (текст паники и стек) в ответе 500. Включают true/1 (без учёта регистра,
	// пробелы по краям обрезаются); не задана, false/0 и любое другое значение — выключено. На проде не задавать.
	ErrorDetails string `mapstructure:"HTTP_ERROR_DETAILS" json:"http_error_details"`

	// MetricsEnabled Метрики ginHelpers_* и маршрут /metrics на роутере. false/0 — выключены; не задана и любое другое
	// значение — включены (как раньше). Выключают, если /metrics не должен быть на публичном адресе сервиса.
	MetricsEnabled string `mapstructure:"METRICS_ENABLED" json:"metrics_enabled"`

	// MetricsURLLabel Метка url метрик ginHelpers_*: route — шаблон маршрута (c.FullPath(), «/items/:id»;
	// несматченный — «unmatched»), число серий ограничено; не задана и любое другое значение — путь запроса, как раньше.
	MetricsURLLabel string `mapstructure:"METRICS_URL_LABEL" json:"metrics_url_label"`

	// Таймауты http.Server, секунды. 0 (не задано) — значение по умолчанию
	// (15 / 10 / 30 / 60), -1 — без таймаута.
	ServerReadTimeoutSec       int `mapstructure:"SERVER_READ_TIMEOUT_SEC"        json:"server_read_timeout_sec"`
	ServerReadHeaderTimeoutSec int `mapstructure:"SERVER_READ_HEADER_TIMEOUT_SEC" json:"server_read_header_timeout_sec"`
	ServerWriteTimeoutSec      int `mapstructure:"SERVER_WRITE_TIMEOUT_SEC"       json:"server_write_timeout_sec"`
	ServerIdleTimeoutSec       int `mapstructure:"SERVER_IDLE_TIMEOUT_SEC"        json:"server_idle_timeout_sec"`
}
