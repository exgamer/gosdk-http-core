package validators

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/exgamer/gosdk-http-core/pkg/gin/validation"
	"github.com/exgamer/gosdk-http-core/pkg/middleware"
	"github.com/gin-gonic/gin"
)

// testDay Дата `YYYY-MM-DD` со своим UnmarshalJSON: другое значение — ошибка
type testDay struct {
	time.Time
}

func (d *testDay) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return errors.New("date must be a string")
	}

	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return errors.New("date must be YYYY-MM-DD")
	}

	d.Time = t

	return nil
}

type testPeriod struct {
	EndDay    testDay `json:"endDay"`
	DaysCount int     `json:"daysCount"`
}

type testSlot struct {
	AtDay testDay `json:"atDay"`
}

type testEmbedded struct {
	EmbeddedDay testDay `json:"embeddedDay"`
}

type unmarshalerBody struct {
	validation.Request
	testEmbedded
	StartDay  testDay    `json:"startDay"`
	ItemCount int        `json:"itemCount"`
	Period    testPeriod `json:"period"`
	Slots     []testSlot `json:"slots"`
	Raw       json.RawMessage
}

func serveUnmarshaler(t *testing.T, body string) (int, errorBody, *unmarshalerBody) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var got *unmarshalerBody

	r := gin.New()
	r.Use(middleware.FormattedResponseMiddleware())
	r.POST("/x", func(c *gin.Context) {
		req := &unmarshalerBody{}
		if !ValidateRequestBody(c, req) {
			return
		}

		got = req
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		return w.Code, errorBody{}, got
	}

	return w.Code, decodeError(t, w), got
}

// Ошибка своего UnmarshalJSON — 422 «validation error» с ключом поля (как у ошибки типа) и текстом ошибки
func TestValidateRequestBody_UnmarshalerFieldKey(t *testing.T) {
	cases := []struct {
		name     string
		keysEnv  string
		body     string
		key, msg string
	}{
		{"top level", "", `{"startDay":"2024-13-01"}`, "start_day", "date must be YYYY-MM-DD"},
		{"top level json keys", "json", `{"startDay":"2024-13-01"}`, "startDay", "date must be YYYY-MM-DD"},
		{"nested", "", `{"period":{"daysCount":1,"endDay":5}}`, "period_end_day", "date must be a string"},
		{"nested json keys", "json", `{"period":{"daysCount":1,"endDay":5}}`, "period.endDay", "date must be a string"},
		{"slice item json keys", "json", `{"slots":[{"atDay":"2024-01-01"},{"atDay":"x"}]}`, "slots.atDay", "date must be YYYY-MM-DD"},
		{"embedded json keys", "json", `{"embeddedDay":"x"}`, "embeddedDay", "date must be YYYY-MM-DD"},
		{"case-insensitive key json keys", "json", `{"STARTDAY":"x"}`, "startDay", "date must be YYYY-MM-DD"},
		{"first failing in body order", "json", `{"period":{"endDay":"x"},"startDay":"y"}`, "period.endDay", "date must be YYYY-MM-DD"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(ValidationKeysEnv, tc.keysEnv)

			code, body, _ := serveUnmarshaler(t, tc.body)
			if code != http.StatusUnprocessableEntity || body.Message != "validation error" {
				t.Fatalf("status %d message %q, want 422 validation error", code, body.Message)
			}

			if len(body.Details) != 1 || body.Details[tc.key] != tc.msg {
				t.Fatalf("details = %v, want {%s: %s}", body.Details, tc.key, tc.msg)
			}
		})
	}
}

// Ключ ошибки своего UnmarshalJSON в обоих режимах VALIDATION_KEYS тот же, что у ошибки типа на том же пути
func TestValidateRequestBody_UnmarshalerKeyLikeTypeError(t *testing.T) {
	for _, env := range []string{"", "json"} {
		t.Run("VALIDATION_KEYS="+env, func(t *testing.T) {
			t.Setenv(ValidationKeysEnv, env)

			_, typeErr, _ := serveUnmarshaler(t, `{"period":{"daysCount":"x"}}`)
			_, custom, _ := serveUnmarshaler(t, `{"period":{"endDay":"x"}}`)

			want := map[string]string{"": "period_days_count", "json": "period.daysCount"}[env]
			if _, ok := typeErr.Details[want]; !ok {
				t.Fatalf("type error details = %v, want key %q", typeErr.Details, want)
			}

			wantCustom := map[string]string{"": "period_end_day", "json": "period.endDay"}[env]
			if _, ok := custom.Details[wantCustom]; !ok {
				t.Fatalf("unmarshaler details = %v, want key %q", custom.Details, wantCustom)
			}
		})
	}
}

// Валидное тело, ошибка типа, синтаксис, хвост, пустое тело и RawMessage — как раньше
func TestValidateRequestBody_UnmarshalerUnchanged(t *testing.T) {
	code, _, got := serveUnmarshaler(t, `{"startDay":"2024-02-03","period":{"endDay":"2024-02-04"},"slots":[{"atDay":"2024-02-05"}],"Raw":{"any":1}}`)
	if code != http.StatusOK || got == nil || got.StartDay.Format(time.DateOnly) != "2024-02-03" ||
		got.Period.EndDay.Format(time.DateOnly) != "2024-02-04" || string(got.Raw) != `{"any":1}` {
		t.Fatalf("valid body: status %d got %+v", code, got)
	}

	for _, tc := range []struct {
		name, body, message string
		details             map[string]any
	}{
		{"type error", `{"itemCount":"x"}`, "validation error", map[string]any{"item_count": "Invalid type expected int but got string"}},
		{"syntax error", `{"startDay":`, "incorrect request body", nil},
		{"syntax error in custom field", `{"startDay":x}`, "incorrect request body", nil},
		{"trailing data", `{"startDay":"2024-02-03"}{}`, "incorrect request body", nil},
		{"empty body", ``, "incorrect request body", nil},
		{"not an object", `[1]`, "validation error", map[string]any{"": "Invalid type expected validators.unmarshalerBody but got array"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, body, _ := serveUnmarshaler(t, tc.body)
			if code != http.StatusUnprocessableEntity || body.Message != tc.message {
				t.Fatalf("status %d message %q, want 422 %q", code, body.Message, tc.message)
			}

			if len(body.Details) != len(tc.details) {
				t.Fatalf("details = %v, want %v", body.Details, tc.details)
			}

			for k, v := range tc.details {
				if body.Details[k] != v {
					t.Fatalf("details = %v, want %v", body.Details, tc.details)
				}
			}
		})
	}
}
