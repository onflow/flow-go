package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/module"

	httpmetrics "github.com/slok/go-http-metrics/metrics"
	metricsProm "github.com/slok/go-http-metrics/metrics/prometheus"
)

type RestCollector struct {
	httpRequestDurHistogram   *prometheus.HistogramVec
	httpResponseSizeHistogram *prometheus.HistogramVec
	httpRequestsInflight      *prometheus.GaugeVec
	httpRequestsTotal         *prometheus.GaugeVec
}

var _ module.RestMetrics = (*RestCollector)(nil)

// NewRestCollector returns a new metrics RestCollector that implements the RestCollector
// using Prometheus as the backend.
func NewRestCollector(cfg metricsProm.Config) module.RestMetrics {
	if len(cfg.DurationBuckets) == 0 {
		cfg.DurationBuckets = prometheus.DefBuckets
	}

	if len(cfg.SizeBuckets) == 0 {
		cfg.SizeBuckets = prometheus.ExponentialBuckets(100, 10, 8)
	}

	if cfg.Registry == nil {
		cfg.Registry = prometheus.DefaultRegisterer
	}

	if cfg.HandlerIDLabel == "" {
		cfg.HandlerIDLabel = "handler"
	}

	if cfg.StatusCodeLabel == "" {
		cfg.StatusCodeLabel = "code"
	}

	if cfg.MethodLabel == "" {
		cfg.MethodLabel = "method"
	}

	if cfg.ServiceLabel == "" {
		cfg.ServiceLabel = "service"
	}

	r := &RestCollector{
		httpRequestDurHistogram: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: cfg.Prefix,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "The latency of the HTTP requests.",
			Buckets:   cfg.DurationBuckets,
		}, []string{cfg.ServiceLabel, cfg.HandlerIDLabel, cfg.MethodLabel, cfg.StatusCodeLabel}),

		httpResponseSizeHistogram: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: cfg.Prefix,
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "The size of the HTTP responses.",
			Buckets:   cfg.SizeBuckets,
		}, []string{cfg.ServiceLabel, cfg.HandlerIDLabel, cfg.MethodLabel, cfg.StatusCodeLabel}),

		httpRequestsInflight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: cfg.Prefix,
			Subsystem: "http",
			Name:      "requests_inflight",
			Help:      "The number of inflight requests being handled at the same time.",
		}, []string{cfg.ServiceLabel, cfg.HandlerIDLabel}),

		httpRequestsTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: cfg.Prefix,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "The number of requests handled over time.",
		}, []string{cfg.MethodLabel, cfg.HandlerIDLabel}),
	}

	cfg.Registry.MustRegister(
		r.httpRequestDurHistogram,
		r.httpResponseSizeHistogram,
		r.httpRequestsInflight,
		r.httpRequestsTotal,
	)

	return r
}

// These methods are called automatically by go-http-metrics/middleware
func (r *RestCollector) ObserveHTTPRequestDuration(_ context.Context, p httpmetrics.HTTPReqProperties, duration time.Duration) {
	r.httpRequestDurHistogram.WithLabelValues(p.Service, p.ID, p.Method, p.Code).Observe(duration.Seconds())
}

func (r *RestCollector) ObserveHTTPResponseSize(_ context.Context, p httpmetrics.HTTPReqProperties, sizeBytes int64) {
	r.httpResponseSizeHistogram.WithLabelValues(p.Service, p.ID, p.Method, p.Code).Observe(float64(sizeBytes))
}

func (r *RestCollector) AddInflightRequests(_ context.Context, p httpmetrics.HTTPProperties, quantity int) {
	r.httpRequestsInflight.WithLabelValues(p.Service, p.ID).Add(float64(quantity))
}

// New custom method to track all requests made for every REST API request
func (r *RestCollector) AddTotalRequests(_ context.Context, method string, routeName string) {
	r.httpRequestsTotal.WithLabelValues(method, routeName).Inc()
}
