package gin

import (
	"context"
	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	constants2 "github.com/exgamer/gosdk-core/pkg/constants"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"github.com/go-errors/errors"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	timeout "github.com/vearne/gin-timeout"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"net/http"
	"time"
)

// InitRouter Базовая инициализация gin
func InitRouter(baseConfig *baseConfig.BaseConfig, httpConfig *config.HttpConfig) *gin.Engine {
	if !baseConfig.Debug {
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

	router.GET("/"+prefix+"/api-docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"code": "PAGE_NOT_FOUND", "message": "404 page not found"})
	})
	router.HandleMethodNotAllowed = true
	p := ginprometheus.NewPrometheus("ginHelpers")
	p.Use(router)
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
	c.Set(constants2.AppInfoKey, GetInstanceAppInfo(baseConfig))
}

func SetHttpInfo(c *gin.Context) {
	c.Set(constants.HttpInfoKey, GetInstanceHttpInfo(c))
}

func GetInstanceAppInfo(appConfig *baseConfig.BaseConfig) *baseConfig.AppInfo {
	appInfo := &baseConfig.AppInfo{}
	appInfo.AppEnv = "UNKNOWN (maybe you not used RequestMiddleware)"
	appInfo.ServiceName = "UNKNOWN (maybe you not used RequestMiddleware)"

	if appConfig != nil {
		appInfo.AppEnv = appConfig.AppEnv
		appInfo.ServiceName = appConfig.Name
		appInfo.DebugMode = appConfig.Debug
	}

	return appInfo
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

	httpInfo.CacheControl = c.GetHeader(constants.CacheControlHeaderName)
	httpInfo.RequestUrl = c.Request.URL.Path
	httpInfo.RequestMethod = c.Request.Method
	httpInfo.RequestScheme = c.Request.URL.Scheme
	httpInfo.RequestHost = c.Request.Host

	return httpInfo
}

func GetAppInfoFromContext(ctx context.Context) *baseConfig.AppInfo {
	if v := ctx.Value(constants2.AppInfoKey); v != nil {
		if ai, ok := v.(*baseConfig.AppInfo); ok {
			return ai
		}
	}
	return nil
}

func GetHttpInfoFromContext(ctx context.Context) *config.HttpInfo {
	if v := ctx.Value(constants.HttpInfoKey); v != nil {
		if hi, ok := v.(*config.HttpInfo); ok {
			return hi
		}
	}
	return nil
}
