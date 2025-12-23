package app

import (
	"github.com/exgamer/gosdk-core/pkg/app"
	"github.com/exgamer/gosdk-core/pkg/di"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/gin-gonic/gin"
)

// GetRouter возвращает HTTP router.
func GetRouter(a *app.App) (*gin.Engine, error) {
	c, err := di.Resolve[*gin.Engine](a.Container)

	if err != nil {
		return nil, err
	}

	return c, nil
}

// GetHttpConfig возвращает HTTP Config.
func GetHttpConfig(a *app.App) (*config.HttpConfig, error) {
	c, err := di.Resolve[*config.HttpConfig](a.Container)

	if err != nil {
		return nil, err
	}

	return c, nil
}
