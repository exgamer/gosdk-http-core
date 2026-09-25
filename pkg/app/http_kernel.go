package app

import (
	"context"
	"fmt"
	"github.com/exgamer/gosdk-core/pkg/app"
	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	coreConstants "github.com/exgamer/gosdk-core/pkg/constants"
	"github.com/exgamer/gosdk-core/pkg/di"
	"github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	ginHelper "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/exgamer/gosdk-http-core/pkg/metrics"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"time"
)

const HttpKernelName = "http"

type HttpKernel struct {
	HttpConfig *config.HttpConfig
	Router     *gin.Engine
	Server     *http.Server
}

func (m *HttpKernel) Name() string {
	return HttpKernelName
}

func (m *HttpKernel) Init(a *app.App) error {
	{
		httpConfig := &config.HttpConfig{}
		err := baseConfig.InitConfig(httpConfig)

		if err != nil {
			return err
		}

		m.HttpConfig = httpConfig

		logger.Dump(a.GetContext(), httpConfig)

		di.Register(a.Container, m.HttpConfig)
	}

	m.Router = ginHelper.InitRouter(a.BaseConfig, m.HttpConfig)

	m.Router.Use(withAppInfo(a.GetContext))

	di.Register(a.Container, m.Router)

	appConfig, err := di.GetBaseConfig(a.Container)

	if err != nil {
		return err
	}

	metricsCollector := metrics.NewCollector(appConfig.Name)

	di.Register(a.Container, metricsCollector)

	m.Server = newServer(m.HttpConfig, m.Router)

	return nil
}

func (m *HttpKernel) Start(a *app.App) error {
	go func() {
		if err := m.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			if a != nil {
				a.Fail(fmt.Errorf("http server: %w", err))

				return
			}
			log.Printf("http server error: %v", err)
		}
	}()

	return nil
}

func (m *HttpKernel) Stop(ctx context.Context) error {
	if m.Server == nil {
		return nil
	}

	// если ctx без дедлайна, App уже даёт timeout — ок
	// flush sentry-события на shutdown - зона ответственности SentryKernel (gosdk-sentry-core)
	return m.Server.Shutdown(ctx)
}

// withAppInfo дополняет собственный контекст запроса AppInfo приложения
// (его читают логгер и ответы с ошибкой). Контекст запроса не подменяется
// контекстом приложения: тот отменяется по SIGTERM раньше, чем Shutdown
// дождётся текущих запросов, и они падали бы с context canceled. Отмена при
// уходе клиента сохраняется — записи, которые обязаны завершиться, сервис
// отвязывает сам: context.WithoutCancel(ctx).
func withAppInfo(appCtx func() context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		appInfo := appCtx().Value(coreConstants.AppInfoKey)
		//lint:ignore SA1029 gosdk-core читает AppInfo по этому строковому ключу
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), coreConstants.AppInfoKey, appInfo))

		c.Next()
	}
}

// Таймауты сервера по умолчанию — если SERVER_*_TIMEOUT_SEC не заданы.
const (
	defaultReadTimeout       = 15 * time.Second
	defaultReadHeaderTimeout = 10 * time.Second
	defaultWriteTimeout      = 30 * time.Second
	defaultIdleTimeout       = 60 * time.Second
)

func newServer(cfg *config.HttpConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.ServerAddress,
		Handler:           handler,
		ReadTimeout:       timeoutOrDefault(cfg.ServerReadTimeoutSec, defaultReadTimeout),
		ReadHeaderTimeout: timeoutOrDefault(cfg.ServerReadHeaderTimeoutSec, defaultReadHeaderTimeout),
		WriteTimeout:      timeoutOrDefault(cfg.ServerWriteTimeoutSec, defaultWriteTimeout),
		IdleTimeout:       timeoutOrDefault(cfg.ServerIdleTimeoutSec, defaultIdleTimeout),
	}
}

// timeoutOrDefault: 0 — def, -1 (любое отрицательное) — без таймаута.
func timeoutOrDefault(sec int, def time.Duration) time.Duration {
	switch {
	case sec < 0:
		return 0
	case sec == 0:
		return def
	default:
		return time.Duration(sec) * time.Second
	}
}
