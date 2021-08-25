package unstaked

import (
	"github.com/onflow/flow-go/module/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type NetworkCollector struct {
	metrics.NetworkCollector

	outboundConnectionCount prometheus.Gauge
	inboundConnectionCount  prometheus.Gauge
}

func NewUnstakedNetworkCollector(collector metrics.NetworkCollector) *NetworkCollector {
	return &NetworkCollector{
		collector,
		promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: metrics.NamespaceNetwork,
			Subsystem: metrics.SubsystemQueue,
			Name:      "unstaked_outbound_connection_count",
			Help:      "the number of outbound connections to unstaked nodes",
		}),
		promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: metrics.NamespaceNetwork,
			Subsystem: metrics.SubsystemQueue,
			Name:      "unstaked_inbound_connection_count",
			Help:      "the number of inbound connections from unstaked nodes",
		}),
	}
}

// NetworkMessageSent tracks the message size of the last message sent out on the wire
// in bytes for the given topic
func (nc *NetworkCollector) NetworkMessageSent(sizeBytes int, topic string, messageType string) {
	nc.NetworkCollector.NetworkMessageSent(sizeBytes, "unstaked_"+topic, messageType)
}

// NetworkMessageReceived tracks the message size of the last message received on the wire
// in bytes for the given topic
func (nc *NetworkCollector) NetworkMessageReceived(sizeBytes int, topic string, messageType string) {
	nc.NetworkCollector.NetworkMessageReceived(sizeBytes, "unstaked_"+topic, messageType)
}

// NetworkDuplicateMessagesDropped tracks the number of messages dropped by the network layer due to duplication
func (nc *NetworkCollector) NetworkDuplicateMessagesDropped(topic, messageType string) {
	nc.NetworkCollector.NetworkDuplicateMessagesDropped("unstaked_"+topic, messageType)
}

func (nc *NetworkCollector) OutboundConnections(connectionCount uint) {
	nc.outboundConnectionCount.Set(float64(connectionCount))
}

func (nc *NetworkCollector) InboundConnections(connectionCount uint) {
	nc.inboundConnectionCount.Set(float64(connectionCount))
}
