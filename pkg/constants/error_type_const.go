package constants

import "net/http"

const (
	NotFound            = "not_found"
	AccessDenied        = "access_denied"
	OperationFailed     = "operation_failed"
	IncorrectParams     = "incorrect_parameters"
	ValidationError     = "validation_error"
	InternalServerError = "internal_server_error"
	Unauthorized        = "unauthorized"
	MethodNotAllowed    = "method_not_allowed"
	RequestTimeout      = "request_timeout"
	Conflict            = "conflict"
	PayloadTooLarge     = "payload_too_large"
	TooManyRequests     = "too_many_requests"
	BadGateway          = "bad_gateway"
	ServiceUnavailable  = "service_unavailable"
	GatewayTimeout      = "gateway_timeout"
	// ClientError — прочие 4xx, для которых нет отдельного типа.
	ClientError = "client_error"
)

// GetErrorTypeByStatusCode возвращает тип ошибки для респонза по хттп статус коду.
// Статус без отдельного типа: 4xx — ClientError, остальное — InternalServerError.
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
	case http.StatusUnauthorized:
		return Unauthorized
	case http.StatusMethodNotAllowed:
		return MethodNotAllowed
	case http.StatusRequestTimeout:
		return RequestTimeout
	case http.StatusConflict:
		return Conflict
	case http.StatusRequestEntityTooLarge:
		return PayloadTooLarge
	case http.StatusTooManyRequests:
		return TooManyRequests
	case http.StatusBadGateway:
		return BadGateway
	case http.StatusServiceUnavailable:
		return ServiceUnavailable
	case http.StatusGatewayTimeout:
		return GatewayTimeout
	}

	if statusCode >= 400 && statusCode < 500 {
		return ClientError
	}

	return InternalServerError
}
