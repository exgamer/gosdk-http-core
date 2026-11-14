package validators

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/exgamer/gosdk-http-core/pkg/gin/validation"
	"github.com/exgamer/gosdk-http-core/pkg/middleware"
	"github.com/gin-gonic/gin"
)

type itemBody struct {
	ComponentIDs []int `json:"component_ids" binding:"required"`
}

type createBody struct {
	validation.Request
	ComponentIDs []int      `json:"component_ids" binding:"required"`
	Items        []itemBody `json:"items" binding:"dive"`
}

func serveBodyKeys(t *testing.T, body string) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(middleware.FormattedResponseMiddleware())
	r.POST("/x", func(c *gin.Context) { ValidateRequestBody(c, &createBody{}) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var out struct {
		Data errorBody `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("response %d %s: %v", w.Code, w.Body.String(), err)
	}

	return out.Data.Details
}

// Default — the previous keys (last Go field name in snake_case), unchanged for existing clients: the top-level and
// the nested ComponentIDs both give component_i_ds (one key).
func TestValidationKeys_DefaultUnchanged(t *testing.T) {
	d := serveBodyKeys(t, `{"items":[{}]}`)

	if _, ok := d["component_i_ds"]; !ok || len(d) != 1 {
		t.Fatalf("details = %v, want the previous single key component_i_ds", d)
	}
}

// VALIDATION_KEYS=json — keys from the json tag with the full path of a nested field.
func TestValidationKeys_JSON(t *testing.T) {
	t.Setenv(ValidationKeysEnv, "json")

	d := serveBodyKeys(t, `{"items":[{"component_ids":[1]},{}]}`)

	for _, key := range []string{"component_ids", "items[1].component_ids"} {
		if _, ok := d[key]; !ok {
			t.Fatalf("details = %v, want key %q", d, key)
		}
	}
}

// VALIDATION_KEYS=json — a JSON type error keeps its JSON path.
func TestValidationKeys_JSONTypeError(t *testing.T) {
	t.Setenv(ValidationKeysEnv, "json")

	d := serveBodyKeys(t, `{"component_ids":"x"}`)

	if _, ok := d["component_ids"]; !ok {
		t.Fatalf("details = %v, want key component_ids", d)
	}
}

type sourceQuery struct {
	validation.Request
	ComponentIDs []int `json:"componentIds" form:"component_ids" binding:"required"`
}

// VALIDATION_KEYS=json — the key comes from the tag of the request source: query — form, body — json.
func TestValidationKeys_JSONTagBySource(t *testing.T) {
	t.Setenv(ValidationKeysEnv, "json")
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(middleware.FormattedResponseMiddleware())
	r.GET("/q", func(c *gin.Context) { ValidateRequestQuery(c, &sourceQuery{}) })
	r.POST("/b", func(c *gin.Context) { ValidateRequestBody(c, &sourceQuery{}) })

	for _, tc := range []struct{ method, path, body, key string }{
		{http.MethodGet, "/q", "", "component_ids"},
		{http.MethodPost, "/b", "{}", "componentIds"},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		var out struct {
			Data errorBody `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &out)

		if _, ok := out.Data.Details[tc.key]; !ok {
			t.Fatalf("%s %s: details = %v, want key %q", tc.method, tc.path, out.Data.Details, tc.key)
		}
	}
}
