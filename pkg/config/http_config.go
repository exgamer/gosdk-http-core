package config

// HttpConfig Http конфиг
type HttpConfig struct {
	SwaggerPrefix  string `mapstructure:"SWAGGER_PREFIX" json:"swagger_prefix"`
	ServerAddress  string `mapstructure:"SERVER_ADDRESS" json:"server_address"`
	SentryDsn      string `mapstructure:"SENTRY_DSN"    json:"sentry_dsn"`
	HandlerTimeout int    `mapstructure:"HANDLER_TIMEOUT"    json:"handler_timeout"`
}
