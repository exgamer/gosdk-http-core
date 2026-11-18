package gin

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	constants2 "github.com/exgamer/gosdk-core/pkg/constants"
	"github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/go-errors/errors"
	"github.com/google/uuid"
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

	// Доверенные прокси для ClientIP (X-Forwarded-For, X-Real-IP): только частные сети и loopback — ingress и
	// балансировщик внутри кластера. По умолчанию gin доверяет любому адресу, и клиент подделывает свой IP заголовком.
	// Сервис за прокси с публичным адресом задаёт свой список: router.SetTrustedProxies(...) после InitRouter.
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		logger.Warning(context.Background(), fmt.Sprintf("trusted proxies not set: %v", err))
	}

	// nosniff: браузер не угадывает тип ответа по содержимому (JSON с HTML внутри не исполняется как страница).
	router.Use(noSniff)

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
		if !metricsPathLabel(httpConfig.MetricsURLLabel) {
			p.ReqCntURLLabelMappingFn = routeLabel
		}

		p.Use(router)
	} else {
		logger.Info(context.Background(), fmt.Sprintf("metrics disabled: METRICS_ENABLED=%q (ginHelpers metrics and /metrics off)", httpConfig.MetricsEnabled))
	}
	//router.Use(gin.Logger())
	if httpConfig.HandlerTimeout > 0 {
		router.Use(handlerTimeout(time.Duration(httpConfig.HandlerTimeout)*time.Second, baseConfig.Name))
	}

	details, recognized := parseBoolSetting(httpConfig.ErrorDetails)
	logBoolSetting("error details", "HTTP_ERROR_DETAILS", httpConfig.ErrorDetails, details, recognized)

	if details {
		router.Use(func(c *gin.Context) {
			c.Set(errorDetailsKey, true)
			c.Next()
		})
	}

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

// trustedProxies Частные сети, loopback и 100.64.0.0/10 (адреса подов в части CNI): адреса ingress и балансировщика
// внутри кластера.
var trustedProxies = []string{
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8", "100.64.0.0/10", "::1", "fc00::/7",
}

// noSniff Заголовок X-Content-Type-Options: nosniff на каждый ответ.
func noSniff(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Next()
}

// errorDetailsKey Ключ gin.Context: HTTP_ERROR_DETAILS включён (ставит InitRouter).
const errorDetailsKey = "gosdk.http_error_details"

// ErrorDetailsEnabled Включён ли HTTP_ERROR_DETAILS для запроса (роутер из InitRouter): тогда в ответе 5xx — текст
// ошибки и details; иначе — только текст статуса. Без InitRouter — выключено.
func ErrorDetailsEnabled(c *gin.Context) bool {
	return c.GetBool(errorDetailsKey)
}

// metricsPathLabel Метка url метрик — путь запроса только при явном METRICS_URL_LABEL=path (без учёта регистра);
// не задана, route и прочее — шаблон маршрута. Нераспознанное значение — WARN при старте.
func metricsPathLabel(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "path":
		return true
	case "", "route":
		return false
	default:
		logger.Warning(context.Background(), fmt.Sprintf("unrecognized METRICS_URL_LABEL=%q, metrics url label is the route template; use route or path", value))

		return false
	}
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

// logPanic Пишет [correlation_id,METHOD,path,status] panic: <текст>\n<стек>. Путь — без query,
// заголовки и тело не пишутся. Нет HttpInfo (gateway без RequestInfoMiddleware) — метод и путь из запроса.
func logPanic(c *gin.Context, goErr *errors.Error) {
	ctx := c.Request.Context()
	method := c.Request.Method
	path := c.Request.URL.Path
	requestID := ""

	if httpInfo := GetHttpInfoFromContext(ctx); httpInfo != nil {
		method = httpInfo.RequestMethod
		path = httpInfo.RequestUrl
		requestID = httpInfo.CorrelationId
	}

	stack := strings.TrimSuffix(string(goErr.Stack()), "\n")
	message := "panic: " + goErr.Error() + "\n" + stack

	httpLogger.FormattedError(ctx, method, path, c.Writer.Status(), requestID, message)
}

// requestIdPattern Допустимый входящий id корреляции: UUID, ULID, hex trace id, base64 — до 128 символов.
var requestIdPattern = regexp.MustCompile(`^[A-Za-z0-9._:/+=-]{1,128}$`)

// IsValidRequestId Входящий id корреляции принимается как есть; иначе сервис генерирует свой.
func IsValidRequestId(id string) bool {
	return requestIdPattern.MatchString(id)
}

// incomingCorrelationHeaders Заголовки входящего id корреляции по приоритету.
var incomingCorrelationHeaders = []string{
	constants.CorrelationIdHeaderName,
	constants.XRequestIdHeaderName,
	constants.RequestIdHeaderName, //nolint:staticcheck // SA1019: прежний заголовок принимается на входе
}

// IncomingCorrelationID Id корреляции входящего запроса: первый допустимый (IsValidRequestId) из X-Correlation-Id,
// X-Request-Id, Request-Id; нет — "".
func IncomingCorrelationID(h http.Header) string {
	for _, name := range incomingCorrelationHeaders {
		if id := h.Get(name); IsValidRequestId(id) {
			return id
		}
	}

	return ""
}

// resolveCorrelationID Id корреляции запроса: входящий (IncomingCorrelationID) или новый UUID. Записывается
// в заголовки запроса X-Correlation-Id и Request-Id — следующие middleware берут тот же id.
func resolveCorrelationID(c *gin.Context) string {
	id := IncomingCorrelationID(c.Request.Header)
	if id == "" {
		id = uuid.New().String()
	}

	c.Request.Header.Set(constants.CorrelationIdHeaderName, id)
	c.Request.Header.Set(constants.RequestIdHeaderName, id) //nolint:staticcheck // SA1019: прежний заголовок — для кода сервисов, читающего его

	return id
}

func GetInstanceHttpInfo(c *gin.Context) *config.HttpInfo {
	httpInfo := &config.HttpInfo{}
	// id корреляции: X-Correlation-Id → X-Request-Id → Request-Id; не пришёл или не по формату (мусор, переносы
	// строк, слишком длинный) — генерим свой: он уходит в логи, ответы и исходящие запросы
	httpInfo.CorrelationId = resolveCorrelationID(c)
	httpInfo.RequestId = httpInfo.CorrelationId //nolint:staticcheck // SA1019: прежнее поле заполняется тем же id

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
