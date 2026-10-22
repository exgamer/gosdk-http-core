package gin

import "testing"

type boolSettingCase struct {
	name       string
	settings   []string
	enabled    bool
	recognized bool
}

func TestParseBoolSetting(t *testing.T) {
	cases := []boolSettingCase{
		{name: "on", settings: []string{"true", "1", " TRUE ", "True", " 1 ", "\ttrue\n"}, enabled: true, recognized: true},
		{name: "off", settings: []string{"", "   ", "\t\n", "false", "0", " False ", "FALSE"}, enabled: false, recognized: true},
		{
			name: "unrecognized: not strconv.ParseBool, no yes/on",
			settings: []string{
				"yes", "on", "t", "T", "f", "F", "enabled", "2", "-1", "true1", "tru e", "ture", "no", "off", "да",
			},
			enabled:    false,
			recognized: false,
		},
	}

	for _, tc := range cases {
		for _, setting := range tc.settings {
			enabled, recognized := parseBoolSetting(setting)
			if enabled != tc.enabled || recognized != tc.recognized {
				t.Errorf("%s: parseBoolSetting(%q) = (%v, %v), want (%v, %v)",
					tc.name, setting, enabled, recognized, tc.enabled, tc.recognized)
			}

			// One parser for both settings.
			swEnabled, swRecognized := swaggerEnabled(setting)
			if swEnabled != enabled || swRecognized != recognized {
				t.Errorf("swaggerEnabled(%q) = (%v, %v), parseBoolSetting = (%v, %v)",
					setting, swEnabled, swRecognized, enabled, recognized)
			}
		}
	}
}
