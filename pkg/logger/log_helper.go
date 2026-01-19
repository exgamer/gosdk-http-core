package logger

import (
	"context"
	"github.com/exgamer/gosdk-core/pkg/logger"
	config2 "github.com/exgamer/gosdk-http-core/pkg/config"
	"strconv"
	"strings"
)

// FormattedInfo Форматированный лог
func FormattedInfo(ctx context.Context, method string, uri string, status int, requestId string, message string) {
	logger.Info(ctx, getFormattedMessage(method, uri, status, requestId, message))
}

// FormattedError Форматированный лог ошибки
func FormattedError(ctx context.Context, method string, uri string, status int, requestId string, message string) {
	logger.Error(ctx, getFormattedMessage(method, uri, status, requestId, message))
}

// FormattedLogWithHttpInfo Форматированный лог для RequestData
func FormattedLogWithHttpInfo(ctx context.Context, httpInfo *config2.HttpInfo, message string) {
	FormattedInfo(ctx, httpInfo.RequestMethod, httpInfo.RequestUrl, 0, httpInfo.RequestId, message)
}

// FormattedErrorWithHttpInfo Форматированный лог ошибки для RequestData
func FormattedErrorWithHttpInfo(ctx context.Context, httpInfo *config2.HttpInfo, message string) {
	FormattedError(ctx, httpInfo.RequestMethod, httpInfo.RequestUrl, 1, httpInfo.RequestId, message)
}

func getFormattedMessage(method string, uri string, status int, requestId string, message string) string {
	messageBuilder := strings.Builder{}
	messageBuilder.WriteString("[" + requestId + "," + method + "," + uri + "," + strconv.Itoa(status) + "]")
	messageBuilder.WriteString(" " + message)

	return messageBuilder.String()
}
