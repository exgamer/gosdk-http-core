package helpers

const DefaultPaginationPage uint = 1
const DefaultPaginationPerPage uint = 30

// MaxPaginationPerPage Верхняя граница per_page в GetPagerRequest: больше — обрезается до неё (одна страница не
// выгружает таблицу целиком).
const MaxPaginationPerPage uint = 100
