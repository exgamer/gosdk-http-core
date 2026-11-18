package helpers

import (
	"github.com/gin-gonic/gin"
)

// GetPagerRequest Страница из query: page и per_page не переданы или 0 — значения по умолчанию, per_page больше
// MaxPaginationPerPage — MaxPaginationPerPage.
func GetPagerRequest(ctx *gin.Context) (*PagerRequest, error) {
	var p PagerRequest

	if err := ctx.ShouldBindQuery(&p); err != nil {

		return nil, err
	}

	if p.Page == 0 {
		p.Page = DefaultPaginationPage
	}

	if p.PerPage == 0 {
		p.PerPage = DefaultPaginationPerPage
	}

	if p.PerPage > MaxPaginationPerPage {
		p.PerPage = MaxPaginationPerPage
	}

	return &p, nil
}
