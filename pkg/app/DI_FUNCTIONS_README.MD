##  Что доступно из DI после инициализации приложения

Http роутер:

```go
router, err := app.GetRouter(a *app.App) (*gin.Engine, error)
```

Http конфиг приложения:

```go
httpConfig := app.GetHttpConfig(a *app.App) (*config.HttpConfig, error)
```

