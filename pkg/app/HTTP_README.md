## Инициализация приложения V2

<hr style="border: 1px solid orange;"/>


### В app.go

```go
package app

import (
	app "github.com/exgamer/gosdk-http-core/pkg/app/v2"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/database"
	redis2 "github.com/exgamer/gosdk-http-core/pkg/database/redis"
	"github.com/exgamer/gosdk-http-core/pkg/di"
	"jmart/banner-service/cmd/app/docs"
	"jmart/banner-service/internal/factories"
	"jmart/banner-service/internal/routes"
)

func InitApp() *app.App {
	instance := app.NewApp()
	instance.PrepareConfigsFunc = prepareConfigs()
	instance.PrepareComponentsFunc = prepareComponentsFunc()
	instance.PrepareHttpFunc = prepareHttp()

	return instance
}

func prepareConfigs() func(app *app.App) error {
	return func(app *app.App) error {
		dbConfig, err := database.InitDbConfig()

		if err != nil {
			return err
		}

		di.Register(app.Container, dbConfig)  // ложим в сервис контейнер

		redisConfig, err := redis2.InitRedisConfig()

		if err != nil {
			return err
		}

		di.Register(app.Container, redisConfig) // ложим в сервис контейнер
		
		docs.SwaggerInfo.Host = app.BaseConfig.ServerAddress

		return nil
	}
}

func prepareComponentsFunc() func(app *app.App) error {
	return func(app *app.App) error {
		dbConfig, err := di.Resolve[*config.DbConfig](app.Container) // достаем из  сервис контейнера

		if err != nil {
			return err
		}

		dbClient, err := database.InitMysqlGormConnection(dbConfig)

		if err != nil {
			return err
		}

		redisConfig, err := di.Resolve[*config.RedisConfig](app.Container) // достаем из  сервис контейнера

		if err != nil {
			return err
		}

		redisClient, err := redis2.InitRedisClient(redisConfig)

		if err != nil {
			return err
		}

		redisRepositoryFactory := factories.NewRedisRepositoryFactory(redisClient)
		mysqlRepositoryFactory := factories.NewMysqlRepositoryFactory(dbClient)
		entityManagerFactory := factories.NewEntityManagerFactory(mysqlRepositoryFactory)
		servicesFactory := factories.NewServiceFactory(mysqlRepositoryFactory, redisRepositoryFactory, entityManagerFactory)
		di.Register(app.Container, servicesFactory) // регаем то что нужно будет достать в другом месте, например для регистрации в http хендлере нужены будут сервисы

		return nil
	}
}

func prepareHttp() func(app *app.App) error {
	return func(app *app.App) error {
		servicesFactory, err := di.Resolve[*factories.ServicesFactory](app.Container) // достаем из  сервис контейнера

		if err != nil {
			return err
		}

		handlersFactory := factories.NewHandlersFactory(servicesFactory)
		routes.SetRoutes(app, handlersFactory, app.TraceClient)

		return nil
	}
}



```


### В main.go

```go
func main() {
	// Init App
	appInstance := app.NewApp()
	err := appInstance.RunHttp()

	if err != nil {
		log.Fatalf("App Init error: %s", err)
	}
}


```