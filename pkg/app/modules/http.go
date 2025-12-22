package modules

import (
	"context"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/exgamer/gosdk-core/pkg/app"
	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	ginHelper "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"time"
)

type HttpModule struct {
	HttpConfig            *config.HttpConfig
	Router                *gin.Engine
	Server                *http.Server
	PrepareComponentsFunc func(app *app.App, module *HttpModule) error
}

func (m *HttpModule) Name() string {
	return "http"
}

func (m *HttpModule) Register(a *app.App) error {
	{
		httpConfig := &config.HttpConfig{}
		err := baseConfig.InitConfig(httpConfig)

		if err != nil {
			return err
		}

		spew.Dump(httpConfig)

		m.HttpConfig = httpConfig
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

	if m.PrepareComponentsFunc != nil {
		if err := m.PrepareComponentsFunc(a, m); err != nil {
			return err
		}
	}

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

func (m *HttpModule) Start(a *app.App) error {
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

func (m *HttpModule) Stop(ctx context.Context) error {
	if m.Server == nil {
		return nil
	}

	// если ctx без дедлайна, App уже даёт timeout — ок
	err := m.Server.Shutdown(ctx)
	_ = sentry.Flush(2 * time.Second)

	return err
}
