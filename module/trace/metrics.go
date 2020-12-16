package trace

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	spanDurationMetric = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "span_duration_s",
		Help:    "The duration of the Jaeger span in seconds",
		Buckets: []float64{.001, .01, .1, .5, 5, 10},
	}, []string{"name"})
)
