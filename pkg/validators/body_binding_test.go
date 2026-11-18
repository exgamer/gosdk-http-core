package validators

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/exgamer/gosdk-http-core/pkg/gin/validation"
	"github.com/exgamer/gosdk-http-core/pkg/middleware"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// jsonOnlyBody Запрос без тегов form; UserID заполняет сервис, не клиент.
type jsonOnlyBody struct {
	validation.Request
	Name   string `json:"name" binding:"required"`
	UserID int    `json:"-"`
}

// formBody Запрос с тегами form; UserID — `json:"-"` без тега form.
type formBody struct {
	validation.Request
	Name   string `form:"name" json:"name" binding:"required"`
	UserID int    `json:"-"`
	Role   string `json:"-" binding:"omitempty,eq=user"`
}

// hiddenQuery Параметры query; UserID — `json:"-"` без тега form.
type hiddenQuery struct {
	validation.Request
	Page   int `form:"page"`
	UserID int `json:"-"`
}

func serveBodyRequest(t *testing.T, req *http.Request, newRequest func() validation.IRequest) (*httptest.ResponseRecorder, validation.IRequest) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	var got validation.IRequest

	engine := gin.New()
	engine.Use(middleware.FormattedResponseMiddleware())
	engine.Any("/b", func(c *gin.Context) {
		r := newRequest()
		if !ValidateRequestBody(c, r) {
			return
		}

		got = r
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	return rec, got
}

func bodyRequest(method, target, contentType, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return req
}

func newJSONOnly() validation.IRequest { return &jsonOnlyBody{} }
func newForm() validation.IRequest     { return &formBody{} }

// Без Content-Type тело разбирается как JSON, а не из query и формы.
func TestValidateRequestBody_NoContentTypeIsJSON(t *testing.T) {
	rec, got := serveBodyRequest(t, bodyRequest(http.MethodPost, "/b?Name=fromquery&UserID=7", "", `{"name":"Ann"}`), newJSONOnly)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}

	if r := got.(*jsonOnlyBody); r.Name != "Ann" || r.UserID != 0 {
		t.Fatalf("got %+v, want name from JSON and no UserID", r)
	}

	// пустое тело без Content-Type — пустой объект: правила проверяются, query не читается
	rec, _ = serveBodyRequest(t, bodyRequest(http.MethodPost, "/b?Name=fromquery", "", ""), newJSONOnly)
	if body := decodeError(t, rec); rec.Code != http.StatusUnprocessableEntity || body.Message != "validation error" {
		t.Fatalf("empty body without Content-Type = %d %s, want 422 validation error", rec.Code, rec.Body.String())
	}
}

// application/merge-patch+json — JSON.
func TestValidateRequestBody_PlusJSON(t *testing.T) {
	rec, got := serveBodyRequest(t, bodyRequest(http.MethodPatch, "/b", "application/merge-patch+json; charset=utf-8", `{"name":"Bob"}`), newJSONOnly)
	if rec.Code != http.StatusOK || got.(*jsonOnlyBody).Name != "Bob" {
		t.Fatalf("merge-patch = %d %s", rec.Code, rec.Body.String())
	}
}

// Форма в запрос без тегов form и неизвестный тип тела — 415.
func TestValidateRequestBody_UnsupportedMediaType(t *testing.T) {
	cases := []struct{ ct, body string }{
		{"application/x-www-form-urlencoded", "name=Ann&UserID=7"},
		{"multipart/form-data; boundary=x", "--x--\r\n"},
		{"text/plain", "name=Ann"},
	}

	for _, tc := range cases {
		rec, _ := serveBodyRequest(t, bodyRequest(http.MethodPost, "/b", tc.ct, tc.body), newJSONOnly)
		if rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("%s: status = %d %s, want 415", tc.ct, rec.Code, rec.Body.String())
		}
	}
}

// Форма в запрос с тегами form разбирается; поля `json:"-"` без тега form из формы и query не заполняются,
// а правила проверяются по восстановленному значению.
func TestValidateRequestBody_FormHiddenFieldsNotBound(t *testing.T) {
	rec, got := serveBodyRequest(t, bodyRequest(http.MethodPost, "/b?UserID=8", "application/x-www-form-urlencoded", "name=Ann&UserID=7&Role=admin"), newForm)
	if rec.Code != http.StatusOK {
		t.Fatalf("form = %d %s", rec.Code, rec.Body.String())
	}

	if r := got.(*formBody); r.Name != "Ann" || r.UserID != 0 || r.Role != "" {
		t.Fatalf("got %+v, want only name", r)
	}

	var buf bytes.Buffer

	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("name", "Kim")
	_ = mw.WriteField("UserID", "9")
	_ = mw.Close()

	rec, got = serveBodyRequest(t, bodyRequest(http.MethodPost, "/b", mw.FormDataContentType(), buf.String()), newForm)
	if rec.Code != http.StatusOK || got.(*formBody).Name != "Kim" || got.(*formBody).UserID != 0 {
		t.Fatalf("multipart = %d %s %+v", rec.Code, rec.Body.String(), got)
	}
}

// GET — параметры query, как раньше; поле `json:"-"` — нет.
func TestValidateRequestBody_GetBindsQuery(t *testing.T) {
	rec, got := serveBodyRequest(t, bodyRequest(http.MethodGet, "/b?name=Ann&UserID=7", "", ""), newForm)
	if rec.Code != http.StatusOK || got.(*formBody).Name != "Ann" || got.(*formBody).UserID != 0 {
		t.Fatalf("GET = %d %s %+v", rec.Code, rec.Body.String(), got)
	}
}

