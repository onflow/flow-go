package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type AccessCollector struct {
	connectionReused      prometheus.Counter
	connectionsInPool     *prometheus.GaugeVec
	connectionAdded       prometheus.Counter
	connectionEstablished prometheus.Counter
	connectionInvalidated prometheus.Counter
	connectionUpdated     prometheus.Counter
	connectionEvicted     prometheus.Counter
}

func NewAccessCollector() *AccessCollector {
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
