package gin

import (
	"context"
	"fmt"
	"strings"

	"github.com/exgamer/gosdk-core/pkg/logger"
)

// parseBoolSetting Разбор настройки-переключателя (SWAGGER_ENABLED, HTTP_ERROR_DETAILS).
// Включают только true/1 (без учёта регистра, пробелы по краям обрезаются); false/0 и пусто — выключено.
// recognized=false — непустое значение вне true/1/false/0, тоже выключено.
// strconv.ParseBool не подходит: он принимает t/T/f/F.
func parseBoolSetting(setting string) (enabled bool, recognized bool) {
	switch strings.ToLower(strings.TrimSpace(setting)) {
	case "true", "1":
		return true, true
	case "false", "0", "":
		return false, true
	default:
		return false, false
	}
}

// logBoolSetting Пишет при старте состояние настройки; нераспознанное значение — ещё WARN.
// context.Background(): у InitRouter нет ctx, вызов при старте вне запроса.
func logBoolSetting(feature, env, setting string, enabled, recognized bool) {
	ctx := context.Background()

	if !recognized {
		logger.Warning(ctx, fmt.Sprintf("unrecognized %s=%q, %s disabled; use true/1 or false/0", env, setting, feature))
	}

	state := "disabled"
	if enabled {
		state = "enabled"
	}

	logger.Info(ctx, fmt.Sprintf("%s %s: %s=%q", feature, state, env, setting))
}