// ValidateRequestQuery: поле `json:"-"` без тега form из query не заполняется.
func TestValidateRequestQuery_HiddenFieldNotBound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := &hiddenQuery{}
	engine := gin.New()
	engine.GET("/q", func(c *gin.Context) {
		req.UserID = 42 // заполнил сервис до валидации

		if ValidateRequestQuery(c, req) {
			c.Status(http.StatusOK)
		}
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/q?page=2&UserID=7", nil))

	if rec.Code != http.StatusOK || req.Page != 2 || req.UserID != 42 {
		t.Fatalf("query = %d %+v, want page 2 and UserID 42", rec.Code, req)
	}
}

// JSON-тело больше 10 МиБ — 413, не 422; ровно 10 МиБ — разбирается.
func TestValidateRequestBody_TooLarge413(t *testing.T) {
	pad := strings.Repeat("a", maxBodyBytes)
	rec, _ := serveBodyRequest(t, bodyRequest(http.MethodPost, "/b", "application/json", `{"name":"`+pad+`"}`), newJSONOnly)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}

	exact := `{"name":"` + strings.Repeat("a", maxBodyBytes-len(`{"name":""}`)) + `"}`
	if rec, _ = serveBodyRequest(t, bodyRequest(http.MethodPost, "/b", "application/json", exact), newJSONOnly); rec.Code != http.StatusOK {
		t.Fatalf("exactly 10 MiB: status = %d, want 200", rec.Code)
	}
}

// XML — как раньше.
func TestValidateRequestBody_XMLUnchanged(t *testing.T) {
	type xmlBody struct {
		validation.Request
		Name string `xml:"name" binding:"required"`
	}

	rec, got := serveBodyRequest(t, bodyRequest(http.MethodPost, "/b", "application/xml", `<xmlBody><name>Ann</name></xmlBody>`),
		func() validation.IRequest { return &xmlBody{} })
	if rec.Code != http.StatusOK || got.(*xmlBody).Name != "Ann" {
		t.Fatalf("xml = %d %s", rec.Code, rec.Body.String())
	}
}

type dispatchThree struct {
	validation.Request
	Code string `json:"code" binding:"http_core_dispatch"`
}

func (r *dispatchThree) CustomValidationRules() map[string]validator.Func {
	return map[string]validator.Func{"http_core_dispatch": func(fl validator.FieldLevel) bool { return len(fl.Field().String()) == 3 }}
}

type dispatchFive struct {
	validation.Request
	Code string `json:"code" binding:"http_core_dispatch"`
}

func (r *dispatchFive) CustomValidationRules() map[string]validator.Func {
	return map[string]validator.Func{"http_core_dispatch": func(fl validator.FieldLevel) bool { return len(fl.Field().String()) == 5 }}
}

// Одно имя правила у двух запросов: каждый проверяется своей функцией, даже если оба зарегистрированы до первой
// валидации (раньше первый запрос получал функцию последнего зарегистрированного).
func TestCustomRules_SameNameRegisteredBeforeFirstUse(t *testing.T) {
	registerCustomRules(&dispatchThree{})
	registerCustomRules(&dispatchFive{})

	three := func() validation.IRequest { return &dispatchThree{} }
	five := func() validation.IRequest { return &dispatchFive{} }

	cases := []struct {
		newRequest func() validation.IRequest
		body       string
		want       int
	}{
		{three, `{"code":"abc"}`, http.StatusOK},
		{three, `{"code":"abcde"}`, http.StatusUnprocessableEntity},
		{five, `{"code":"abcde"}`, http.StatusOK},
		{five, `{"code":"abc"}`, http.StatusUnprocessableEntity},
	}

	for _, tc := range cases {
		rec, _ := serveBodyRequest(t, bodyRequest(http.MethodPost, "/b", "application/json", tc.body), tc.newRequest)
		if rec.Code != tc.want {
			t.Errorf("%T %s: status = %d, want %d", tc.newRequest(), tc.body, rec.Code, tc.want)
		}
	}
}

type lockProbe struct {
	validation.Request
	Value int `json:"value" binding:"http_core_lock_probe"`
}

func (r *lockProbe) CustomValidationRules() map[string]validator.Func {
	return map[string]validator.Func{"http_core_lock_probe": func(validator.FieldLevel) bool { return true }}
}

// Медленное тело не держит блокировку правил: регистрация правил другого запроса не ждёт чтения тела.
func TestValidateRequestBody_BodyReadOutsideLock(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)

		req := httptest.NewRequest(http.MethodPost, "/b", pr)
		req.Header.Set("Content-Type", "application/json")
		serveBodyRequest(t, req, newJSONOnly)
	}()

	_, _ = pw.Write([]byte(`{"name":`)) // тело начато, но не закончено
	registered := make(chan struct{})

	go func() {
		registerCustomRules(&lockProbe{})
		close(registered)
	}()

	select {
	case <-registered:
	case <-time.After(2 * time.Second):
		t.Fatal("rule registration blocked by a slow request body")
	}

	_, _ = pw.Write([]byte(`"Ann"}`))
	_ = pw.Close()
	<-done
}
