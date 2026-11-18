package validators

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/exgamer/gosdk-http-core/pkg/gin/validation"
	"github.com/gin-gonic/gin"
)

// innerHidden Вложенная структура: Note — от клиента, UserID — `json:"-"` без тега form.
type innerHidden struct {
	Note   string `form:"note"`
	UserID int    `json:"-"`
}

// owner Структура под скрытым указателем.
type owner struct {
	ID int
}

// pointerQuery Вложенные структуры по указателю.
type pointerQuery struct {
	validation.Request
	Page  int `form:"page"`
	Inner *innerHidden
	Owner *owner `json:"-"`
	Count *int   `json:"-"`
}

// bindPointerQuery prepare заполняет запрос до валидации (как сервис).
func bindPointerQuery(t *testing.T, target string, prepare func(*pointerQuery)) *pointerQuery {
	t.Helper()

	gin.SetMode(gin.TestMode)

	req := &pointerQuery{}
	engine := gin.New()
	engine.GET("/q", func(c *gin.Context) {
		prepare(req)

		if ValidateRequestQuery(c, req) {
			c.Status(http.StatusOK)
		}
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}

	return req
}

// nil-указатель на вложенную структуру, созданный биндингом: скрытое поле не заполняется, поле с form — заполняется.
func TestValidateRequestQuery_NilPointerNestedHiddenNotBound(t *testing.T) {
	req := bindPointerQuery(t, "/q?page=2&note=a&UserID=7", func(*pointerQuery) {})

	if req.Page != 2 || req.Inner == nil || req.Inner.Note != "a" || req.Inner.UserID != 0 {
		t.Fatalf("got page %d inner %+v, want page 2, note a, UserID 0", req.Page, req.Inner)
	}
}

// Заданный сервисом указатель на вложенную структуру: скрытое поле сохраняет значение сервиса.
func TestValidateRequestQuery_SetPointerNestedHiddenKept(t *testing.T) {
	req := bindPointerQuery(t, "/q?note=a&UserID=7", func(r *pointerQuery) {
		r.Inner = &innerHidden{UserID: 42}
	})

	if req.Inner == nil || req.Inner.Note != "a" || req.Inner.UserID != 42 {
		t.Fatalf("inner = %+v, want note a and UserID 42", req.Inner)
	}
}

// Скрытый указатель (`*Inner` с `json:"-"`): gin заполнил бы структуру под ним на месте или создал бы новую.
func TestValidateRequestQuery_HiddenPointerNotBound(t *testing.T) {
	count := 5

	req := bindPointerQuery(t, "/q?ID=7&Count=9", func(r *pointerQuery) {
		r.Owner = &owner{ID: 42}
		r.Count = &count
	})

	if req.Owner == nil || req.Owner.ID != 42 || req.Count != &count || count != 5 {
		t.Fatalf("owner = %+v, count = %d, want ID 42 and count 5", req.Owner, count)
	}

	req = bindPointerQuery(t, "/q?ID=7&Count=9", func(*pointerQuery) {})

	if req.Owner != nil || req.Count != nil {
		t.Fatalf("owner = %+v, count = %v, want nil", req.Owner, req.Count)
	}
}
