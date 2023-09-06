package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	httpmetrics "github.com/slok/go-http-metrics/metrics"

	"github.com/onflow/flow-go/module"
)

type RestCollector struct {
	httpRequestDurHistogram   *prometheus.HistogramVec
	httpResponseSizeHistogram *prometheus.HistogramVec
	httpRequestsInflight      *prometheus.GaugeVec
	httpRequestsTotal         *prometheus.GaugeVec

	// urlToRouteMapper is a callback that converts a URL to a route name
	urlToRouteMapper func(string) (string, error)
}

var _ module.RestMetrics = (*RestCollector)(nil)

// NewRestCollector returns a new metrics RestCollector that implements the RestCollector
// using Prometheus as the backend.
func NewRestCollector(urlToRouteMapper func(string) (string, error), registerer prometheus.Registerer) (*RestCollector, error) {
	if urlToRouteMapper == nil {
		return nil, fmt.Errorf("urlToRouteMapper cannot be nil")
	}

	r := &RestCollector{
		urlToRouteMapper: urlToRouteMapper,
		httpRequestDurHistogram: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespaceRestAPI,
			Subsystem: subsystemHTTP,
			Name:      "request_duration_seconds",
			Help:      "The latency of the HTTP requests.",
			Buckets:   prometheus.DefBuckets,
		}, []string{LabelService, LabelHandler, LabelMethod, LabelStatusCode}),

		httpResponseSizeHistogram: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespaceRestAPI,
			Subsystem: subsystemHTTP,
			Name:      "response_size_bytes",
			Help:      "The size of the HTTP responses.",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
		}, []string{LabelService, LabelHandler, LabelMethod, LabelStatusCode}),

		httpRequestsInflight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespaceRestAPI,
			Subsystem: subsystemHTTP,
			Name:      "requests_inflight",
			Help:      "The number of inflight requests being handled at the same time.",
		}, []string{LabelService, LabelHandler}),

		httpRequestsTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespaceRestAPI,
			Subsystem: subsystemHTTP,
			Name:      "requests_total",
			Help:      "The number of requests handled over time.",
		}, []string{LabelMethod, LabelHandler}),
	}

	registerer.MustRegister(
		r.httpRequestDurHistogram,
		r.httpResponseSizeHistogram,
		r.httpRequestsInflight,
		r.httpRequestsTotal,
	)

	return r, nil
}

// ObserveHTTPRequestDuration records the duration of the REST request.
// This method is called automatically by go-http-metrics/middleware
func (r *RestCollector) ObserveHTTPRequestDuration(_ context.Context, p httpmetrics.HTTPReqProperties, duration time.Duration) {
	handler := r.mapURLToRoute(p.ID)
	r.httpRequestDurHistogram.WithLabelValues(p.Service, handler, p.Method, p.Code).Observe(duration.Seconds())
}

// ObserveHTTPResponseSize records the response size of the REST request.
// This method is called automatically by go-http-metrics/middleware
func (r *RestCollector) ObserveHTTPResponseSize(_ context.Context, p httpmetrics.HTTPReqProperties, sizeBytes int64) {
	handler := r.mapURLToRoute(p.ID)
	r.httpResponseSizeHistogram.WithLabelValues(p.Service, handler, p.Method, p.Code).Observe(float64(sizeBytes))
}

// AddInflightRequests increments and decrements the number of inflight request being processed.
// This method is called automatically by go-http-metrics/middleware
func (r *RestCollector) AddInflightRequests(_ context.Context, p httpmetrics.HTTPProperties, quantity int) {
	handler := r.mapURLToRoute(p.ID)
	r.httpRequestsInflight.WithLabelValues(p.Service, handler).Add(float64(quantity))
}

// AddTotalRequests records all REST requests
// This is a custom method called by the REST handler
func (r *RestCollector) AddTotalRequests(_ context.Context, method, path string) {
	handler := r.mapURLToRoute(path)
	r.httpRequestsTotal.WithLabelValues(method, handler).Inc()
}

// mapURLToRoute uses the urlToRouteMapper callback to convert a URL to a route name
// This normalizes the URL, removing dynamic information converting it to a static string
func (r *RestCollector) mapURLToRoute(url string) string {
	route, err := r.urlToRouteMapper(url)
	if err != nil {
		return "unknown"
	}

	return route
}
