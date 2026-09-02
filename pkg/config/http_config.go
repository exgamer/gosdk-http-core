package config

// HttpConfig Http конфиг
type HttpConfig struct {
	SwaggerPrefix      string `mapstructure:"SWAGGER_PREFIX"       json:"swagger_prefix"`
	ServerAddress      string `mapstructure:"SERVER_ADDRESS"       json:"server_address"`
	HandlerTimeout     int    `mapstructure:"HANDLER_TIMEOUT"      json:"handler_timeout"`
	CorsAllowedOrigins string `mapstructure:"CORS_ALLOWED_ORIGINS" json:"cors_allowed_origins"`
}
