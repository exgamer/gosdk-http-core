package constants

import "net/http"

const (
	NotFound            = "not_found"
	AccessDenied        = "access_denied"
	OperationFailed     = "operation_failed"
	IncorrectParams     = "incorrect_parameters"
	ValidationError     = "validation_error"
	InternalServerError = "internal_server_error"
)

// GetErrorTypeByStatusCode возвращает тип ошибки для респонза по хттп статус коду
func GetErrorTypeByStatusCode(statusCode int) string {
	switch statusCode {
	case http.StatusUnprocessableEntity:
		return ValidationError
	case http.StatusInternalServerError:
		return InternalServerError
	case http.StatusForbidden:
		return AccessDenied
	case http.StatusNotAcceptable:
		return OperationFailed
	case http.StatusNotFound:
		return NotFound
	case http.StatusBadRequest:
		return IncorrectParams
	default:
		return InternalServerError
	}
}
