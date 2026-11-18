package validators

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/exgamer/gosdk-http-core/pkg/gin/validation"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// errTrailingData Данные после первого JSON-значения тела
var errTrailingData = errors.New("request body has data after the JSON value")

// jsonOneValueBinding Привязка JSON-тела в ValidateRequestBody: как binding.JSON (тот же декодер gin, UseNumber,
// валидация), но данные после первого JSON-значения — ошибка (422 «incorrect request body»), а не молча отброшенный
// хвост: `{"a":1}{"a":2}` и `{"a":1}garbage` раньше проходили по первому объекту. Пробелы в конце — как раньше.
// Ошибка собственного UnmarshalJSON поля — unmarshalerFieldError с путём поля (ответ 422 с ключом поля)
type jsonOneValueBinding struct{}

func (jsonOneValueBinding) Name() string {
	return binding.JSON.Name()
}

func (b jsonOneValueBinding) Bind(req *http.Request, obj any) error {
	if req == nil || req.Body == nil {
		return binding.JSON.Bind(req, obj)
	}

	body, err := io.ReadAll(http.MaxBytesReader(nil, req.Body, maxBodyBytes))
	if err != nil {
		return err
	}

	return b.BindBody(body, obj)
}

func (jsonOneValueBinding) BindBody(body []byte, obj any) error {
	if err := binding.JSON.BindBody(body, obj); err != nil {
		return withUnmarshalerField(body, obj, err)
	}

	decoder := json.NewDecoder(bytes.NewReader(body))

	var first json.RawMessage
	if err := decoder.Decode(&first); err != nil {
		return err
	}

	if err := decoder.Decode(&json.RawMessage{}); !errors.Is(err, io.EOF) {
		return errTrailingData
	}

	return nil
}

// maxBodyBytes Предел тела, которое ValidateRequestBody читает в память (JSON, XML и прочие не-формы): 10 МиБ.
// Больше — 413 (*http.MaxBytesError). Формы и загрузки файлов (multipart) этим пределом не ограничиваются.
const maxBodyBytes = 10 << 20

// errUnsupportedMediaType Тип тела, который ValidateRequestBody не разбирает в запрос (ответ 415).
var errUnsupportedMediaType = errors.New("unsupported media type")

// readBody Тело запроса целиком, не больше maxBodyBytes (больше — *http.MaxBytesError). Читается до блокировки правил.
func readBody(c *gin.Context) ([]byte, error) {
	if c.Request.Body == nil || c.Request.Body == http.NoBody {
		return nil, nil
	}

	return io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes))
}

// isJSONContentType application/json и application/*+json (merge-patch+json, problem+json и т. п.).
func isJSONContentType(ct string) bool {
	return ct == binding.MIMEJSON || (strings.HasPrefix(ct, "application/") && strings.HasSuffix(ct, "+json"))
}

// bindBody Биндинг тела ValidateRequestBody по Content-Type:
//   - GET — параметры query, как раньше (binding.Default для GET);
//   - без Content-Type, application/json, application/*+json — JSON (пустое тело без Content-Type — пустой объект);
//   - x-www-form-urlencoded, multipart/form-data — форма, только если у запроса есть теги form; иначе 415;
//   - XML, YAML, TOML, protobuf, msgpack — как раньше (binding.Default);
//   - прочее — 415.
//
// Тело (кроме формы) читается до блокировки правил и не больше maxBodyBytes (больше — 413).
func bindBody(c *gin.Context, request validation.IRequest) error {
	if c.Request.Method == http.MethodGet {
		return bindWithCustomRules(c, request, binding.Form)
	}

	ct := strings.ToLower(strings.TrimSpace(c.ContentType()))

	switch {
	case ct == "" || isJSONContentType(ct):
		body, err := readBody(c)
		if err != nil {
			return err
		}

		if ct == "" && len(bytes.TrimSpace(body)) == 0 {
			body = []byte("{}")
		}

		return bindBodyWithCustomRules(request, jsonOneValueBinding{}, body)
	case ct == binding.MIMEPOSTForm || ct == binding.MIMEMultipartPOSTForm:
		if !hasFormTags(reflect.TypeOf(request)) {
			return errUnsupportedMediaType
		}

		return bindWithCustomRules(c, request, binding.Default(c.Request.Method, ct))
	}

	bodyBinding, ok := binding.Default(c.Request.Method, ct).(binding.BindingBody)
	if !ok {
		return errUnsupportedMediaType
	}

	body, err := readBody(c)
	if err != nil {
		return err
	}

	return bindBodyWithCustomRules(request, bodyBinding, body)
}
