package gin

import (
	"context"
	"fmt"
	"strings"

	"github.com/exgamer/gosdk-core/pkg/logger"
)

// swaggerEnabled Включён ли маршрут Swagger по значению SWAGGER_ENABLED.
// Включают только true/1 (без регистра, пробелы по краям обрезаются); всё прочее — выключен.
// recognized=false — непустое значение вне true/1/false/0.
func swaggerEnabled(setting string) (enabled bool, recognized bool) {
	switch strings.ToLower(strings.TrimSpace(setting)) {
	case "true", "1":
		return true, true
	case "false", "0", "":
		return false, true
	default:
		return false, false
	}
}

// logSwaggerState Пишет при старте состояние Swagger; нераспознанное значение — ещё WARN.
// context.Background(): у InitRouter нет ctx, вызов при старте вне запроса.
func logSwaggerState(setting string, enabled, recognized bool) {
	ctx := context.Background()

	if !recognized {
		logger.Warning(ctx, fmt.Sprintf("unrecognized SWAGGER_ENABLED=%q, swagger disabled; use true/1 or false/0", setting))
	}

	state := "disabled"
	if enabled {
		state = "enabled"
	}

	logger.Info(ctx, fmt.Sprintf("swagger %s: SWAGGER_ENABLED=%q", state, setting))
}
