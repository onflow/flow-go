package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/slok/go-http-metrics/metrics"
	metricsProm "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"

	"github.com/gorilla/mux"
)

// Example recorder taken from:
// https://github.com/slok/go-http-metrics/blob/master/metrics/prometheus/prometheus.go
type CustomRecorder interface {
	metrics.Recorder
	AddTotalRequests(ctx context.Context, service string, id string)
}

type recorder struct {
	httpRequestDurHistogram   *prometheus.HistogramVec
	httpResponseSizeHistogram *prometheus.HistogramVec
	httpRequestsInflight      *prometheus.GaugeVec
	httpRequestsTotal         *prometheus.GaugeVec
}

// NewRecorder returns a new metrics recorder that implements the recorder
// using Prometheus as the backend.
func NewRecorder(cfg metricsProm.Config) CustomRecorder {
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

	r := &recorder{
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
		}, []string{cfg.ServiceLabel, cfg.HandlerIDLabel}),
	}

	cfg.Registry.MustRegister(
		r.httpRequestDurHistogram,
		r.httpResponseSizeHistogram,
		r.httpRequestsInflight,
		r.httpRequestsTotal,
	)

	return r
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func MetricsMiddleware() mux.MiddlewareFunc {
	r := NewRecorder(metricsProm.Config{Prefix: "access_rest_api"})
	metricsMiddleware := middleware.New(middleware.Config{Recorder: r})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// This is a custom metric being called on every http request
			r.AddTotalRequests(req.Context(), req.Method, req.URL.Path)

			// Modify the writer
			respWriter := &responseWriter{w, http.StatusOK}

			// Record go-http-metrics/middleware metrics and continue to the next handler
			std.Handler("", metricsMiddleware, next).ServeHTTP(respWriter, req)
		})
	}
}

// These methods are called automatically by go-http-metrics/middleware
func (r recorder) ObserveHTTPRequestDuration(_ context.Context, p metrics.HTTPReqProperties, duration time.Duration) {
	r.httpRequestDurHistogram.WithLabelValues(p.Service, p.ID, p.Method, p.Code).Observe(duration.Seconds())
}

func (r recorder) ObserveHTTPResponseSize(_ context.Context, p metrics.HTTPReqProperties, sizeBytes int64) {
	r.httpResponseSizeHistogram.WithLabelValues(p.Service, p.ID, p.Method, p.Code).Observe(float64(sizeBytes))
}

func (r recorder) AddInflightRequests(_ context.Context, p metrics.HTTPProperties, quantity int) {
	r.httpRequestsInflight.WithLabelValues(p.Service, p.ID).Add(float64(quantity))
}

// New custom method to track all requests made for every REST API request
func (r recorder) AddTotalRequests(_ context.Context, method string, id string) {
	r.httpRequestsTotal.WithLabelValues(method, id).Inc()
}
