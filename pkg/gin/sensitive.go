package gin

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
)

// SensitiveKeysEnv Переменная окружения с дополнительными чувствительными ключами (через запятую, без учёта регистра):
// значения таких параметров query и заголовков маскируются в событиях Sentry и в debug-логе запроса.
const SensitiveKeysEnv = "SENSITIVE_KEYS"

// SensitiveKeysAllowEnv Ключи (через запятую, без учёта регистра), которые не маскируются, даже если подходят под
// правила SDK или SENSITIVE_KEYS: например state, если он в сервисе не секрет.
const SensitiveKeysAllowEnv = "SENSITIVE_KEYS_ALLOW"

// sensitiveExact Ключи, маскируемые по полному совпадению (без учёта регистра): заголовки доступа и параметры OAuth/OIDC.
var sensitiveExact = map[string]struct{}{
	"authorization": {}, "proxy-authorization": {}, "cookie": {}, "set-cookie": {}, "x-api-key": {}, "x-auth-token": {},
	"password": {}, "token": {}, "access_token": {}, "refresh_token": {}, "id_token": {}, "client_secret": {},
	"code": {}, "state": {},
	// iin — ИИН (персональные данные): заголовок Iin и одноимённые поля
	"iin": {},
}

// sensitiveParts Части ключа, по которым он чувствителен где угодно в имени.
var sensitiveParts = []string{"password", "apikey", "api_key", "api-key"}

// sensitiveSuffixes Окончания чувствительного ключа.
var sensitiveSuffixes = []string{"-token", "_token", "-secret", "_secret", "-key", "_key"}

// notSensitive Ключи с «чувствительным» окончанием, которые секретом не являются.
var notSensitive = map[string]struct{}{"x-idempotency-key": {}, "idempotency-key": {}, "idempotency_key": {}}

// IsSensitiveKey Чувствителен ли параметр query или заголовок name: полное совпадение со списком SDK, окончание
// (-token, _secret, _key…), password/apikey в имени или ключ из SENSITIVE_KEYS; ключ из SENSITIVE_KEYS_ALLOW — нет.
func IsSensitiveKey(name string) bool {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" || inEnvList(SensitiveKeysAllowEnv, key) {
		return false
	}

	if _, ok := sensitiveExact[key]; ok {
		return true
	}

	if inEnvList(SensitiveKeysEnv, key) {
		return true
	}

	if _, ok := notSensitive[key]; ok {
		return false
	}

	for _, part := range sensitiveParts {
		if strings.Contains(key, part) {
			return true
		}
	}

	for _, suffix := range sensitiveSuffixes {
		if strings.HasSuffix(key, suffix) {
			return true
		}
	}

	return false
}

// inEnvList Есть ли key (в нижнем регистре) в списке через запятую переменной env.
func inEnvList(env, key string) bool {
	for _, item := range strings.Split(os.Getenv(env), ",") {
		if e := strings.ToLower(strings.TrimSpace(item)); e != "" && e == key {
			return true
		}
	}

	return false
}

// MaskHeaders Копия заголовков, у которых значения чувствительных ключей заменены на mask.
func MaskHeaders(h http.Header, mask string) http.Header {
	out := h.Clone()

	for key := range out {
		if IsSensitiveKey(key) {
			out[key] = []string{mask}
		}
	}

	return out
}

// MaskQuery Копия параметров query, у которых значения чувствительных ключей заменены на mask.
func MaskQuery(q url.Values, mask string) url.Values {
	out := make(url.Values, len(q))

	for key, values := range q {
		if IsSensitiveKey(key) {
			out[key] = []string{mask}

			continue
		}

		out[key] = append([]string(nil), values...)
	}

	return out
}

// MaskJSONKeys Копия разобранного JSON (map[string]any, []any, значения), в которой значения чувствительных ключей
// (IsSensitiveKey) на любой глубине заменены на mask. Исходное значение не меняется.
func MaskJSONKeys(v any, mask string) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))

		for key, value := range t {
			if IsSensitiveKey(key) {
				out[key] = mask

				continue
			}

			out[key] = MaskJSONKeys(value, mask)
		}

		return out
	case []any:
		out := make([]any, len(t))
		for i, value := range t {
			out[i] = MaskJSONKeys(value, mask)
		}

		return out
	default:
		return v
	}
}

// MaskBodyString Тело запроса строкой с замаскированными секретами: JSON-объект или массив — значения чувствительных
// ключей (MaskJSONKeys), тело формы (`a=1&password=x`) с чувствительным параметром — его значение; прочее — как есть.
func MaskBodyString(body, mask string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return body
	}

	if trimmed[0] == '{' || trimmed[0] == '[' {
		decoder := json.NewDecoder(strings.NewReader(trimmed))
		decoder.UseNumber()

		var v any
		if err := decoder.Decode(&v); err == nil {
			if b, err := json.Marshal(MaskJSONKeys(v, mask)); err == nil {
				return string(b)
			}
		}

		return body
	}

	if values, err := url.ParseQuery(trimmed); err == nil && hasSensitiveKey(values) {
		return encodeMaskedQuery(values, mask)
	}

	return body
}

// encodeMaskedQuery Параметры формы строкой (ключи по алфавиту, как url.Values.Encode); значение чувствительного
// ключа — mask без экранирования.
func encodeMaskedQuery(values url.Values, mask string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	var b strings.Builder

	for _, key := range keys {
		vs := values[key]
		if IsSensitiveKey(key) {
			vs = []string{""}
		}

		for _, v := range vs {
			if b.Len() > 0 {
				b.WriteByte('&')
			}

			b.WriteString(url.QueryEscape(key))
			b.WriteByte('=')

			if IsSensitiveKey(key) {
				b.WriteString(mask)

				continue
			}

			b.WriteString(url.QueryEscape(v))
		}
	}

	return b.String()
}

func hasSensitiveKey(values url.Values) bool {
	for key := range values {
		if IsSensitiveKey(key) {
			return true
		}
	}

	return false
}
