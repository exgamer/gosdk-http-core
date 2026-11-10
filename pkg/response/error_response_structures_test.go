package response

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/exgamer/gosdk-http-core/pkg/structures"
	"github.com/gin-gonic/gin"
)

func formattedError(t *testing.T, status int) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	ErrorResponseWithStatus(c, status, errors.New("ошибка"), map[string]any{"code": "some_code", "amount": 10})
	Formatted(c)

	return w, w.Body.Bytes()
}

func decodeStrict(t *testing.T, body []byte, dst any) {
	t.Helper()

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		t.Fatalf("тело ответа не совпадает со структурой: %v; тело: %s", err, body)
	}
}

func TestErrorResponseStructuresMatchFormattedBody(t *testing.T) {
	cases := []struct {
		name   string
		status int
		dst    func() any
		read   func(any) (bool, int, string, map[string]any)
	}{
		{"401", http.StatusUnauthorized, func() any { return &structures.UnauthorizedErrorResponse{} }, func(v any) (bool, int, string, map[string]any) {
			r := v.(*structures.UnauthorizedErrorResponse)
			return r.Success, r.Data.Status, r.Data.Message, r.Data.Details
		}},
		{"413", http.StatusRequestEntityTooLarge, func() any { return &structures.PayloadTooLargeErrorResponse{} }, func(v any) (bool, int, string, map[string]any) {
			r := v.(*structures.PayloadTooLargeErrorResponse)
			return r.Success, r.Data.Status, r.Data.Message, r.Data.Details
		}},
		{"415", http.StatusUnsupportedMediaType, func() any { return &structures.UnsupportedMediaTypeErrorResponse{} }, func(v any) (bool, int, string, map[string]any) {
			r := v.(*structures.UnsupportedMediaTypeErrorResponse)
			return r.Success, r.Data.Status, r.Data.Message, r.Data.Details
		}},
		{"500", http.StatusInternalServerError, func() any { return &structures.InternalServerErrorResponse{} }, func(v any) (bool, int, string, map[string]any) {
			r := v.(*structures.InternalServerErrorResponse)
			return r.Success, r.Data.Status, r.Data.Message, r.Data.Details
		}},
		{"503", http.StatusServiceUnavailable, func() any { return &structures.ServiceUnavailableErrorResponse{} }, func(v any) (bool, int, string, map[string]any) {
			r := v.(*structures.ServiceUnavailableErrorResponse)
			return r.Success, r.Data.Status, r.Data.Message, r.Data.Details
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, body := formattedError(t, tc.status)
			if w.Code != tc.status {
				t.Fatalf("статус %d, ожидали %d", w.Code, tc.status)
			}

			dst := tc.dst()
			decodeStrict(t, body, dst)

			success, status, message, details := tc.read(dst)
			if success || status != tc.status || message != "ошибка" || details["code"] != "some_code" {
				t.Fatalf("поля не разобраны: success=%v status=%d message=%q details=%v", success, status, message, details)
			}
		})
	}
}

func TestSuccessAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	SuccessAccepted(c, map[string]string{"id": "1"})
	Formatted(c)

	if w.Code != http.StatusAccepted {
		t.Fatalf("статус %d, ожидали 202", w.Code)
	}

	var got structures.Response[map[string]string]
	decodeStrict(t, w.Body.Bytes(), &got)

	if !got.Success || got.Data["id"] != "1" {
		t.Fatalf("тело: %s", w.Body.String())
	}
}

func TestRawAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	RawAccepted(c, map[string]string{"id": "1"})
	Formatted(c)

	if w.Code != http.StatusAccepted {
		t.Fatalf("статус %d, ожидали 202", w.Code)
	}

	var got map[string]string
	decodeStrict(t, w.Body.Bytes(), &got)

	if got["id"] != "1" {
		t.Fatalf("тело: %s", w.Body.String())
	}
}

func TestRawErrorResponseMatchesRawBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	ErrorResponseWithStatusRaw(c, http.StatusUnauthorized, errors.New("ошибка"), map[string]any{"code": "some_code"})
	Formatted(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("статус %d, ожидали 401", w.Code)
	}

	var got structures.RawErrorResponse
	decodeStrict(t, w.Body.Bytes(), &got)

	if got.Status != http.StatusUnauthorized || got.Message != "ошибка" || got.Details["code"] != "some_code" {
		t.Fatalf("поля не разобраны: %+v; тело: %s", got, w.Body.String())
	}
}
