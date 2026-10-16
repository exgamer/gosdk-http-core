package validators

import (
	"reflect"
	"sync"

	"github.com/exgamer/gosdk-http-core/pkg/gin/validation"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// ruleKey Правило конкретного типа запроса: одно имя правила у разных запросов может означать разные функции
type ruleKey struct {
	request reflect.Type
	name    string
}

// customRules Пользовательские правила, уже зарегистрированные в валидаторе gin.
// RegisterValidation пишет в map валидатора без блокировки, а Struct читает её при разборе нового типа,
// поэтому регистрация (Lock) не идёт одновременно с биндингом через ValidateRequestBody/Query (RLock)
var customRules = struct {
	sync.RWMutex
	registered map[ruleKey]struct{}
}{registered: make(map[ruleKey]struct{})}

// bindWithCustomRules Регистрирует правила запроса один раз на тип запроса и имя правила, затем биндит
// (c.ShouldBindWith — то же, что ShouldBind/ShouldBindQuery с выбранным binding)
func bindWithCustomRules(c *gin.Context, request validation.IRequest, b binding.Binding) error {
	registerCustomRules(request)

	customRules.RLock()
	defer customRules.RUnlock()

	return c.ShouldBindWith(request, b)
}

func registerCustomRules(request validation.IRequest) {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}

	rules := request.CustomValidationRules()
	if len(rules) == 0 {
		return
	}

	requestType := reflect.TypeOf(request)
	if allRegistered(requestType, rules) {
		return
	}

	customRules.Lock()
	defer customRules.Unlock()

	for name, fn := range rules {
		key := ruleKey{request: requestType, name: name}
		if _, done := customRules.registered[key]; done {
			continue
		}

		// ошибка — пустое имя или nil-функция: как и раньше, правило не регистрируется
		if err := v.RegisterValidation(name, fn); err != nil {
			continue
		}

		customRules.registered[key] = struct{}{}
	}
}

func allRegistered(requestType reflect.Type, rules map[string]validator.Func) bool {
	customRules.RLock()
	defer customRules.RUnlock()

	for name := range rules {
		if _, done := customRules.registered[ruleKey{request: requestType, name: name}]; !done {
			return false
		}
	}

	return true
}
