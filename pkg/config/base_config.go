package config

// BaseConfig Основной конфиг приложения
type BaseConfig struct {
	Name           string `mapstructure:"APP_NAME" json:"app_name"`
	SwaggerPrefix  string `mapstructure:"SWAGGER_PREFIX" json:"swagger_prefix"`
	ContainerName  string `mapstructure:"CONTAINER_NAME" json:"container_name"`
	AppEnv         string `mapstructure:"APP_ENV"    json:"app_env"`
	Version        string `mapstructure:"APP_VERSION" json:"app_version"`
	ServerAddress  string `mapstructure:"SERVER_ADDRESS" json:"server_address"`
	SentryDsn      string `mapstructure:"SENTRY_DSN"    json:"sentry_dsn"`
	TimeZone       string `mapstructure:"TIMEZONE"    json:"timezone"`
	HandlerTimeout int    `mapstructure:"HANDLER_TIMEOUT"    json:"handler_timeout"`
	Debug          bool   `mapstructure:"DEBUG"    json:"debug"`
	GrpcSrvPort    string `mapstructure:"GRPC_SERVER_PORT"`
}
