package validators

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/exgamer/gosdk-core/pkg/regex"
	"github.com/exgamer/gosdk-http-core/pkg/gin/validation"
	"github.com/exgamer/gosdk-http-core/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/gookit/validate"
	"github.com/iancoleman/strcase"
	"net/http"
	"strconv"
)

// ValidateRequestQuery - Валидация GET параметров HTTP реквеста
func ValidateRequestQuery(c *gin.Context, request validation.IRequest) bool {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		for n, f := range request.CustomValidationRules() {
			v.RegisterValidation(n, f)
		}
	}

	if err := c.BindQuery(request); err != nil {
		var ve validator.ValidationErrors

		if errors.As(err, &ve) {
			out := make(map[string]any, len(ve))

			for _, fe := range ve {
				msg := request.CustomValidationMessage(fe)

				if msg == fe.Tag() {
					msg = request.ValidationMessage(fe)
				}

				out[strcase.ToSnake(fe.Field())] = msg
			}

			response.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("validation error"), out)

			return false
		}

		// Обработка ошибок unmarshal
		var unmarshalTypeError *json.UnmarshalTypeError

		if errors.As(err, &unmarshalTypeError) {
			out := make(map[string]any)
			out[strcase.ToSnake(unmarshalTypeError.Field)] = fmt.Sprintf("Invalid type expected %s but got %s", unmarshalTypeError.Type, unmarshalTypeError.Value)
			response.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("validation error"), out)

			return false
		}

		// Обработка ошибок unmarshal
		var parseNumTypeError *strconv.NumError

		if errors.As(err, &parseNumTypeError) {
			out := make(map[string]any)
			out[strcase.ToSnake(parseNumTypeError.Num)] = parseNumTypeError.Error()
			response.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, parseNumTypeError, out)

			return false
		}

		response.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("incorrect request body"), nil)

		return false
	}

	return true
}

// ValidateRequestBody - Валидация тела HTTP реквеста
func ValidateRequestBody(c *gin.Context, request validation.IRequest) bool {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		for n, f := range request.CustomValidationRules() {
			v.RegisterValidation(n, f)
		}
	}

	if err := c.ShouldBind(&request); err != nil {
		var ve validator.ValidationErrors

		if errors.As(err, &ve) {
			out := make(map[string]any, len(ve))

			for _, fe := range ve {
				msg := request.CustomValidationMessage(fe)

				if msg == fe.Tag() {
					msg = request.ValidationMessage(fe)
				}

				out[strcase.ToSnake(fe.Field())] = msg
			}

			response.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("validation error"), out)

			return false
		}

		// Обработка ошибок unmarshal
		var unmarshalTypeError *json.UnmarshalTypeError

		if errors.As(err, &unmarshalTypeError) {
			out := make(map[string]any)
			out[strcase.ToSnake(unmarshalTypeError.Field)] = fmt.Sprintf("Invalid type expected %s but got %s", unmarshalTypeError.Type, unmarshalTypeError.Value)

			response.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("validation error"), out)

			return false
		}

		// Обработка ошибок syntax
		var syntaxTypeError *json.SyntaxError

		if errors.As(err, &syntaxTypeError) {
			response.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("incorrect request body"), nil)

			return false
		}

		response.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("incorrect request body"), nil)

		return false
	}

	return true
}

func GetIntQueryParam(c *gin.Context, name string) (int, error) {
	checkErr := regex.StringIsPositiveInt(c.Param(name))

	if checkErr != nil {
		return 0, errors.New("wrong param, must be a positive integer. Max: 2147483647")
	}

	id, cErr := strconv.Atoi(c.Param(name))

	if cErr != nil {
		return 0, cErr
	}

	return id, nil
}

// ValidationErrorsAsMap -возвращает ошибки валидации как map
func ValidationErrorsAsMap(validationErrors validate.Errors) map[string]any {
	eMap := make(map[string]any, len(validationErrors))

	for k, ve := range validationErrors {
		eMap[k] = ve.String()
	}

	return eMap
}

// BindANdValidateStruct - биндит в структуру массив битов и валидирует
func BindANdValidateStruct[T any](byte []byte, i *T) (map[string]string, error) {
	err := json.Unmarshal(byte, i)

	if err != nil {
		return nil, err
	}

	v := validator.New()
	err = v.Struct(i)

	if err != nil {
		var ve validator.ValidationErrors
		out := make(map[string]string, len(ve))

		if errors.As(err, &ve) {
			for _, fe := range ve {
				out[strcase.ToSnake(fe.Field())] = fe.Error()
			}
		}

		return out, nil
	}

	return nil, nil
}
