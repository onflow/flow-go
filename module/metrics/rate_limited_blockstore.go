package metrics

import (
	"github.com/onflow/flow-go/module"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type RateLimitedBlockstoreCollector struct {
	bytesRead prometheus.Counter
}

func NewRateLimitedBlockstoreCollector() module.RateLimitedBlockstoreMetrics {
	return &RateLimitedBlockstoreCollector{
		bytesRead: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceStateSync,
			Subsystem: subsystemExecutionDataService,
			Name:      "bytes_read",
			Help:      "number of bytes read from the blockstore",
		}),
	}
}

func (r *RateLimitedBlockstoreCollector) BytesRead(n int) {
	r.bytesRead.Add(float64(n))
}
