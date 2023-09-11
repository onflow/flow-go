package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics/internal"
)

type RateLimitedBlockstoreCollector struct {
	bytesRead prometheus.Counter
}

func NewRateLimitedBlockstoreCollector(prefix string) module.RateLimitedBlockstoreMetrics {
	return &RateLimitedBlockstoreCollector{
		bytesRead: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: internal.NamespaceStateSync,
			Subsystem: internal.SubsystemExeDataBlobstore,
			Name:      prefix + "_bytes_read",
			Help:      "number of bytes read from the blockstore",
		}),
	}
}

func (r *RateLimitedBlockstoreCollector) BytesRead(n int) {
	r.bytesRead.Add(float64(n))
}
