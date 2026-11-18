package helpers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// per_page: 0 — по умолчанию, больше MaxPaginationPerPage — обрезается.
func TestGetPagerRequest_PerPageBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := map[string]uint{"": DefaultPaginationPerPage, "per_page=50": 50, "per_page=100": 100, "per_page=100000": MaxPaginationPerPage}

	for query, want := range cases {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)

		p, err := GetPagerRequest(c)
		if err != nil {
			t.Fatalf("%q: %v", query, err)
		}

		if p.PerPage != want || p.Page != DefaultPaginationPage {
			t.Errorf("%q: per_page = %d page = %d, want %d 1", query, p.PerPage, p.Page, want)
		}
	}
}
