package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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
			Namespace: namespaceLoader,
			Help:      "transactions sent by the loader",
		}),
		transactionsLost: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "transactions_lost",
			Namespace: namespaceLoader,
			Help:      "transaction that took too long to return",
		}),
		tpsConfigured: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "transactions_per_second_configured",
			Namespace: namespaceLoader,
			Help:      "transactions per second that the loader should send",
		}),
		transactionsExecuted: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "transactions_executed",
			Namespace: namespaceLoader,
			Help:      "transaction successfully executed by the loader",
		}),
		tteInSeconds: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:      "transactions_executed_in_seconds",
			Namespace: namespaceLoader,
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
