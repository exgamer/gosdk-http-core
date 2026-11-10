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
	"github.com/iancoleman/strcase"
	"net/http"
	"net/url"
	"slices"
	"strconv"
)

// ValidateRequestQuery - Валидация GET параметров HTTP реквеста
func ValidateRequestQuery(c *gin.Context, request validation.IRequest) bool {
	// binding.Query без Bind: BindQuery сам отвечает 400 и кладёт ошибку в c.Errors, ответ ниже — 422
	err := bindWithCustomRules(c, request, binding.Query)
	if err != nil {
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
			out := numErrorDetails(c.Request.URL.Query(), parseNumTypeError)
			response.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("validation error"), out)

			return false
		}

		response.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("incorrect request body"), nil)

		return false
	}

	return true
}

// ValidateRequestPath - Валидация параметров пути HTTP реквеста (`/stations/:code`): поля запроса с тегом `uri`,
// правила — тегом `binding` и CustomValidationRules, как у ValidateRequestQuery. Ошибка — 422 с ключом по полю,
// без значения параметра в ответе
func ValidateRequestPath(c *gin.Context, request validation.IRequest) bool {
	err := bindPathWithCustomRules(c, request)
	if err == nil {
		return true
	}

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

	var parseNumTypeError *strconv.NumError

	if errors.As(err, &parseNumTypeError) {
		out := numErrorDetails(pathValues(c.Params), parseNumTypeError)
		response.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("validation error"), out)

		return false
	}

	response.ErrorResponseUntrackableSentry(c, http.StatusUnprocessableEntity, errors.New("incorrect request path"), nil)

	return false
}

// pathValues Параметры пути в виде url.Values — для binding.Uri и для поиска параметра по значению ошибки разбора
func pathValues(params gin.Params) url.Values {
	out := make(url.Values, len(params))

	for _, p := range params {
		out[p.Key] = append(out[p.Key], p.Value)
	}

	return out
}

// ValidateRequestBody - Валидация тела HTTP реквеста
func ValidateRequestBody(c *gin.Context, request validation.IRequest) bool {
	bodyBinding := binding.Default(c.Request.Method, c.ContentType())

	err := bindWithCustomRules(c, request, bodyBinding)
	if err != nil {
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

// numErrorDetails Ошибка разбора числа/флага по параметрам запроса: ключ — параметр со значением из ошибки,
// сообщение — без самого значения (ошибка gin поле не называет)
func numErrorDetails(query url.Values, numErr *strconv.NumError) map[string]any {
	msg := "Invalid value, expected number"
	if numErr.Func == "ParseBool" {
		msg = "Invalid value, expected boolean"
	}

	out := make(map[string]any)

	for key, values := range query {
		if slices.Contains(values, numErr.Num) {
			out[key] = msg
		}
	}

	if len(out) == 0 {
		return nil
	}

	return out
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
