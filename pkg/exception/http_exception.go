package exception

import (
	"errors"
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

func NewHttpException(code int, err error, context map[string]any) *HttpException {
	return &HttpException{
		Err:           err,
		Context:       context,
		Code:          code,
		TrackInSentry: true,
	}
}

func NewInternalServerErrorException(err error, context map[string]any) *HttpException {
	return NewHttpException(http.StatusInternalServerError, err, context)
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
