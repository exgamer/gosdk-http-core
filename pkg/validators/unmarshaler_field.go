package validators

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
)

// unmarshalerFieldError Ошибка собственного UnmarshalJSON поля тела: Field — путь поля по именам JSON, как у
// json.UnmarshalTypeError.Field (`items.date`, без индексов), Err — ошибка декодера
type unmarshalerFieldError struct {
	Field string
	Err   error
}

func (e *unmarshalerFieldError) Error() string {
	return e.Err.Error()
}

func (e *unmarshalerFieldError) Unwrap() error {
	return e.Err
}

// withUnmarshalerField Ошибка разбора тела с полем, чей UnmarshalJSON её вернул. encoding/json поле не называет,
// поэтому тело разбирается заново по полям запроса obj (в порядке ключей тела), пока ошибка поля не совпадёт с err.
// Ошибки типа и синтаксиса, а также не найденное поле — err как есть
func withUnmarshalerField(body []byte, obj any, err error) error {
	var typeErr *json.UnmarshalTypeError
	var syntaxErr *json.SyntaxError

	if errors.As(err, &typeErr) || errors.As(err, &syntaxErr) {
		return err
	}

	if field, ok := unmarshalerField(body, reflect.TypeOf(obj), "", err.Error()); ok {
		return &unmarshalerFieldError{Field: field, Err: err}
	}

	return err
}

// unmarshalerField Путь поля структуры t, разбор значения которого из объекта raw даёт ошибку с текстом target.
// Вложенные структуры и срезы структур — вглубь; поле со своим UnmarshalJSON, map и прочее — путь самого поля
func unmarshalerField(raw []byte, t reflect.Type, prefix, target string) (string, bool) {
	t = elem(t)
	if t == nil || t.Kind() != reflect.Struct {
		return "", false
	}

	members, ok := objectMembers(raw)
	if !ok {
		return "", false
	}

	fields := jsonFields(t)

	for _, m := range members {
		f, ok := matchJSONField(fields, m.key)
		if !ok {
			continue
		}

		err := json.Unmarshal(m.value, reflect.New(f.typ).Interface())
		if err == nil || err.Error() != target {
			continue
		}

		path := f.name
		if prefix != "" {
			path = prefix + "." + f.name
		}

		if deeper, ok := nestedUnmarshalerField(m.value, f.typ, path, target); ok {
			return deeper, true
		}

		return path, true
	}

	return "", false
}

// nestedUnmarshalerField Поиск внутри значения поля: структура без своего UnmarshalJSON — по её полям, срез или
// массив таких структур — по элементам (в пути без индекса, как у json.UnmarshalTypeError)
func nestedUnmarshalerField(raw []byte, t reflect.Type, path, target string) (string, bool) {
	t = elem(t)
	if t == nil || hasUnmarshaler(t) {
		return "", false
	}

	switch t.Kind() {
	case reflect.Struct:
		return unmarshalerField(raw, t, path, target)
	case reflect.Slice, reflect.Array:
		item := elem(t.Elem())
		if item == nil || item.Kind() != reflect.Struct || hasUnmarshaler(item) {
			return "", false
		}

		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return "", false
		}

		for _, v := range items {
			if field, ok := unmarshalerField(v, item, path, target); ok {
				return field, true
			}
		}
	}

	return "", false
}

var unmarshalerType = reflect.TypeFor[json.Unmarshaler]()

// hasUnmarshaler Тип (или указатель на него) разбирается своим UnmarshalJSON
func hasUnmarshaler(t reflect.Type) bool {
	return t.Implements(unmarshalerType) || reflect.PointerTo(t).Implements(unmarshalerType)
}

// jsonMember Ключ и значение объекта тела в порядке следования
type jsonMember struct {
	key   string
	value json.RawMessage
}

// objectMembers Ключи и значения JSON-объекта raw по порядку; не объект — false
func objectMembers(raw []byte) ([]jsonMember, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))

	if tok, err := decoder.Token(); err != nil || tok != json.Delim('{') {
		return nil, false
	}

	var out []jsonMember

	for decoder.More() {
		tok, err := decoder.Token()
		if err != nil {
			return nil, false
		}

		key, ok := tok.(string)
		if !ok {
			return nil, false
		}

		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, false
		}

		out = append(out, jsonMember{key: key, value: value})
	}

	return out, true
}

// jsonField Поле структуры, видимое encoding/json: имя в JSON и тип
type jsonField struct {
	name string
	typ  reflect.Type
}

// jsonFields Поля структуры t по правилам encoding/json: имя из тега json (иначе имя Go-поля), «-» — пропуск,
// поля встроенных структур без тега — как свои (внешние поля с тем же именем важнее)
func jsonFields(t reflect.Type) []jsonField {
	var out []jsonField

	seen := make(map[string]bool)

	var embedded []reflect.Type

	for i := range t.NumField() {
		f := t.Field(i)

		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}

		name, _, _ := strings.Cut(tag, ",")

		if f.Anonymous && name == "" {
			if ft := elem(f.Type); ft.Kind() == reflect.Struct {
				embedded = append(embedded, ft)

				continue
			}
		}

		if !f.IsExported() {
			continue
		}

		if name == "" {
			name = f.Name
		}

		if !seen[name] {
			seen[name] = true
			out = append(out, jsonField{name: name, typ: f.Type})
		}
	}

	for _, et := range embedded {
		for _, f := range jsonFields(et) {
			if !seen[f.name] {
				seen[f.name] = true
				out = append(out, f)
			}
		}
	}

	return out
}

// matchJSONField Поле по ключу тела: точное имя, затем без учёта регистра — как encoding/json
func matchJSONField(fields []jsonField, key string) (jsonField, bool) {
	for _, f := range fields {
		if f.name == key {
			return f, true
		}
	}

	for _, f := range fields {
		if strings.EqualFold(f.name, key) {
			return f, true
		}
	}

	return jsonField{}, false
}
