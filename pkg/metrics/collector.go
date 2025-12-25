package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"sync"
)

const (
	MetricNameHttpRequest = "http_request_metrics_info"
	MetricLabelHttpStatus = "status"
	MetricLabelHttpMethod = "method"
	MetricLabelHttpUrl    = "url"
)

func NewCollector(serviceName string) *Collector {
	m := &Collector{serviceName: serviceName}
	m.init()
	m.register()

	return m
}

type Collector struct {
	serviceName        string
	httpRequestMetrics *prometheus.HistogramVec
	once               sync.Once
}

func (m *Collector) init() {
	m.httpRequestMetrics = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        MetricNameHttpRequest,
			Help:        "Histogram of response time for handler in seconds.",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: prometheus.Labels{"service": m.serviceName},
		},
		[]string{MetricLabelHttpStatus, MetricLabelHttpMethod, MetricLabelHttpUrl},
	)
}

func (m *Collector) register() {
	m.once.Do(func() {
		prometheus.MustRegister(m.httpRequestMetrics)
	})
}

func (m *Collector) GetMetrics(statusCode int, method string, path string, duration float64) {
	m.httpRequestMetrics.
		WithLabelValues(strconv.Itoa(statusCode), method, path).
		Observe(duration)
}
