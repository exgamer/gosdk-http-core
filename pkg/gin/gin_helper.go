package gin

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	constants2 "github.com/exgamer/gosdk-core/pkg/constants"
	"github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/go-errors/errors"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	ginprometheus "github.com/zsais/go-gin-prometheus"

	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
	httpLogger "github.com/exgamer/gosdk-http-core/pkg/logger"
)

// panicMessage Текст ответа 500 при панике, когда детали ошибки выключены.
const panicMessage = "internal server error"

// InitRouter Базовая инициализация gin
func InitRouter(baseConfig *baseConfig.BaseConfig, httpConfig *config.HttpConfig) *gin.Engine {
	if !logger.IsDebugLevel() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	prefix := httpConfig.SwaggerPrefix

	if prefix == "" {
		prefix = baseConfig.Name
	}

	if prefix == "" {
		prefix = "swagger"
	}

	enabled, recognized := swaggerEnabled(httpConfig.SwaggerEnabled)
	logSwaggerState(httpConfig.SwaggerEnabled, enabled, recognized)

	if enabled {
		router.GET("/"+prefix+"/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"code": "PAGE_NOT_FOUND", "message": "404 page not found"})
	})
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"code": "METHOD_NOT_ALLOWED", "message": "405 method not allowed"})
	})
	router.HandleMethodNotAllowed = true
	// DisableBodyReading: метрики не читают тело запроса — размер берётся из Content-Length. Иначе middleware
	// метрик читает тело целиком (io.ReadAll) до обработчиков: в память без лимита размера и до дедлайнов сервиса
	if metricsEnabled(httpConfig.MetricsEnabled) {
		p := ginprometheus.NewWithConfig(ginprometheus.Config{Subsystem: "ginHelpers", DisableBodyReading: true})
		if strings.EqualFold(strings.TrimSpace(httpConfig.MetricsURLLabel), "route") {
			p.ReqCntURLLabelMappingFn = routeLabel
		}

		p.Use(router)
	} else {
		logger.Info(context.Background(), fmt.Sprintf("metrics disabled: METRICS_ENABLED=%q (ginHelpers metrics and /metrics off)", httpConfig.MetricsEnabled))
	}
	//router.Use(gin.Logger())
	if httpConfig.HandlerTimeout > 0 {
		router.Use(handlerTimeout(time.Duration(httpConfig.HandlerTimeout) * time.Second))
	}

	details, recognized := parseBoolSetting(httpConfig.ErrorDetails)
	logBoolSetting("error details", "HTTP_ERROR_DETAILS", httpConfig.ErrorDetails, details, recognized)

	// writer nil: gin не пишет дамп [Recovery] с заголовками запроса в stderr, паника — одной записью в лог SDK.
	router.Use(gin.CustomRecoveryWithWriter(nil, errorHandler(details)))

	return router
}

// metricsEnabled Метрики выключает только явное false/0 (без учёта регистра); не задано и прочее — включены.
func metricsEnabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false", "0":
		return false
	}

	return true
}

// routeLabel Метка url — шаблон маршрута: число серий не растёт с идентификаторами в пути.
func routeLabel(c *gin.Context) string {
	if route := c.FullPath(); route != "" {
		return route
	}

	return "unmatched"
}

// ErrorHandler Обработчик паники для gin.CustomRecovery без InitRouter (gateway): детали в ответе выключены.
func ErrorHandler(c *gin.Context, err any) {
	// Wrap — здесь, в функции, которую вызывает gin: от неё отсчитывается стек.
	goErr := errors.Wrap(err, 2)
	respondPanic(c, goErr, false)
}

// errorHandler Обработчик паники InitRouter с настройкой HTTP_ERROR_DETAILS, разобранной при старте.
func errorHandler(details bool) gin.RecoveryFunc {
	return func(c *gin.Context, err any) {
		goErr := errors.Wrap(err, 2)
		respondPanic(c, goErr, details)
	}
}

// respondPanic Одно событие в Sentry, ответ 500, одна запись [ERROR] в лог.
// Abort: после паники в middleware следующие обработчики цепочки не выполняются (как в gin.Recovery).
// Событие и лог — со статусом, который получит клиент (после таймаута обработчика — 503).
func respondPanic(c *gin.Context, goErr *errors.Error, details bool) {
	captureToSentry(c, goErr, panicStatus(c))
	c.AbortWithStatusJSON(http.StatusInternalServerError, panicBody(goErr, details))
	logPanic(c, goErr)
}

// panicStatus Ставит статус ответа паники до события Sentry и возвращает тот, что получит клиент.
// Writer не меняет уже отправленный статус и статус таймаута: тогда вернётся он, а не 500.
func panicStatus(c *gin.Context) int {
	if !c.Writer.Written() {
		c.Writer.WriteHeader(http.StatusInternalServerError)
	}

	return c.Writer.Status()
}

// panicBody Тело ответа 500. Без деталей — только константы: текст паники и стек не читаются.
func panicBody(goErr *errors.Error, details bool) gin.H {
	if !details {
		return gin.H{"message": panicMessage, "details": []string{}, "success": false, "service_code": 0}
	}

	frames := make([]string, 0)
	for _, frame := range goErr.StackFrames() {
		frames = append(frames, frame.String())
	}

	return gin.H{"message": goErr.Error(), "details": frames, "success": false, "service_code": 0}
}

// logPanic Пишет [request_id,METHOD,path,status] panic: <текст>\n<стек>. Путь — без query,
// заголовки и тело не пишутся. Нет HttpInfo (gateway без RequestInfoMiddleware) — метод и путь из запроса.
func logPanic(c *gin.Context, goErr *errors.Error) {
	ctx := c.Request.Context()
	method := c.Request.Method
	path := c.Request.URL.Path
	requestID := ""

	if httpInfo := GetHttpInfoFromContext(ctx); httpInfo != nil {
		method = httpInfo.RequestMethod
		path = httpInfo.RequestUrl
		requestID = httpInfo.RequestId
	}

	stack := strings.TrimSuffix(string(goErr.Stack()), "\n")
	message := "panic: " + goErr.Error() + "\n" + stack

	httpLogger.FormattedError(ctx, method, path, c.Writer.Status(), requestID, message)
}

func GetInstanceHttpInfo(c *gin.Context) *config.HttpInfo {
	httpInfo := &config.HttpInfo{}
	httpInfo.RequestId = c.GetHeader(constants.RequestIdHeaderName)
	// если request id не пришел с заголовком, генерим его, чтобы прокидывать дальше при http запросах
	if httpInfo.RequestId == "" {
		httpInfo.GenerateRequestId()
		c.Request.Header.Add(constants.RequestIdHeaderName, httpInfo.RequestId)
	}

	httpInfo.LanguageCode = c.GetHeader(constants.LanguageHeaderName)

	if httpInfo.LanguageCode == "" {
		httpInfo.LanguageCode = constants2.LangCodeRu
	}

	httpInfo.RequestOrigin = c.GetHeader(constants.OriginHeaderName)
	httpInfo.CacheControl = c.GetHeader(constants.CacheControlHeaderName)
	httpInfo.RequestUrl = c.Request.URL.Path
	httpInfo.RequestMethod = c.Request.Method
	httpInfo.RequestScheme = c.Request.URL.Scheme
	httpInfo.RequestHost = c.Request.Host

	return httpInfo
}
