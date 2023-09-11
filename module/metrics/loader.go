package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module/metrics/internal"
)

type LoaderCollector struct {
	transactionsSent prometheus.Counter
	transactionsLost prometheus.Counter
	tpsConfigured    prometheus.Gauge

	transactionsExecuted prometheus.Counter
	tteInSeconds         prometheus.Histogram
}

func NewLoaderCollector() *LoaderCollector {

	cc := &LoaderCollector{
		transactionsSent: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "transactions_sent",
			Namespace: internal.NamespaceLoader,
			Help:      "transactions sent by the loader",
		}),
		transactionsLost: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "transactions_lost",
			Namespace: internal.NamespaceLoader,
			Help:      "transaction that took too long to return",
		}),
		tpsConfigured: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "transactions_per_second_configured",
			Namespace: internal.NamespaceLoader,
			Help:      "transactions per second that the loader should send",
		}),
		transactionsExecuted: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "transactions_executed",
			Namespace: internal.NamespaceLoader,
			Help:      "transaction successfully executed by the loader",
		}),
		tteInSeconds: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:      "transactions_executed_in_seconds",
			Namespace: internal.NamespaceLoader,
			Help:      "Time To Execute histogram for transactions (in seconds)",
			Buckets:   prometheus.ExponentialBuckets(2, 2, 8),
		}),
	}

	return cc
}

func (cc *LoaderCollector) TransactionSent() {
	cc.transactionsSent.Inc()
}

func (cc *LoaderCollector) TransactionLost() {
	cc.transactionsLost.Inc()
}

func (cc *LoaderCollector) SetTPSConfigured(tps uint) {
	cc.tpsConfigured.Set(float64(tps))
}

func (cc *LoaderCollector) TransactionExecuted(duration time.Duration) {
	cc.transactionsExecuted.Inc()
	cc.tteInSeconds.Observe(float64(duration.Seconds()))
}
