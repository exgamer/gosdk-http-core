package middleware

import (
	"errors"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
	gin2 "github.com/exgamer/gosdk-http-core/pkg/gin"
	"github.com/exgamer/gosdk-http-core/pkg/http/helpers"
	"github.com/gin-gonic/gin"
	"net/http"
	"slices"
)

// CompanyMiddleware Middleware дял компаний
func CompanyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		appInfo := gin2.GetAppInfo(c)

		if appInfo.CompanyId == 0 {
			c.Abort()
			helpers.AppExceptionResponse(c, exception.NewAppException(http.StatusForbidden, errors.New("company id not found"), nil))
			helpers.FormattedResponse(c)

			return
		}

		if appInfo.CurrentCompanyId == 0 {
			c.Abort()
			helpers.AppExceptionResponse(c, exception.NewAppException(http.StatusForbidden, errors.New("current company id not found"), nil))
			helpers.FormattedResponse(c)

			return
		}

		if !slices.Contains(appInfo.CompanyIds, appInfo.CompanyId) {
			c.Abort()
			helpers.AppExceptionResponse(c, exception.NewAppException(http.StatusForbidden, errors.New("company not found in vendor companies"), nil))
			helpers.FormattedResponse(c)

			return
		}

		if !slices.Contains(appInfo.CompanyIds, appInfo.CurrentCompanyId) {
			c.Abort()
			helpers.AppExceptionResponse(c, exception.NewAppException(http.StatusForbidden, errors.New("current company not found in vendor companies"), nil))
			helpers.FormattedResponse(c)

			return
		}

		c.Next()
	}
}
