package helpers

type PagerRequest struct {
	Page    uint `form:"page"`
	PerPage uint `form:"per_page"`
}
