package validators

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/exgamer/gosdk-http-core/pkg/gin/validation"
	"github.com/exgamer/gosdk-http-core/pkg/middleware"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var stationCodeFormat = regexp.MustCompile(`^[a-z0-9_-]+$`)

type stationPath struct {
	validation.Request
	Code string `uri:"code" binding:"required,max=8,http_core_station_code"`
	Page int    `uri:"page"`
}

func (r *stationPath) CustomValidationRules() map[string]validator.Func {
	return map[string]validator.Func{"http_core_station_code": func(fl validator.FieldLevel) bool {
		return stationCodeFormat.MatchString(fl.Field().String())
	}}
}

func (r *stationPath) CustomValidationMessage(fe validator.FieldError) string {
	if fe.Tag() == "http_core_station_code" {
		return "invalid station code"
	}

	return fe.Tag()
}

func servePath(t *testing.T, path string) (*httptest.ResponseRecorder, *stationPath, []error) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	var got *stationPath
	var ginErrors []error

	engine := gin.New()
	engine.Use(middleware.FormattedResponseMiddleware())
	engine.GET("/stations/:code/pages/:page", func(c *gin.Context) {
		req := &stationPath{}
		defer func() {
			for _, e := range c.Errors {
				ginErrors = append(ginErrors, e)
			}
		}()

		if !ValidateRequestPath(c, req) {
			return
		}

		got = req
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	return rec, got, ginErrors
}

func TestValidateRequestPath_Valid(t *testing.T) {
	rec, got, ginErrors := servePath(t, "/stations/st_01/pages/3")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}

	if got == nil || got.Code != "st_01" || got.Page != 3 {
		t.Fatalf("bound path = %+v", got)
	}

	if len(ginErrors) != 0 {
		t.Fatalf("gin errors = %v", ginErrors)
	}
}

func TestValidateRequestPath_QueryDoesNotOverridePath(t *testing.T) {
	rec, got, _ := servePath(t, "/stations/st_01/pages/3?code=other&page=9")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}

	if got == nil || got.Code != "st_01" || got.Page != 3 {
		t.Fatalf("bound path = %+v, query must not override path", got)
	}
}

func TestValidateRequestPath_ValidationError_422(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		message string
	}{
		{name: "custom rule", path: "/stations/St.01/pages/1", message: "invalid station code"},
		{name: "max length", path: "/stations/abcdefghi/pages/1", message: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, got, ginErrors := servePath(t, tc.path)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body %s", rec.Code, rec.Body.String())
			}

			body := decodeError(t, rec)

			msg, ok := body.Details["code"]
			if !ok {
				t.Fatalf("details = %v, want key code", body.Details)
			}

			if tc.message != "" && msg != tc.message {
				t.Fatalf("details.code = %v, want %q", msg, tc.message)
			}

			if got != nil {
				t.Fatal("handler continued after failed validation")
			}

			if len(ginErrors) != 0 {
				t.Fatalf("gin errors = %v, want none", ginErrors)
			}
		})
	}
}

func TestValidateRequestPath_ParseError_422WithoutRawValue(t *testing.T) {
	rec, got, _ := servePath(t, "/stations/st_01/pages/qwe")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %s", rec.Code, rec.Body.String())
	}

	body := decodeError(t, rec)

	if body.Details["page"] != "Invalid value, expected number" {
		t.Fatalf("details = %v, want page: Invalid value, expected number", body.Details)
	}

	if strings.Contains(rec.Body.String(), "qwe") {
		t.Fatalf("body contains raw value: %s", rec.Body.String())
	}

	if got != nil {
		t.Fatal("handler continued after failed validation")
	}
}
