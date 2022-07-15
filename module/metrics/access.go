package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type AccessCollector struct {
	connectionReused  prometheus.Counter
	connectionsInPool *prometheus.GaugeVec
	connectionAdded   prometheus.Counter
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
			Help:      "counter for the number of times connections are added",
		}),
	}

	return ac
}

func (ac *AccessCollector) ConnectionFromPoolRetrieved() {
	ac.connectionReused.Inc()
}

func (ac *AccessCollector) TotalConnectionsInPool(connectionCount uint, connectionPoolSize uint) {
	ac.connectionsInPool.WithLabelValues("connections").Set(float64(connectionCount))
	ac.connectionsInPool.WithLabelValues("pool_size").Set(float64(connectionPoolSize))
}

func (ac *AccessCollector) ConnectionAddedToPool() {
	ac.connectionAdded.Inc()
}
