package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type CleanerCollector struct {
	gcDuration prometheus.Histogram
}

func NewCleanerCollector() *CleanerCollector {
	cc := &CleanerCollector{
		gcDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespaceStorage,
			Subsystem: subsystemBadger,
			Name:      "garbage_collection_runtime_s",
			Buckets:   []float64{1, 10, 60, 60 * 5, 60 * 15},
			Help:      "the time spent on badger garbage collection",
		}),
	}
	return cc
}

// RanGC records a successful run of the Badger garbage collector.
func (cc *CleanerCollector) RanGC(duration time.Duration) {
	cc.gcDuration.Observe(duration.Seconds())
}
