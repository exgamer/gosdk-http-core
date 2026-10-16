package validators

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/exgamer/gosdk-http-core/pkg/gin/validation"
	"github.com/exgamer/gosdk-http-core/pkg/middleware"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func evenRule(fl validator.FieldLevel) bool {
	return fl.Field().Int()%2 == 0
}

type evenBody struct {
	validation.Request
	Value int `json:"value" binding:"http_core_even"`
}

func (r *evenBody) CustomValidationRules() map[string]validator.Func {
	return map[string]validator.Func{"http_core_even": evenRule}
}

func (r *evenBody) CustomValidationMessage(fe validator.FieldError) string {
	if fe.Tag() == "http_core_even" {
		return "must be even"
	}

	return fe.Tag()
}

type evenQuery struct {
	validation.Request
	Value int `form:"value" binding:"http_core_even_query"`
}

func (r *evenQuery) CustomValidationRules() map[string]validator.Func {
	return map[string]validator.Func{"http_core_even_query": evenRule}
}

type plainBody struct {
	validation.Request
	Name string `json:"name" binding:"required"`
}

type codeThree struct {
	validation.Request
	Code string `json:"code" binding:"http_core_code"`
}

func (r *codeThree) CustomValidationRules() map[string]validator.Func {
	return map[string]validator.Func{"http_core_code": func(fl validator.FieldLevel) bool {
		return len(fl.Field().String()) == 3
	}}
}

type codeFive struct {
	validation.Request
	Code string `json:"code" binding:"http_core_code"`
}

func (r *codeFive) CustomValidationRules() map[string]validator.Func {
	return map[string]validator.Func{"http_core_code": func(fl validator.FieldLevel) bool {
		return len(fl.Field().String()) == 5
	}}
}

func bodyRoute(newRequest func() validation.IRequest) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ValidateRequestBody(c, newRequest()) {
			return
		}

		c.Status(http.StatusOK)
	}
}

func customRulesEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(middleware.FormattedResponseMiddleware())
	engine.POST("/even", bodyRoute(func() validation.IRequest { return &evenBody{} }))
	engine.POST("/plain", bodyRoute(func() validation.IRequest { return &plainBody{} }))
	engine.POST("/code3", bodyRoute(func() validation.IRequest { return &codeThree{} }))
	engine.POST("/code5", bodyRoute(func() validation.IRequest { return &codeFive{} }))
	engine.GET("/even", func(c *gin.Context) {
		if !ValidateRequestQuery(c, &evenQuery{}) {
			return
		}

		c.Status(http.StatusOK)
	})

	return engine
}

func serve(engine *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	return rec
}

func TestValidateRequest_CustomRulesParallel(t *testing.T) {
	engine := customRulesEngine()

	type call struct {
		method string
		target string
		body   string
		want   int
	}

	calls := []call{
		{method: http.MethodPost, target: "/even", body: `{"value":2}`, want: http.StatusOK},
		{method: http.MethodPost, target: "/even", body: `{"value":3}`, want: http.StatusUnprocessableEntity},
		{method: http.MethodGet, target: "/even?value=4", want: http.StatusOK},
		{method: http.MethodGet, target: "/even?value=5", want: http.StatusUnprocessableEntity},
		{method: http.MethodPost, target: "/plain", body: `{"name":"x"}`, want: http.StatusOK},
		{method: http.MethodPost, target: "/plain", body: `{}`, want: http.StatusUnprocessableEntity},
	}

	var wg sync.WaitGroup
	errs := make(chan string, 64*len(calls))

	for i := 0; i < 64; i++ {
		for _, cl := range calls {
			wg.Add(1)

			go func(cl call) {
				defer wg.Done()

				rec := serve(engine, cl.method, cl.target, cl.body)
				if rec.Code != cl.want {
					errs <- fmt.Sprintf("%s %s %s: status = %d, want %d", cl.method, cl.target, cl.body, rec.Code, cl.want)
				}
			}(cl)
		}
	}

	wg.Wait()
	close(errs)

	for e := range errs {
		t.Error(e)
	}
}

func TestValidateRequestBody_CustomRuleMessage(t *testing.T) {
	rec := serve(customRulesEngine(), http.MethodPost, "/even", `{"value":3}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}

	body := decodeError(t, rec)

	if body.Details["value"] != "must be even" {
		t.Fatalf("details = %v, want {value: must be even}", body.Details)
	}
}

func TestValidateRequestBody_SameRuleNameInDifferentRequests(t *testing.T) {
	engine := customRulesEngine()

	cases := []struct {
		target string
		body   string
		want   int
	}{
		{target: "/code3", body: `{"code":"abc"}`, want: http.StatusOK},
		{target: "/code5", body: `{"code":"abcde"}`, want: http.StatusOK},
		{target: "/code3", body: `{"code":"abcde"}`, want: http.StatusUnprocessableEntity},
		{target: "/code5", body: `{"code":"abc"}`, want: http.StatusUnprocessableEntity},
	}

	for _, tc := range cases {
		rec := serve(engine, http.MethodPost, tc.target, tc.body)
		if rec.Code != tc.want {
			t.Errorf("%s %s: status = %d, want %d", tc.target, tc.body, rec.Code, tc.want)
		}
	}
}
