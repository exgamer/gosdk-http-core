package logger

import (
	"github.com/exgamer/gosdk-core/pkg/config"
	config2 "github.com/exgamer/gosdk-http-core/pkg/config"
	exception2 "github.com/exgamer/gosdk-http-core/pkg/exception"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// LogAppException лог AppException
func LogAppException(appException *exception2.HttpException) {
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	errorLog.Println(appException.Error.Error())
}

// FormattedInfo Форматированный лог
func FormattedInfo(serviceName string, method string, uri string, status int, requestId string, message string) {
	FormattedLog("INFO", serviceName, method, uri, status, requestId, message)
}

// FormattedError Форматированный лог ошибки
func FormattedError(serviceName string, method string, uri string, status int, requestId string, message string) {
	FormattedLog("ERROR", serviceName, method, uri, status, requestId, message)
}

// FormattedLog Форматированный лог
func FormattedLog(level string, serviceName string, method string, uri string, status int, requestId string, message string) {
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	messageBuilder := strings.Builder{}
	messageBuilder.WriteString(time.Now().Format("2006-01-02 15:04:05.345"))
	messageBuilder.WriteString(" " + level + " ")
	messageBuilder.WriteString("[" + serviceName + "," + requestId + "]")
	messageBuilder.WriteString("[" + method + "," + uri + "," + strconv.Itoa(status) + "]")
	messageBuilder.WriteString(" " + message)

	log.Println(messageBuilder.String())
	log.SetFlags(log.Ldate | log.Ltime)
}

// FormattedLogWithAppInfo Форматированный лог для RequestData
func FormattedLogWithAppInfo(appInfo *config.AppInfo, httpInfo *config2.HttpInfo, message string) {
	FormattedInfo(appInfo.ServiceName, httpInfo.RequestMethod, httpInfo.RequestUrl, 0, httpInfo.RequestId, message)
}

// FormattedErrorWithAppInfo Форматированный лог ошибки для RequestData
func FormattedErrorWithAppInfo(appInfo *config.AppInfo, httpInfo *config2.HttpInfo, message string) {
	FormattedInfo(appInfo.ServiceName, httpInfo.RequestMethod, httpInfo.RequestUrl, 1, httpInfo.RequestId, message)
}
