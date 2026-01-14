package di

import (
	"github.com/exgamer/gosdk-core/pkg/di"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/metrics"
	"github.com/gin-gonic/gin"
)

// GetRouter возвращает HTTP router.
func GetRouter(c *di.Container) (*gin.Engine, error) {
	r, err := di.Resolve[*gin.Engine](c)

	if err != nil {
		return nil, err
	}

	return r, nil
}

// GetHttpConfig возвращает HTTP Config.
func GetHttpConfig(c *di.Container) (*config.HttpConfig, error) {
	h, err := di.Resolve[*config.HttpConfig](c)

	if err != nil {
		return nil, err
	}

	return h, nil
}

// GetMetricsCollector возвращает MetricsCollector.
func GetMetricsCollector(c *di.Container) (*metrics.Collector, error) {
	m, err := di.Resolve[*metrics.Collector](c)

	if err != nil {
		return nil, err
	}

	return m, nil
}
