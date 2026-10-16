package validators

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/exgamer/gosdk-http-core/pkg/gin/validation"
	"github.com/exgamer/gosdk-http-core/pkg/middleware"
	"github.com/gin-gonic/gin"
)

type listQuery struct {
	validation.Request
	Page   int    `form:"page" binding:"gte=1"`
	Active bool   `form:"active"`
	Name   string `form:"name"`
}

type errorBody struct {
	Status  int            `json:"status"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func serveQuery(t *testing.T, rawQuery string) (*httptest.ResponseRecorder, *listQuery, []error) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	var got *listQuery
	var ginErrors []error

	engine := gin.New()
	engine.Use(middleware.FormattedResponseMiddleware())
	engine.GET("/items", func(c *gin.Context) {
		req := &listQuery{}
		defer func() {
			for _, e := range c.Errors {
				ginErrors = append(ginErrors, e)
			}
		}()

		if !ValidateRequestQuery(c, req) {
			return
		}

		got = req
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/items?"+rawQuery, nil))

	return rec, got, ginErrors
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) errorBody {
	t.Helper()

	var body struct {
		Data errorBody `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}

	return body.Data
}

func TestValidateRequestQuery_Valid(t *testing.T) {
	rec, got, ginErrors := serveQuery(t, "page=2&active=true&name=abc")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}

	if got == nil || got.Page != 2 || !got.Active || got.Name != "abc" {
		t.Fatalf("bound query = %+v", got)
	}

	if len(ginErrors) != 0 {
		t.Fatalf("gin errors = %v", ginErrors)
	}
}

func TestValidateRequestQuery_ValidationError_422(t *testing.T) {
	rec, got, ginErrors := serveQuery(t, "page=0")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}

	body := decodeError(t, rec)

	if body.Status != http.StatusUnprocessableEntity {
		t.Fatalf("body status = %d, want 422", body.Status)
	}

	if _, ok := body.Details["page"]; !ok {
		t.Fatalf("details = %v, want key page", body.Details)
	}

	if got != nil {
		t.Fatal("handler continued after failed validation")
	}

	if len(ginErrors) != 0 {
		t.Fatalf("gin errors = %v, want none", ginErrors)
	}
}

func TestValidateRequestQuery_ParseError_422WithoutRawValue(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		field   string
		message string
		raw     string
	}{
		{name: "int", query: "page=qwe&name=x", field: "page", message: "Invalid value, expected number", raw: "qwe"},
		{name: "bool", query: "page=1&active=yesno", field: "active", message: "Invalid value, expected boolean", raw: "yesno"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, got, ginErrors := serveQuery(t, tc.query)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", rec.Code)
			}

			body := decodeError(t, rec)

			if body.Status != http.StatusUnprocessableEntity {
				t.Fatalf("body status = %d, want 422", body.Status)
			}

			if len(body.Details) != 1 || body.Details[tc.field] != tc.message {
				t.Fatalf("details = %v, want {%s: %s}", body.Details, tc.field, tc.message)
			}

			if strings.Contains(rec.Body.String(), tc.raw) {
				t.Fatalf("body contains raw value %q: %s", tc.raw, rec.Body.String())
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
