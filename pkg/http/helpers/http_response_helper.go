package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/debug"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"net/http"
	"strconv"
)

func FormattedTextErrorResponse(c *gin.Context, statusCode int, message string, context map[string]any) {
	TextErrorResponse(c, statusCode, message, context)
	FormattedResponse(c)
}

func TextErrorResponse(c *gin.Context, statusCode int, message string, context map[string]any) {
	AppExceptionResponse(c, exception.NewAppException(statusCode, errors.New(message), context))
}

func FormattedErrorResponse(c *gin.Context, statusCode int, err error, context map[string]any) {
	ErrorResponse(c, statusCode, err, context)
	FormattedResponse(c)
}

func ErrorResponse(c *gin.Context, statusCode int, err error, context map[string]any) {
	AppExceptionResponse(c, exception.NewAppException(statusCode, err, context))
}

func ErrorResponseUntrackableSentry(c *gin.Context, statusCode int, err error, context map[string]any) {
	AppExceptionResponse(c, exception.NewUntrackableAppException(statusCode, err, context))
}

func FormattedAppExceptionResponse(c *gin.Context, exception *exception.AppException) {
	AppExceptionResponse(c, exception)
	FormattedResponse(c)
}

func AppExceptionResponse(c *gin.Context, exception *exception.AppException) {
	c.Set("exception", exception)
	c.Status(exception.Code)
}

func SuccessResponse(c *gin.Context, data any) {
	c.Set("data", data)
}

func SuccessCreatedResponse(c *gin.Context, data any) {
	c.Set("data", data)
	c.Set("status_code", http.StatusCreated)
}

func SuccessDeletedResponse(c *gin.Context, data any) {
	c.Set("data", data)
	c.Set("status_code", http.StatusNoContent)
}

func FormattedSuccessResponse(c *gin.Context, data any) {
	SuccessResponse(c, data)
	FormattedResponse(c)
}

func FormattedResponse(c *gin.Context) {
	appExceptionObject, exists := c.Get("exception")

	if appExceptionObject != nil {
		fmt.Printf("%+v\n", appExceptionObject)
	}

	if !exists {
		data, _ := c.Get("data")
		var response interface{}

		if dbg := debug.GetDebugCollectorFromGinContext(c); dbg != nil {
			dbg.CalculateTotalTime()
			response = struct {
				Success bool        `json:"success"`
				Data    interface{} `json:"data"`
				Debug   interface{} `json:"debug"`
			}{
				true,
				data,
				dbg,
			}
		} else {
			response = struct {
				Success bool        `json:"success"`
				Data    interface{} `json:"data"`
			}{
				true,
				data,
			}
		}

		jsonBytes, err := json.Marshal(response)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal JSON"})

			return
		}

		c.Writer.Status()
		statusCode, ex := c.Get("status_code")

		if !ex {
			c.Data(http.StatusOK, "application/json", jsonBytes)
		} else {
			c.Data(statusCode.(int), "application/json", jsonBytes)
		}

		return
	}

	appException := exception.AppException{}
	mapstructure.Decode(appExceptionObject, &appException)
	fmt.Printf("%+v\n", appException)
	serviceName := "UNKNOWN (maybe you not used RequestMiddleware)"
	requestId := "UNKNOWN (maybe you not used RequestMiddleware)"
	value, exists := c.Get("app_info")

	if exists {
		appInfo := value.(*config.AppInfo)
		serviceName = appInfo.ServiceName
		requestId = appInfo.RequestId
	}

	responseData := gin.H{
		"status":       appException.Code,
		"error":        appException.GetErrorType(),
		"message":      appException.Error.Error(),
		"request_id":   requestId,
		"hostname":     serviceName,
		"service_code": appException.ServiceCode,
		"details":      appException.Context,
	}

	var response interface{}

	if dbg := debug.GetDebugCollectorFromGinContext(c); dbg != nil {
		dbg.CalculateTotalTime()
		response = struct {
			Success bool        `json:"success"`
			Data    interface{} `json:"data"`
			Debug   interface{} `json:"debug"`
		}{
			false,
			responseData,
			dbg,
		}
	} else {
		response = struct {
			Success bool        `json:"success"`
			Data    interface{} `json:"data"`
		}{
			false,
			responseData,
		}
	}

	jsonBytes, err := json.Marshal(response)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal JSON"})

		return
	}

	if !appException.TrackInSentry {
		c.Data(appException.Code, "application/json", jsonBytes)

		return
	}

	sentry.WithScope(func(scope *sentry.Scope) {
		// Добавляем заголовки запроса
		mapHeaders := make(map[string]any)
		for key, values := range c.Request.Header {
			for _, value := range values {
				mapHeaders[fmt.Sprintf("header_%s", key)] = value
			}
		}
		scope.SetContext("header", mapHeaders)

		// Добавляем Query параметры
		mapQueries := make(map[string]any)
		for key, values := range c.Request.URL.Query() {
			for _, value := range values {
				mapQueries[fmt.Sprintf("query_%s", key)] = value
			}
		}
		scope.SetContext("query", mapQueries)

		if appException.Code >= 400 && appException.Code < 500 {
			scope.SetLevel(sentry.LevelWarning)
		} else {
			scope.SetLevel(sentry.LevelError)
		}

		// Захватываем ошибку
		scope.SetContext("error", responseData)

		sentry.CaptureMessage("Http Status Code  - " + strconv.Itoa(appException.Code) + ". Url - " + c.Request.URL.Path)
	})

	c.Data(appException.Code, "application/json", jsonBytes)
}
