package exception

import (
	"errors"
	"github.com/exgamer/gosdk-http-core/internal/publictext"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
	"github.com/gookit/validate"
	"net/http"
)

// HttpException Модель данных для описания HTTP-ошибки приложения.
type HttpException struct {
	Err           error
	Context       map[string]any
	Code          int
	TrackInSentry bool

	// explicitStatus Код задал сервис явно (NewHttpException, NewUntrackableHttpException, функции response со статусом):
	// текст 5xx без вложенной причины уходит клиенту (response.publicErrorText).
	explicitStatus bool
}

func init() {
	publictext.Explicit = func(v any) bool {
		e, ok := v.(*HttpException)

		return ok && e != nil && e.explicitStatus
	}
}

func (e *HttpException) Error() string {
	if e.Err == nil {
		return "http exception"
	}

	return e.Err.Error()
}

func (e *HttpException) Unwrap() error { return e.Err }

func (e *HttpException) GetErrorType() string {
	return constants.GetErrorTypeByStatusCode(e.Code)
}

// NewHttpException Исключение с кодом, заданным сервисом явно: при 5xx текст простой ошибки (без вложенной причины)
// уходит клиенту, ошибка с причиной (%w, обёртка) — текст статуса.
func NewHttpException(code int, err error, context map[string]any) *HttpException {
	return &HttpException{
		Err:            err,
		Context:        context,
		Code:           code,
		TrackInSentry:  true,
		explicitStatus: true,
	}
}

// NewInternalServerErrorException 500 без явного кода: клиенту — текст статуса, ошибка — в лог и Sentry.
func NewInternalServerErrorException(err error, context map[string]any) *HttpException {
	ex := NewHttpException(http.StatusInternalServerError, err, context)
	ex.explicitStatus = false

	return ex
}

func NewValidationHttpException(context map[string]any) *HttpException {
	return NewHttpException(
		http.StatusUnprocessableEntity,
		errors.New("VALIDATION ERROR"),
		context,
	)
}

func NewUntrackableHttpException(code int, err error, context map[string]any) *HttpException {
	ex := NewHttpException(code, err, context)
	ex.TrackInSentry = false

	return ex
}

func NewValidationAppExceptionFromValidationErrors(validationErrors validate.Errors) *HttpException {
	return NewValidationHttpException(ValidationErrorsAsMap(validationErrors))
}

// ValidationErrorsAsMap -возвращает ошибки валидации как map
func ValidationErrorsAsMap(validationErrors validate.Errors) map[string]any {
	eMap := make(map[string]any, len(validationErrors))

	for k, ve := range validationErrors {
		eMap[k] = ve.String()
	}

	return eMap
}
