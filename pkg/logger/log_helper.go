package logger

import (
	"github.com/exgamer/gosdk-core/pkg/config"
	"github.com/exgamer/gosdk-core/pkg/logger"
	config2 "github.com/exgamer/gosdk-http-core/pkg/config"
	"strconv"
	"strings"
)

// FormattedInfo Форматированный лог
func FormattedInfo(serviceName string, method string, uri string, status int, requestId string, message string) {
	logger.Info(getFormattedMessage(serviceName, method, uri, status, requestId, message))
}

// FormattedError Форматированный лог ошибки
func FormattedError(serviceName string, method string, uri string, status int, requestId string, message string) {
	logger.Error(getFormattedMessage(serviceName, method, uri, status, requestId, message))
}

// FormattedLogWithAppInfo Форматированный лог для RequestData
func FormattedLogWithAppInfo(appInfo *config.AppInfo, httpInfo *config2.HttpInfo, message string) {
	FormattedInfo(appInfo.ServiceName, httpInfo.RequestMethod, httpInfo.RequestUrl, 0, httpInfo.RequestId, message)
}

// FormattedErrorWithAppInfo Форматированный лог ошибки для RequestData
func FormattedErrorWithAppInfo(appInfo *config.AppInfo, httpInfo *config2.HttpInfo, message string) {
	FormattedError(appInfo.ServiceName, httpInfo.RequestMethod, httpInfo.RequestUrl, 1, httpInfo.RequestId, message)
}

func getFormattedMessage(serviceName string, method string, uri string, status int, requestId string, message string) string {
	messageBuilder := strings.Builder{}
	messageBuilder.WriteString("[" + serviceName + "," + requestId + "]")
	messageBuilder.WriteString("[" + method + "," + uri + "," + strconv.Itoa(status) + "]")
	messageBuilder.WriteString(" " + message)

	return messageBuilder.String()
}
