package validation

import "github.com/go-playground/validator/v10"

// IRequest - интерфейс для HTTP запросов
type IRequest interface {
	ValidationMessage(fe validator.FieldError) string
	CustomValidationMessage(fe validator.FieldError) string
	CustomValidationRules() map[string]validator.Func
}
