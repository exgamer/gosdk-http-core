package constants

import (
	"net/http"
	"testing"
)

func TestGetErrorTypeByStatusCode(t *testing.T) {
	cases := map[int]string{
		// Unchanged since before the new types.
		http.StatusBadRequest:          IncorrectParams,
		http.StatusForbidden:           AccessDenied,
		http.StatusNotFound:            NotFound,
		http.StatusNotAcceptable:       OperationFailed,
		http.StatusUnprocessableEntity: ValidationError,
		http.StatusInternalServerError: InternalServerError,

		// Used to fall back to internal_server_error.
		http.StatusUnauthorized:          Unauthorized,
		http.StatusMethodNotAllowed:      MethodNotAllowed,
		http.StatusRequestTimeout:        RequestTimeout,
		http.StatusConflict:              Conflict,
		http.StatusRequestEntityTooLarge: PayloadTooLarge,
		http.StatusTooManyRequests:       TooManyRequests,
		http.StatusBadGateway:            BadGateway,
		http.StatusServiceUnavailable:    ServiceUnavailable,
		http.StatusGatewayTimeout:        GatewayTimeout,

		// No type of their own.
		http.StatusGone:           ClientError,
		499:                       ClientError,
		http.StatusNotImplemented: InternalServerError,
		0:                         InternalServerError,
	}

	for code, want := range cases {
		if got := GetErrorTypeByStatusCode(code); got != want {
			t.Errorf("GetErrorTypeByStatusCode(%d) = %q, want %q", code, got, want)
		}
	}
}
