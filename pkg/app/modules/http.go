package modules

import (
	"context"
	"github.com/davecgh/go-spew/spew"
	"github.com/exgamer/gosdk-core/pkg/app"
	baseConfig "github.com/exgamer/gosdk-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	ginHelper "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
)

type HttpModule struct {
	HttpConfig *config.HttpConfig
	Router     *gin.Engine
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
	{
		if err := sentry.Init(sentry.ClientOptions{
			AttachStacktrace: true,
			TracesSampleRate: 1.0,
			Dsn:              m.HttpConfig.SentryDsn,
		}); err != nil {
			return err
		}
	}

	m.Router = ginHelper.InitRouter(a.BaseConfig, m.HttpConfig)

	return nil
}

func (m *HttpModule) Start(ctx context.Context) error {
	//запускаем сервер
	gErr := m.Router.Run(m.HttpConfig.ServerAddress)

	if gErr != nil {
		return gErr
	}

	return nil
}

func (m *HttpModule) Stop(ctx context.Context) error {
	return nil
}
