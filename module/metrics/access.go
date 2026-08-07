package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
)

type AccessCollectorOpts func(*AccessCollector)

func WithTransactionValidationMetrics(m module.TransactionValidationMetrics) AccessCollectorOpts {
	return func(ac *AccessCollector) {
		ac.TransactionValidationMetrics = m
	}
}

func WithTransactionMetrics(m module.TransactionMetrics) AccessCollectorOpts {
	return func(ac *AccessCollector) {
		ac.TransactionMetrics = m
	}
}

func WithBackendScriptsMetrics(m module.BackendScriptsMetrics) AccessCollectorOpts {
	return func(ac *AccessCollector) {
		ac.BackendScriptsMetrics = m
	}
}

func WithRestMetrics(m module.RestMetrics) AccessCollectorOpts {
	return func(ac *AccessCollector) {
		ac.RestMetrics = m
	}
}

type AccessCollector struct {
	module.RestMetrics
	module.TransactionMetrics
	module.TransactionValidationMetrics
	module.BackendScriptsMetrics

	connectionReused              prometheus.Counter
	connectionsInPool             *prometheus.GaugeVec
	connectionAdded               prometheus.Counter
	connectionEstablished         prometheus.Counter
	connectionInvalidated         prometheus.Counter
	connectionUpdated             prometheus.Counter
	connectionEvicted             prometheus.Counter
	lastFullBlockHeight           prometheus.Gauge
	ingestionFinalizedBlockHeight prometheus.Gauge
	maxReceiptHeight              prometheus.Gauge

	// used to skip heights that are lower than the current max height
	maxReceiptHeightValue counters.StrictMonotonicCounter
}

var _ module.AccessMetrics = (*AccessCollector)(nil)

func NewAccessCollector(opts ...AccessCollectorOpts) *AccessCollector {
	ac := &AccessCollector{
		connectionReused: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "connection_reused",
			Namespace: namespaceAccess,
			Subsystem: subsystemConnectionPool,
			Help:      "counter for the number of times connections get reused",
		}),
		connectionsInPool: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "connections_in_pool",
			Namespace: namespaceAccess,
			Subsystem: subsystemConnectionPool,
			Help:      "counter for the number of connections in the pool against max number tne pool can hold",
		}, []string{"result"}),
		connectionAdded: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "connection_added",
			Namespace: namespaceAccess,
			Subsystem: subsystemConnectionPool,
			Help:      "counter for the number of times connections are added to the pool",
		}),
		connectionEstablished: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "connection_established",
			Namespace: namespaceAccess,
			Subsystem: subsystemConnectionPool,
			Help:      "counter for the number of times connections are established",
		}),
		connectionInvalidated: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "connection_invalidated",
			Namespace: namespaceAccess,
			Subsystem: subsystemConnectionPool,
			Help:      "counter for the number of times connections are invalidated",
		}),
		connectionUpdated: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "connection_updated",
			Namespace: namespaceAccess,
			Subsystem: subsystemConnectionPool,
			Help:      "counter for the number of times existing connections from the pool are updated",
		}),
		connectionEvicted: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "connection_evicted",
			Namespace: namespaceAccess,
			Subsystem: subsystemConnectionPool,
			Help:      "counter for the number of times a cached connection is evicted from the connection pool",
		}),
		lastFullBlockHeight: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "last_full_finalized_block_height",
			Namespace: namespaceAccess,
			Subsystem: subsystemIngestion,
			Help:      "gauge to track the highest consecutive finalized block height with all collections indexed",
		}),
		ingestionFinalizedBlockHeight: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "ingestion_finalized_block_height",
			Namespace: namespaceAccess,
			Subsystem: subsystemIngestion,
			Help:      "gauge to track the latest finalized block height processed by ingestion",
		}),
		maxReceiptHeight: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "max_receipt_height",
			Namespace: namespaceAccess,
			Subsystem: subsystemIngestion,
			Help:      "gauge to track the maximum block height of execution receipts received",
		}),
		maxReceiptHeightValue: counters.NewMonotonicCounter(0),
	}

	for _, opt := range opts {
		opt(ac)
	}

	return ac
}

func (ac *AccessCollector) ConnectionFromPoolReused() {
	ac.connectionReused.Inc()
}

func (ac *AccessCollector) TotalConnectionsInPool(connectionCount uint, connectionPoolSize uint) {
	ac.connectionsInPool.WithLabelValues("connections").Set(float64(connectionCount))
	ac.connectionsInPool.WithLabelValues("pool_size").Set(float64(connectionPoolSize))
}

func (ac *AccessCollector) ConnectionAddedToPool() {
	ac.connectionAdded.Inc()
}

func (ac *AccessCollector) NewConnectionEstablished() {
	ac.connectionEstablished.Inc()
}

func (ac *AccessCollector) ConnectionFromPoolInvalidated() {
	ac.connectionInvalidated.Inc()
}

func (ac *AccessCollector) ConnectionFromPoolUpdated() {
	ac.connectionUpdated.Inc()
}

func (ac *AccessCollector) ConnectionFromPoolEvicted() {
	ac.connectionEvicted.Inc()
}

func (ac *AccessCollector) UpdateLastFullBlockHeight(height uint64) {
	ac.lastFullBlockHeight.Set(float64(height))
}

func (ac *AccessCollector) UpdateIngestionFinalizedBlockHeight(height uint64) {
	ac.ingestionFinalizedBlockHeight.Set(float64(height))
}

func (ac *AccessCollector) UpdateExecutionReceiptMaxHeight(height uint64) {
	if ac.maxReceiptHeightValue.Set(height) {
		ac.maxReceiptHeight.Set(float64(height))
	}
}
