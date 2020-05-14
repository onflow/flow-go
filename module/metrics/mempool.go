package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type MempoolCollector struct {
	entries *prometheus.GaugeVec
}

func NewMempoolCollector() *MempoolCollector {

	mc := &MempoolCollector{
		entries: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "entries_total",
			Namespace: namespaceStorage,
			Subsystem: subsystemMempool,
			Help:      "the number of entries in the mempool",
		}, []string{LabelResource}),
	}

	return mc
}

func (mc *MempoolCollector) MempoolEntries(resource string, entries uint) {
	mc.entries.With(prometheus.Labels{LabelResource: resource}).Set(float64(entries))
}
