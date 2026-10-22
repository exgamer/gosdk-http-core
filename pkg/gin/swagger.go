package gin

// swaggerEnabled Включён ли маршрут Swagger по значению SWAGGER_ENABLED (разбор — parseBoolSetting).
func swaggerEnabled(setting string) (enabled bool, recognized bool) {
	return parseBoolSetting(setting)
}

// logSwaggerState Пишет при старте состояние Swagger; нераспознанное значение — ещё WARN.
func logSwaggerState(setting string, enabled, recognized bool) {
	logBoolSetting("swagger", "SWAGGER_ENABLED", setting, enabled, recognized)
}
