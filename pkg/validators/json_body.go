package validators

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin/binding"
)

// errTrailingData Данные после первого JSON-значения тела
var errTrailingData = errors.New("request body has data after the JSON value")

// jsonOneValueBinding Привязка JSON-тела в ValidateRequestBody: как binding.JSON (тот же декодер gin, UseNumber,
// валидация), но данные после первого JSON-значения — ошибка (422 «incorrect request body»), а не молча отброшенный
// хвост: `{"a":1}{"a":2}` и `{"a":1}garbage` раньше проходили по первому объекту. Пробелы в конце — как раньше
type jsonOneValueBinding struct{}

func (jsonOneValueBinding) Name() string {
	return binding.JSON.Name()
}

func (jsonOneValueBinding) Bind(req *http.Request, obj any) error {
	if req == nil || req.Body == nil {
		return binding.JSON.Bind(req, obj)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}

	if err := binding.JSON.BindBody(body, obj); err != nil {
		return err
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
