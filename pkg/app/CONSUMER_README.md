## Инициализация приложения V2

<hr style="border: 1px solid orange;"/>


### В app.go

```go
package app

import (
	"gitlab.almanit.kz/jmart/go-rest-template/internal/consumers"
	"gitlab.almanit.kz/jmart/go-rest-template/internal/factories"
	"gitlab.almanit.kz/jmart/go-rest-template/internal/routes"
	"github.com/exgamer/gosdk-http-core/pkg/app/v2"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/di"

	"github.com/exgamer/gosdk-http-core/pkg/console"
	"github.com/exgamer/gosdk-http-core/pkg/database"
	"github.com/exgamer/gosdk-http-core/pkg/rabbitmq"
	structures2 "github.com/exgamer/gosdk-http-core/pkg/rabbitmq/structures"
)

func InitApp() *app.App {
	instance := app.NewApp()
	instance.PrepareConfigsFunc = prepareConfigs()
	instance.PrepareComponentsFunc = prepareComponentsFunc()
	instance.PrepareConsumerFunc = prepareConsumer()
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

		rabbitConfig, err := rabbitmq.InitRabbitConfig()

		if err != nil {
			return err
		}

		di.Register(app.Container, rabbitConfig)  // ложим в сервис контейнер

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

		rabbitConfig, err := di.Resolve[*structures2.RabbitConfig](app.Container) // достаем из  сервис контейнера

		if err != nil {
			return err
		}

		rabbitClient, rErr := initRabbitClient(rabbitConfig)

		if rErr != nil {
			return rErr
		}

		app.AmqpClient = rabbitClient

		//Создание фабрик для компонентов проекта
		mysqlRepositoryFactory := factories.NewMysqlRepositoryFactory(dbClient)
		servicesFactory := factories.NewServiceFactory(mysqlRepositoryFactory)
		consumersFactory := factories.NewConsumersFactory(servicesFactory, console.GetInstanceAppInfo(app.BaseConfig))
		di.Register(app.Container, consumersFactory) // регаем то что нужно будет достать в другом месте, например для регистрации в консуьюмерах хендлере нужены будут сервисы

		return nil
	}
}

func prepareHttp() func(app *app.App) error {
	return func(app *app.App) error {
		routes.SetRoutes(app)

		return nil
	}
}

func prepareConsumer() func(app *app.App) error {
	return func(app *app.App) error {
		consumersFactory, err := di.Resolve[*factories.ConsumersFactory](app.Container)

		if err != nil {
			return err
		}

		if err := app.AmqpClient.RegisterMultipleHandler(consumers.GetConsumers(consumersFactory, app.AmqpClient)); err != nil {
			return err
		}

		return nil
	}
}

func initRabbitClient(rabbitConfig *structures2.RabbitConfig) (*rabbitmq.AmqpPubSub, error) {
	amqClient, err := rabbitmq.NewAmqpPubSubByUriAndVhost(rabbitConfig.Host, rabbitConfig.VHost)

	if err != nil {
		return nil, err
	}

	return amqClient, nil
}



```


### В main.go

```go
func main() {
    // Init App
    appInstance := app.InitApp()
    
    go func() {
        if err := appInstance.RunHttp(); err != nil {
        log.Fatalf("Run Http error: %s", err)
    }
    }()
    
    if err := appInstance.RunConsumer(); err != nil {
        log.Fatal(err)
    }
}


```