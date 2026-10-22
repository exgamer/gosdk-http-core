package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	coreConfig "github.com/exgamer/gosdk-core/pkg/config"
	coreContext "github.com/exgamer/gosdk-core/pkg/context"
	"github.com/gin-gonic/gin"

	"github.com/exgamer/gosdk-http-core/pkg/constants"
	"github.com/exgamer/gosdk-http-core/pkg/exception"
)

// SentryMiddleware finds HttpInfo and AppInfo under the new context keys: the event keeps request_id and hostname.
func TestSentryMiddleware_EventCarriesRequestIdAndServiceName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reporter := withReportReporter(t)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		ai := &coreConfig.AppInfo{ServiceName: "svc"}
		c.Request = c.Request.WithContext(coreContext.WithAppInfo(c.Request.Context(), ai))
		c.Next()
	})
	r.Use(RequestInfoMiddleware(nil), SentryMiddleware())
	r.GET("/x", func(c *gin.Context) {
		c.Set("exception", exception.NewInternalServerErrorException(errors.New("db down"), nil))
		c.AbortWithStatus(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(constants.RequestIdHeaderName, "rid-87")
	r.ServeHTTP(httptest.NewRecorder(), req)

	events := reporter.all()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}

	data, _ := events[0].opts.Extra["error"].(map[string]any)
	if data["request_id"] != "rid-87" || data["hostname"] != "svc" {
		t.Errorf("event error data = %v, want request_id rid-87 and hostname svc", data)
	}
}
