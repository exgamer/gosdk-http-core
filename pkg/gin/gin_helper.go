package gin

import (
	"encoding/json"
	"fmt"
	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	constants2 "github.com/exgamer/gosdk-core/pkg/constants"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	"github.com/exgamer/gosdk-http-core/pkg/gin/validation"
	"github.com/exgamer/gosdk-http-core/pkg/helpers"
	"github.com/exgamer/gosdk-http-core/pkg/middleware"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-errors/errors"
	"github.com/go-playground/validator/v10"
	"github.com/iancoleman/strcase"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	timeout "github.com/vearne/gin-timeout"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"net/http"
	"strconv"
	"time"
)

// InitRouter Базовая инициализация gin
func InitRouter(baseConfig *baseConfig.BaseConfig, httpConfig *config.HttpConfig) *gin.Engine {
	if baseConfig.AppEnv == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Options
	router := gin.New()
	prefix := httpConfig.SwaggerPrefix

	if prefix == "" {
		prefix = baseConfig.Name
	}

	if prefix == "" {
		prefix = "swagger"
	}

	router.GET("/"+prefix+"/api-docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"code": "PAGE_NOT_FOUND", "message": "404 page not found"})
	})
	router.HandleMethodNotAllowed = true
	p := ginprometheus.NewPrometheus("ginHelpers")
	p.Use(router)
	router.Use(middleware.RequestInfoMiddleware(baseConfig))
	router.Use(sentrygin.New(sentrygin.Options{}))
	//router.Use(gin.Logger())
	if httpConfig.HandlerTimeout > 0 {
		router.Use(timeout.Timeout(timeout.WithTimeout(time.Duration(httpConfig.HandlerTimeout) * time.Second)))
	}

	router.Use(gin.CustomRecovery(ErrorHandler))

	return router
}

// ErrorHandler Обработчик ошибок gin
func ErrorHandler(c *gin.Context, err any) {
	goErr := errors.Wrap(err, 2)
	details := make([]string, 0)

	for _, frame := range goErr.StackFrames() {
		details = append(details, frame.String())
	}

	sentry.CaptureException(goErr)
	c.JSON(http.StatusInternalServerError, gin.H{"message": goErr.Error(), "details": details, "success": false, "service_code": 0})
}

func Error(c *gin.Context, exception *exception.HttpException) {
	c.Set("exception", exception)
	c.Status(exception.Code)
}

func Success(c *gin.Context, data any) {
	c.Set("data", data)
}

func SetAppInfo(c *gin.Context, baseConfig *baseConfig.BaseConfig) {
	c.Set(constants2.AppInfoKey, getInstanceAppInfo(baseConfig))
}

func SetHttpInfo(c *gin.Context) {
	c.Set(constants.HttpInfoKey, getInstanceHttpInfo(c))
}

func getInstanceAppInfo(appConfig *baseConfig.BaseConfig) *baseConfig.AppInfo {
	appInfo := &baseConfig.AppInfo{}
	appInfo.AppEnv = "UNKNOWN (maybe you not used RequestMiddleware)"
	appInfo.ServiceName = "UNKNOWN (maybe you not used RequestMiddleware)"

	if appConfig != nil {
		appInfo.AppEnv = appConfig.AppEnv
		appInfo.ServiceName = appConfig.Name
	}

	return appInfo
}

func getInstanceHttpInfo(c *gin.Context) *config.HttpInfo {
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

	httpInfo.CacheControl = c.GetHeader(constants.CacheControlHeaderName)
	httpInfo.RequestUrl = c.Request.URL.Path
	httpInfo.RequestMethod = c.Request.Method
	httpInfo.RequestScheme = c.Request.URL.Scheme
	httpInfo.RequestHost = c.Request.Host

	return httpInfo
}

//func GetContext(c *gin.Context) context.Context {
//	return context.WithValue(c.Request.Context(), debug.DebugKey, debug.GetDebugCollectorFromGinContext(c))
//}

