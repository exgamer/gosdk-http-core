package middleware

import (
	"fmt"
	"github.com/exgamer/gosdk-http-core/pkg/clicker"
	"github.com/exgamer/gosdk-http-core/pkg/rabbitmq"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// CollectDataClick -  http мидлвар для записи в кролик а в последующем запись в кликхаус
// требуется пробросить туда клиента кролика
func CollectDataClick(rabbitClient *rabbitmq.AmqpPubSub) gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now()

		c.Next()

		if rabbitClient == nil {
			return
		}

		if c.Request.Method != http.MethodPatch &&
			c.Request.Method != http.MethodPost && c.Request.Method != http.MethodPut &&
			c.Request.Method != http.MethodDelete {
			return
		}

		data := clicker.Collect(c, now)

		if data.Error != nil {
			fmt.Println("error with CollectDataClick : ", data.Error.Error())
		}

		pubCfg := rabbitClient.NewPublisherFanoutDurableConfig(
			"request",
			"jpost-log-service")

		if err := rabbitClient.Publish(data.LogItem, pubCfg...); err != nil {
			fmt.Println("error with publish date : ", err.Error())
		}
	}
}
