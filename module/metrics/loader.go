package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type LoaderCollector struct {
	transactionsSent prometheus.Counter
	tpsConfigured    prometheus.Gauge
}

func NewLoaderCollector() *LoaderCollector {

	cc := &LoaderCollector{
		transactionsSent: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "transactions_sent",
			Namespace: namespaceLoader,
			Help:      "transactions sent by the loader",
		}),
		tpsConfigured: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "transactions_per_second_configured",
			Namespace: namespaceLoader,
			Help:      "transactions per second that the loader should send",
		}),
	}

	return cc
}

func (cc *LoaderCollector) TransactionSent() {
	cc.transactionsSent.Inc()
}

func (cc *LoaderCollector) SetTPSConfigured(tps int) {
	cc.tpsConfigured.Set(float64(tps))
}
