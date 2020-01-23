package trace

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	spanDurationMetric = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "span_duration_ms",
		Help: "The duration of the jaeger span",
	})
)
