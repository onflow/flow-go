package trace

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	spanDurationMetric = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "span_duration_s",
		Help: "The duration of the jaeger span in seconds",
	}, []string{"name"})
)
