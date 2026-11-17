package validators

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/exgamer/gosdk-http-core/pkg/gin/validation"
	"github.com/gin-gonic/gin"
)

type jsonBodyTestRequest struct {
	validation.Request
	Name string `json:"name" binding:"required"`
}

func jsonBodyTestDo(t *testing.T, body string) (*httptest.ResponseRecorder, bool, jsonBodyTestRequest) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	var req jsonBodyTestRequest
	ok := ValidateRequestBody(c, &req)

	return w, ok, req
}

// Тело с данными после первого JSON-значения — 422; одно значение (с пробелами и неизвестным полем) — как раньше
func TestValidateRequestBody_OneJSONValue(t *testing.T) {
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"one object", `{"name":"a"}`, true},
		{"trailing whitespace", "{\"name\":\"a\"}\n  \t", true},
		{"unknown field — as before", `{"name":"a","extra":1}`, true},
		{"second object", `{"name":"a"}{"name":"b"}`, false},
		{"trailing garbage", `{"name":"a"}garbage`, false},
		{"rule error — as before", `{"name":""}`, false},
		{"empty body — as before", ``, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, ok, req := jsonBodyTestDo(t, tc.body)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v; status %d body %s", ok, tc.ok, w.Code, w.Body.String())
			}

			if ok && req.Name != "a" {
				t.Errorf("name = %q, want a", req.Name)
			}

			if !ok && w.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422", w.Code)
			}
		})
	}
}
