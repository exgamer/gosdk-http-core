package validators

import (
	"errors"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/exgamer/gosdk-http-core/pkg/gin/validation"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// ruleSet Функции одного имени правила по типам запросов. first — функция первого зарегистрировавшего типа: её
// получает тип, который пользуется тегом, не объявив правило в своих CustomValidationRules (как раньше — общее
// правило по имени). Набор не меняется после публикации: регистрация публикует новую копию.
type ruleSet struct {
	byType map[reflect.Type]validator.Func
	first  validator.Func
}

// rulesTable Пользовательские правила по имени (copy-on-write: читается без блокировки из валидации).
var rulesTable atomic.Pointer[map[string]ruleSet]

func init() {
	empty := make(map[string]ruleSet)
	rulesTable.Store(&empty)
}

// customRules RegisterValidation пишет в map валидатора без блокировки, а Struct читает её при разборе нового типа,
// поэтому регистрация (Lock) не идёт одновременно с биндингом через ValidateRequestBody/Query/Path (RLock).
// Тело запроса читается до RLock: медленный клиент не держит блокировку и не задерживает регистрацию правил.
var customRules sync.RWMutex

// bindWithCustomRules Регистрирует правила запроса, затем биндит параметры query (binding.Query) или форму
// (binding.Form, binding.FormMultipart); форма разбирается до блокировки. Поля с `json:"-"` без тега form из query и
// формы не заполняются (gin иначе ищет параметр по имени поля): значение до биндинга восстанавливается, правила
// проверяются заново.
func bindWithCustomRules(c *gin.Context, request validation.IRequest, b binding.Binding) error {
	if b == binding.Form || b == binding.FormMultipart {
		if err := c.Request.ParseForm(); err != nil {
			return err
		}

		if err := c.Request.ParseMultipartForm(multipartMemory); err != nil && !errors.Is(err, http.ErrNotMultipart) {
			return err
		}
	}

	registerCustomRules(request)

	restore := protectHiddenFields(request)

	customRules.RLock()
	defer customRules.RUnlock()

	err := c.ShouldBindWith(request, b)

	if restore() {
		var ve validator.ValidationErrors
		if err == nil || errors.As(err, &ve) {
			err = binding.Validator.ValidateStruct(request)
		}
	}

	return err
}

// multipartMemory Память под multipart-форму (как у gin): остальное — во временных файлах.
const multipartMemory = 32 << 20

// bindBodyWithCustomRules Регистрирует правила запроса и биндит уже прочитанное тело.
func bindBodyWithCustomRules(request validation.IRequest, b binding.BindingBody, body []byte) error {
	registerCustomRules(request)

	customRules.RLock()
	defer customRules.RUnlock()

	return b.BindBody(body, request)
}

// bindPathWithCustomRules То же для параметров пути (c.ShouldBindUri с регистрацией правил запроса)
func bindPathWithCustomRules(c *gin.Context, request validation.IRequest) error {
	registerCustomRules(request)

	customRules.RLock()
	defer customRules.RUnlock()

	return binding.Uri.BindUri(pathValues(c.Params), request)
}

// registerCustomRules Правила запроса — в таблицу по типу запроса. В валидаторе gin имя правила одно на процесс,
// поэтому под каждым именем регистрируется диспетчер: он вызывает функцию того запроса, который валидируется
// (fl.Top()). Одно имя у разных запросов с разными функциями — не конфликт: каждый запрос проверяется своей функцией,
// в том числе во вложенных структурах и независимо от порядка первых запросов.
func registerCustomRules(request validation.IRequest) {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}

	rules := request.CustomValidationRules()
	if len(rules) == 0 {
		return
	}

	requestType := baseType(reflect.TypeOf(request))
	if allRegistered(requestType, rules) {
		return
	}

	customRules.Lock()
	defer customRules.Unlock()

	current := *rulesTable.Load()
	next := make(map[string]ruleSet, len(current)+len(rules))

	for name, set := range current {
		next[name] = set
	}

	newNames := make([]string, 0, len(rules))

	for name, fn := range rules {
		// пустое имя или nil-функция: как и раньше, правило не регистрируется
		if name == "" || fn == nil {
			continue
		}

		set, exists := next[name]
		if _, done := set.byType[requestType]; done {
			continue
		}

		byType := make(map[reflect.Type]validator.Func, len(set.byType)+1)
		for t, f := range set.byType {
			byType[t] = f
		}

		byType[requestType] = fn

		first := set.first
		if first == nil {
			first = fn
		}

		next[name] = ruleSet{byType: byType, first: first}

		if !exists {
			newNames = append(newNames, name)
		}
	}

	rulesTable.Store(&next)

	for _, name := range newNames {
		_ = v.RegisterValidation(name, dispatchRule(name))
	}
}

// dispatchRule Функция правила name для валидатора: функция типа валидируемого запроса, иначе — первая
// зарегистрированная под этим именем.
func dispatchRule(name string) validator.Func {
	return func(fl validator.FieldLevel) bool {
		set, ok := (*rulesTable.Load())[name]
		if !ok || set.first == nil {
			return true
		}

		if top := fl.Top(); top.IsValid() {
			if fn, ok := set.byType[baseType(top.Type())]; ok {
				return fn(fl)
			}
		}

		return set.first(fl)
	}
}

// baseType Тип без указателей: запрос регистрируется как *T, а валидируется как *T или T.
func baseType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t
}

func allRegistered(requestType reflect.Type, rules map[string]validator.Func) bool {
	table := *rulesTable.Load()

	for name, fn := range rules {
		if name == "" || fn == nil {
			continue
		}

		if _, done := table[name].byType[requestType]; !done {
			return false
		}
	}

	return true
}
