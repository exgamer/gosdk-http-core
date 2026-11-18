package validators

import (
	"reflect"
	"time"
)

// maxFieldDepth Глубина вложенных структур при обходе полей запроса.
const maxFieldDepth = 16

var timeType = reflect.TypeOf(time.Time{})

// hasFormTags Есть ли у структуры запроса (в том числе во вложенных и встроенных структурах) поле с тегом form:
// тогда тело формы (x-www-form-urlencoded, multipart) разбирается в неё; иначе форма — 415.
func hasFormTags(t reflect.Type) bool {
	return hasFormTagsDepth(baseType(t), 0)
}

func hasFormTagsDepth(t reflect.Type, depth int) bool {
	if t == nil || t.Kind() != reflect.Struct || t == timeType || depth > maxFieldDepth {
		return false
	}

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if tag, ok := sf.Tag.Lookup("form"); ok && tag != "-" {
			return true
		}

		ft := baseType(sf.Type)
		if ft.Kind() == reflect.Struct && hasFormTagsDepth(ft, depth+1) {
			return true
		}
	}

	return false
}

// fieldGuard Восстанавливает часть запроса после биндинга; true — биндинг её изменил.
type fieldGuard func() bool

// protectHiddenFields Запоминает поля запроса с `json:"-"` без тега form (gin заполнил бы их из query и формы по имени
// поля: `?UserID=1`) и возвращает функцию, которая восстанавливает их значения после биндинга; true — какое-то поле
// было изменено биндингом. Вложенные структуры — и по указателю: заданный указатель обходится как структура, у nil
// указателя, который биндинг заполнил, скрытые поля новой структуры обнуляются.
func protectHiddenFields(obj any) func() bool {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return func() bool { return false }
	}

	var guards []fieldGuard

	collectHidden(v.Elem(), &guards, 0)

	return func() bool {
		changed := false

		for _, restore := range guards {
			if restore() {
				changed = true
			}
		}

		return changed
	}
}

// isHidden Поле с `json:"-"` без тега form: клиент его не задаёт.
func isHidden(sf reflect.StructField) bool {
	_, hasForm := sf.Tag.Lookup("form")

	return sf.Tag.Get("json") == "-" && !hasForm
}

// structPointer Указатель на структуру (не time.Time).
func structPointer(t reflect.Type) bool {
	return t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct && t.Elem() != timeType
}

func collectHidden(v reflect.Value, out *[]fieldGuard, depth int) {
	if v.Kind() != reflect.Struct || v.Type() == timeType || depth > maxFieldDepth {
		return
	}

	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		fv := v.Field(i)

		if isHidden(sf) {
			saveValue(fv, out, depth)

			continue
		}

		if sf.Tag.Get("form") == "-" {
			continue
		}

		switch {
		case sf.Type.Kind() == reflect.Struct:
			collectHidden(fv, out, depth+1)
		case structPointer(sf.Type) && !fv.IsNil():
			collectHidden(fv.Elem(), out, depth+1)
		case structPointer(sf.Type) && fv.CanSet():
			// nil: биндинг может создать структуру и заполнить её скрытые поля
			*out = append(*out, func() bool {
				return !fv.IsNil() && zeroHidden(fv.Elem(), depth+1)
			})
		}
	}
}

// saveValue Запоминает значение скрытого поля. Заданный указатель gin заполняет на месте, не меняя сам указатель, —
// поэтому запоминается и значение под ним, а у структуры под указателем — её поля вглубь.
func saveValue(fv reflect.Value, out *[]fieldGuard, depth int) {
	if !fv.CanSet() {
		return
	}

	value := reflect.New(fv.Type()).Elem()
	value.Set(fv)

	if fv.Kind() != reflect.Pointer || fv.IsNil() {
		*out = append(*out, func() bool {
			if reflect.DeepEqual(fv.Interface(), value.Interface()) {
				return false
			}

			fv.Set(value)

			return true
		})

		return
	}

	elem := reflect.New(fv.Type().Elem()).Elem()
	elem.Set(fv.Elem())
	*out = append(*out, func() bool {
		changed := false

		if fv.Pointer() != value.Pointer() {
			fv.Set(value)

			changed = true
		}

		if !reflect.DeepEqual(value.Elem().Interface(), elem.Interface()) {
			value.Elem().Set(elem)

			changed = true
		}

		return changed
	})

	if structPointer(fv.Type()) && depth <= maxFieldDepth {
		saveStruct(fv.Elem(), out, depth+1)
	}
}

// saveStruct Запоминает все поля структуры внутри скрытого поля.
func saveStruct(v reflect.Value, out *[]fieldGuard, depth int) {
	if v.Type() == timeType || depth > maxFieldDepth {
		return
	}

	for i := 0; i < v.NumField(); i++ {
		saveValue(v.Field(i), out, depth)
	}
}

// zeroHidden Обнуляет скрытые поля структуры, созданной биндингом; true — какое-то поле было заполнено.
func zeroHidden(v reflect.Value, depth int) bool {
	if v.Kind() != reflect.Struct || v.Type() == timeType || depth > maxFieldDepth {
		return false
	}

	changed := false
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		fv := v.Field(i)

		switch {
		case isHidden(sf):
			if fv.CanSet() && !fv.IsZero() {
				fv.SetZero()

				changed = true
			}
		case sf.Tag.Get("form") == "-":
		case sf.Type.Kind() == reflect.Struct:
			changed = zeroHidden(fv, depth+1) || changed
		case structPointer(sf.Type) && !fv.IsNil():
			changed = zeroHidden(fv.Elem(), depth+1) || changed
		}
	}

	return changed
}
