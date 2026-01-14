##  Что доступно из DI после инициализации приложения

Http роутер:

```go
router, err := di.GetRouter(c *di.Container) (*gin.Engine, error)
```

Http конфиг приложения:

```go
httpConfig := di.GetHttpConfig(c *di.Container) (*config.HttpConfig, error)
```

Сборщик метрик:

metColl := di.GetMetricsCollector(c *di.Container) (*metrics.Collector, error)