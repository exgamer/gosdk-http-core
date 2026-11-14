package gin

import (
	"net/http"
	"net/url"
	"testing"
)

func TestIsSensitiveKey(t *testing.T) {
	t.Setenv(SensitiveKeysEnv, " Otp , pin")

	for key, want := range map[string]bool{
		"code": true, "STATE": true, "access_token": true, "refresh_token": true, "id_token": true, "client_secret": true,
		"password": true, "new_password": true, "X-Api-Key": true, "X-Auth-Token": true, "Authorization": true,
		"Proxy-Authorization": true, "Cookie": true, "Set-Cookie": true, "X-Csrf-Token": true, "webhook_secret": true,
		"private_key": true, "otp": true, "PIN": true,
		"author_id": false, "author": false, "authority": false, "session_count": false, "X-Idempotency-Key": false,
		"page": false, "per_page": false, "Accept": false, "X-Request-Id": false, "": false,
	} {
		if got := IsSensitiveKey(key); got != want {
			t.Errorf("IsSensitiveKey(%q) = %v, want %v", key, got, want)
		}
	}
}

// SENSITIVE_KEYS_ALLOW unmasks a key, even one of the SDK list.
func TestIsSensitiveKey_Allow(t *testing.T) {
	t.Setenv(SensitiveKeysAllowEnv, "state")

	if IsSensitiveKey("state") || !IsSensitiveKey("code") {
		t.Fatal("SENSITIVE_KEYS_ALLOW=state: state must not be masked, code must")
	}
}

func TestMaskQueryAndHeaders(t *testing.T) {
	q := MaskQuery(url.Values{"code": {"x"}, "page": {"2"}}, "*****")
	if q.Get("code") != "*****" || q.Get("page") != "2" {
		t.Fatalf("MaskQuery = %v", q)
	}

	src := http.Header{"X-Api-Key": {"k"}, "Accept": {"a"}}
	h := MaskHeaders(src, "***")
	if h.Get("X-Api-Key") != "***" || h.Get("Accept") != "a" || src.Get("X-Api-Key") != "k" {
		t.Fatalf("MaskHeaders = %v, source = %v", h, src)
	}
}
