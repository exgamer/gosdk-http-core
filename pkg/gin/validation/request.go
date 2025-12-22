package validation

import (
	"github.com/go-playground/validator/v10"
)

// Request - HTTP запрос
type Request struct {
}

func (r *Request) ValidationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email"
	case "gt":
		return "{0} must be grater than {1}"
	}

	return fe.Tag()
}

func (r *Request) CustomValidationMessage(fe validator.FieldError) string {
	return fe.Tag()
}

func (r *Request) CustomValidationRules() map[string]validator.Func {
	return make(map[string]validator.Func)
}
