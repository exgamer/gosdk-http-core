package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/exgamer/gosdk-core/pkg/debug"
	"github.com/exgamer/gosdk-core/pkg/helpers"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"net/http"
)

func FormattedTextErrorResponse(c *gin.Context, statusCode int, message string, context map[string]any) {
	TextErrorResponse(c, statusCode, message, context)
	FormattedResponse(c)
}

func TextErrorResponse(c *gin.Context, statusCode int, message string, context map[string]any) {
	AppExceptionResponse(c, exception.NewHttpException(statusCode, errors.New(message), context))
}

func FormattedErrorResponse(c *gin.Context, statusCode int, err error, context map[string]any) {
	ErrorResponse(c, statusCode, err, context)
	FormattedResponse(c)
}

func ErrorResponse(c *gin.Context, statusCode int, err error, context map[string]any) {
	AppExceptionResponse(c, exception.NewHttpException(statusCode, err, context))
}

func ErrorResponseUntrackableSentry(c *gin.Context, statusCode int, err error, context map[string]any) {
	AppExceptionResponse(c, exception.NewUntrackableAppException(statusCode, err, context))
}

func FormattedAppExceptionResponse(c *gin.Context, exception *exception.HttpException) {
	AppExceptionResponse(c, exception)
	FormattedResponse(c)
}

func AppExceptionResponse(c *gin.Context, exception *exception.HttpException) {
	c.Set("exception", exception)
	c.AbortWithStatus(exception.Code)
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

		if dbg := debug.GetDebugFromContext(c.Request.Context()); dbg != nil {
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

	appException := exception.HttpException{}
	mapstructure.Decode(appExceptionObject, &appException)
	serviceName := "UNKNOWN (maybe you not used RequestMiddleware)"
	requestId := "UNKNOWN (maybe you not used RequestMiddleware)"
	appInfo := helpers.GetAppInfoFromContext(c.Request.Context())

	if appInfo != nil {
		serviceName = appInfo.ServiceName
	}

	httpInfo := gin2.GetHttpInfoFromContext(c.Request.Context())

	if exists {
		requestId = httpInfo.RequestId
	}

	responseData := gin.H{
		"status":     appException.Code,
		"error":      appException.GetErrorType(),
		"message":    appException.Error.Error(),
		"request_id": requestId,
		"hostname":   serviceName,
		"details":    appException.Context,
	}

	var response interface{}

	if dbg := debug.GetDebugFromContext(c.Request.Context()); dbg != nil {
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

	c.Data(appException.Code, "application/json", jsonBytes)
}
