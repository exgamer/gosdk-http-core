package exception

import (
	"github.com/exgamer/gosdk-core/pkg/validation"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
	"github.com/go-errors/errors"
	"github.com/gookit/validate"
	"net/http"
)

// HttpException Модель данных для описания ошибки
type HttpException struct {
	Error         error
	Context       map[string]any
	Code          int
	TrackInSentry bool
}

func (e *HttpException) GetErrorType() string {
	return constants.GetErrorTypeByStatusCode(e.Code)
}

func NewHttpException(code int, err error, context map[string]any) *HttpException {
	return &HttpException{Error: err, Context: context, Code: code, TrackInSentry: true}
}

func NewInternalServerErrorException(err error, context map[string]any) *HttpException {
	return &HttpException{Error: err, Context: context, Code: http.StatusInternalServerError, TrackInSentry: true}
}

func NewValidationAppException(context map[string]any) *HttpException {

	return &HttpException{Error: errors.New("VALIDATION ERROR"), Context: context, Code: http.StatusUnprocessableEntity, TrackInSentry: true}
}

func NewUntrackableAppException(code int, err error, context map[string]any) *HttpException {
	return &HttpException{Error: err, Context: context, Code: code, TrackInSentry: false}
}

func NewValidationAppExceptionFromValidationErrors(validationErrors validate.Errors) *HttpException {
	return NewValidationAppException(validation.ValidationErrorsAsMap(validationErrors))
}
