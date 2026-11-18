package app

import (
	"testing"
	"time"
)

// WARN при старте — HANDLER_TIMEOUT не меньше срока записи сервера; без таймаута обработчика или записи — нет.
func TestHandlerTimeoutWarning(t *testing.T) {
	cases := []struct {
		handler int
		write   time.Duration
		warn    bool
	}{
		{handler: 0, write: 30 * time.Second},
		{handler: 10, write: 30 * time.Second},
		{handler: 30, write: 30 * time.Second, warn: true},
		{handler: 60, write: 30 * time.Second, warn: true},
		{handler: 60, write: 0},
	}

	for _, tc := range cases {
		if got := handlerTimeoutWarning(tc.handler, tc.write) != ""; got != tc.warn {
			t.Errorf("handlerTimeoutWarning(%d, %s) warn = %v, want %v", tc.handler, tc.write, got, tc.warn)
		}
	}
}