func GetAppInfoFromGinContext(c *gin.Context) *baseConfig.AppInfo {
	if dbg, ok := c.Request.Context().Value(constants2.AppInfoKey).(*baseConfig.AppInfo); ok {
		return dbg
	}

	return nil
}

func GetHttpInfoFromGinContext(c *gin.Context) *baseConfig.AppInfo {
	if dbg, ok := c.Request.Context().Value(constants.HttpInfoKey).(*baseConfig.AppInfo); ok {
		return dbg
	}

	return nil
}

func GetAppInfo(c *gin.Context) *baseConfig.AppInfo {
	value, exists := c.Get(constants2.AppInfoKey)

	if exists {
		appInfo := value.(*baseConfig.AppInfo)

		return appInfo
	}

	return getInstanceAppInfo(nil)
}

func GetHttpInfo(c *gin.Context) *config.HttpInfo {
	value, exists := c.Get(constants.HttpInfoKey)

	if exists {
		httpInfo := value.(*config.HttpInfo)

		return httpInfo
	}

	return getInstanceHttpInfo(c)
}

// ValidateRequestQuery - Валидация GET параметров HTTP реквеста
func ValidateRequestQuery(c *gin.Context, request validation.IRequest) bool {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		for n, f := range request.CustomValidationRules() {
			v.RegisterValidation(n, f)
		}
	}

	if err := c.BindQuery(request); err != nil {
		var ve validator.ValidationErrors

		if errors.As(err, &ve) {
			out := make(map[string]any, len(ve))

			for _, fe := range ve {
				msg := request.CustomValidationMessage(fe)

				if msg == fe.Tag() {
					msg = request.ValidationMessage(fe)
				}

				out[strcase.ToSnake(fe.Field())] = msg
			}

			helpers.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("validation error"), out)

			return false
		}

		// Обработка ошибок unmarshal
		var unmarshalTypeError *json.UnmarshalTypeError

		if errors.As(err, &unmarshalTypeError) {
			out := make(map[string]any)
			out[strcase.ToSnake(unmarshalTypeError.Field)] = fmt.Sprintf("Invalid type expected %s but got %s", unmarshalTypeError.Type, unmarshalTypeError.Value)
			helpers.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("validation error"), out)

			return false
		}

		// Обработка ошибок unmarshal
		var parseNumTypeError *strconv.NumError

		if errors.As(err, &parseNumTypeError) {
			out := make(map[string]any)
			out[strcase.ToSnake(parseNumTypeError.Num)] = parseNumTypeError.Error()
			helpers.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, parseNumTypeError, out)

			return false
		}

		helpers.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("incorrect request body"), nil)

		return false
	}

	return true
}

// ValidateRequestBody - Валидация тела HTTP реквеста
func ValidateRequestBody(c *gin.Context, request validation.IRequest) bool {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		for n, f := range request.CustomValidationRules() {
			v.RegisterValidation(n, f)
		}
	}

	if err := c.ShouldBind(&request); err != nil {
		var ve validator.ValidationErrors

		if errors.As(err, &ve) {
			out := make(map[string]any, len(ve))

			for _, fe := range ve {
				msg := request.CustomValidationMessage(fe)

				if msg == fe.Tag() {
					msg = request.ValidationMessage(fe)
				}

				out[strcase.ToSnake(fe.Field())] = msg
			}

			helpers.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("validation error"), out)

			return false
		}

		// Обработка ошибок unmarshal
		var unmarshalTypeError *json.UnmarshalTypeError

		if errors.As(err, &unmarshalTypeError) {
			out := make(map[string]any)
			out[strcase.ToSnake(unmarshalTypeError.Field)] = fmt.Sprintf("Invalid type expected %s but got %s", unmarshalTypeError.Type, unmarshalTypeError.Value)

			helpers.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("validation error"), out)

			return false
		}

		// Обработка ошибок syntax
		var syntaxTypeError *json.SyntaxError

		if errors.As(err, &syntaxTypeError) {
			helpers.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("incorrect request body"), nil)

			return false
		}

		helpers.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("incorrect request body"), nil)

		return false
	}

	return true
}
