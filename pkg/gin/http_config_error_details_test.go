package gin

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	baseConfig "github.com/exgamer/gosdk-core/pkg/config"

	"github.com/exgamer/gosdk-http-core/pkg/config"
)

func TestHttpConfig_ErrorDetailsFromEnv(t *testing.T) {
	cases := []struct {
		name  string
		value string
		set   bool
	}{
		{name: "raw value", value: " TRUE ", set: true},
		{name: "unset"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setErrorDetailsEnv(t, tc.value, tc.set)

			cfg := config.HttpConfig{}
			if err := baseConfig.InitConfig(&cfg); err != nil {
				t.Fatalf("InitConfig: %v", err)
			}

			if cfg.ErrorDetails != tc.value {
				t.Errorf("ErrorDetails = %q, want %q", cfg.ErrorDetails, tc.value)
			}
		})
	}
}

func TestHttpConfig_ErrorDetailsJSON(t *testing.T) {
	data, err := json.Marshal(config.HttpConfig{ErrorDetails: "1"})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	if !strings.Contains(string(data), `"http_error_details":"1"`) {
		t.Errorf("json = %s, want \"http_error_details\":\"1\"", data)
	}
}

func setErrorDetailsEnv(t *testing.T, value string, set bool) {
	t.Helper()

	t.Setenv("HTTP_ERROR_DETAILS", value)
	if set {
		return
	}

	if err := os.Unsetenv("HTTP_ERROR_DETAILS"); err != nil {
		t.Fatalf("unset HTTP_ERROR_DETAILS: %v", err)
	}
}
