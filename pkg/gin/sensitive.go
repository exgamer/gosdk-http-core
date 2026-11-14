package gin

import (
	"net/http"
	"net/url"
	"os"
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
