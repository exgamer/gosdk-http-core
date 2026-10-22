package config

// HttpConfig Http конфиг
type HttpConfig struct {
	SwaggerPrefix      string `mapstructure:"SWAGGER_PREFIX"       json:"swagger_prefix"`
	ServerAddress      string `mapstructure:"SERVER_ADDRESS"       json:"server_address"`
	HandlerTimeout     int    `mapstructure:"HANDLER_TIMEOUT"      json:"handler_timeout"`
	CorsAllowedOrigins string `mapstructure:"CORS_ALLOWED_ORIGINS" json:"cors_allowed_origins"`

	// SwaggerEnabled Включают true/1 (без учёта регистра, пробелы по краям обрезаются);
	// не задана, false/0 и любое другое значение — Swagger выключен.
	SwaggerEnabled string `mapstructure:"SWAGGER_ENABLED" json:"swagger_enabled"`

	// Таймауты http.Server, секунды. 0 (не задано) — значение по умолчанию
	// (15 / 10 / 30 / 60), -1 — без таймаута.
	ServerReadTimeoutSec       int `mapstructure:"SERVER_READ_TIMEOUT_SEC"        json:"server_read_timeout_sec"`
	ServerReadHeaderTimeoutSec int `mapstructure:"SERVER_READ_HEADER_TIMEOUT_SEC" json:"server_read_header_timeout_sec"`
	ServerWriteTimeoutSec      int `mapstructure:"SERVER_WRITE_TIMEOUT_SEC"       json:"server_write_timeout_sec"`
	ServerIdleTimeoutSec       int `mapstructure:"SERVER_IDLE_TIMEOUT_SEC"        json:"server_idle_timeout_sec"`
}
