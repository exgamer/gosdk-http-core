package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// A second collector in the process (second app, tests) does not panic and writes into the same histogram.
func TestNewCollector_SecondDoesNotPanic(t *testing.T) {
	first := NewCollector("svc-test")

	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("second NewCollector panicked: %v", p)
		}
	}()

	second := NewCollector("svc-test")
	second.GetMetrics(200, "GET", "/x", 0.1)

	if first.httpRequestMetrics != second.httpRequestMetrics {
		t.Fatal("second collector must reuse the registered histogram")
	}

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range families {
		if f.GetName() == MetricNameHttpRequest && len(f.GetMetric()) == 1 {
			return
		}
	}

	t.Fatal("the observation of the second collector is not in the default registry")
}
