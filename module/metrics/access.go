package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type AccessCollector struct {
	connectionReused      prometheus.Counter
	connectionAddedToPool *prometheus.GaugeVec
}

func NewAccessCollector() *AccessCollector {
	ac := &AccessCollector{
		connectionReused: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "connection_reused",
			Namespace: namespaceAccess,
			Subsystem: subsystemConnectionPool,
			Help:      "counter for the number of times connections get reused",
		}),
		connectionAddedToPool: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "connection_added",
			Namespace: namespaceAccess,
			Subsystem: subsystemConnectionPool,
			Help:      "counter for the number of connections in the pool against max number tne pool can hold",
		}, []string{"result"}),
	}

	return ac
}

func (ac *AccessCollector) ConnectionFromPoolRetrieved() {
	ac.connectionReused.Inc()
}

func (ac *AccessCollector) TotalConnectionsInPool(connectionCount uint, connectionPoolSize uint) {
	ac.connectionAddedToPool.WithLabelValues("connections").Set(float64(connectionCount))
	ac.connectionAddedToPool.WithLabelValues("pool_size").Set(float64(connectionPoolSize))
}
