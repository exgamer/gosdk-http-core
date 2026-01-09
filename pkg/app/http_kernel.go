package app

import (
	"context"
	"fmt"
	"github.com/exgamer/gosdk-core/pkg/app"
	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/exgamer/gosdk-core/pkg/di"
	"github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	ginHelper "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/exgamer/gosdk-http-core/pkg/metrics"
	"github.com/getsentry/sentry-go"
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

		logger.Dump(httpConfig)

		di.Register(a.Container, m.HttpConfig)
	}
	// Инициализация сентри
	if m.HttpConfig.SentryDsn != "" {
		if err := sentry.Init(sentry.ClientOptions{
			AttachStacktrace: true,
			TracesSampleRate: 1.0,
			Dsn:              m.HttpConfig.SentryDsn,
		}); err != nil {
			return err
		}
	}

	m.Router = ginHelper.InitRouter(a.BaseConfig, m.HttpConfig)

	di.Register(a.Container, m.Router)

	appConfig, err := app.GetBaseConfig(a)

	if err != nil {
		return err
	}

	metricsCollector := metrics.NewCollector(appConfig.Name)

	di.Register(a.Container, metricsCollector)

	m.Server = &http.Server{
		Addr:    m.HttpConfig.ServerAddress,
		Handler: m.Router, // <-- gin как handler
		//@TODO возможно вынести в настройки
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

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
	err := m.Server.Shutdown(ctx)
	_ = sentry.Flush(2 * time.Second)

	return err
}
