package validators

import (
	"os"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/iancoleman/strcase"
)

// ValidationKeysEnv Ключи ошибок 422. json — имя из тега источника (тело — json, затем form, uri; query — form, затем
// json, uri; путь — uri, затем form, json; нет тега — имя поля в snake_case)
// и полный путь вложенного поля (`items[2].component_ids`). Не задана и любое другое значение — как раньше:
// последнее имя Go-поля в snake_case (`ComponentIDs` → `component_i_ds`).
const ValidationKeysEnv = "VALIDATION_KEYS"

// Порядок тегов имени поля по источнику запроса.
var (
	tagsBody  = []string{"json", "form", "uri"}
	tagsQuery = []string{"form", "json", "uri"}
	tagsPath  = []string{"uri", "form", "json"}
)

func jsonKeys() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(ValidationKeysEnv)), "json")
}

// fieldErrorKey Ключ ошибки поля в ответе 422 для запроса request; tags — порядок тегов источника.
func fieldErrorKey(request any, fe validator.FieldError, tags []string) string {
	if !jsonKeys() {
		return strcase.ToSnake(fe.Field())
	}

	if key := tagPath(reflect.TypeOf(request), fe.StructNamespace(), tags); key != "" {
		return key
	}

	return strcase.ToSnake(fe.Field())
}

// typeErrorKey Ключ ошибки типа JSON (UnmarshalTypeError.Field — уже путь по именам JSON).
func typeErrorKey(field string) string {
	if jsonKeys() {
		return field
	}

	return strcase.ToSnake(field)
}

// tagPath Путь поля по тегам: namespace валидатора `Request.Items[2].ComponentIDs` → `items[2].component_ids`.
// Встроенные структуры в путь не входят. Не нашли поле — "".
func tagPath(t reflect.Type, namespace string, tags []string) string {
	parts := strings.Split(namespace, ".")
	if len(parts) < 2 {
		return ""
	}

	out := make([]string, 0, len(parts)-1)

	for _, part := range parts[1:] {
		name, index := part, ""
		if i := strings.IndexByte(part, '['); i >= 0 {
			name, index = part[:i], part[i:]
		}

		t = elem(t)
		if t.Kind() != reflect.Struct {
			return ""
		}

		field, ok := t.FieldByName(name)
		if !ok {
			return ""
		}

		t = field.Type
		if field.Anonymous {
			continue
		}

		out = append(out, tagName(field, tags)+index)

		if index != "" {
			t = elem(t)
			if t.Kind() == reflect.Slice || t.Kind() == reflect.Array || t.Kind() == reflect.Map {
				t = t.Elem()
			}
		}
	}

	return strings.Join(out, ".")
}

func elem(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t
}

// tagName Имя поля по первому заданному тегу из tags; без тега (или «-») — имя поля в snake_case.
func tagName(field reflect.StructField, tags []string) string {
	for _, tag := range tags {
		name, _, _ := strings.Cut(field.Tag.Get(tag), ",")
		if name != "" && name != "-" {
			return name
		}
	}

	return strcase.ToSnake(field.Name)
}
