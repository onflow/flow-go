package connection_manager

import (
	libp2pnet "github.com/libp2p/go-libp2p-core/network"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

var _ network.Observer = &ConnectionMetrics{}

type ConnectionMetrics struct {
	*libp2pnet.NoopNotifiee
	metrics module.NetworkMetrics // metrics to report connection statistics
}

// called by libp2p when a connection opened
func (c *ConnectionMetrics) Connected(n libp2pnet.Network, _ libp2pnet.Conn) {
	c.updateConnectionMetric(n)
}

// called by libp2p when a connection closed
func (c *ConnectionMetrics) Disconnected(n libp2pnet.Network, _ libp2pnet.Conn) {
	c.updateConnectionMetric(n)
}
func (c *ConnectionMetrics) updateConnectionMetric(n libp2pnet.Network) {
	var inbound uint = 0
	var outbound uint = 0
	for _, conn := range n.Conns() {
		switch conn.Stat().Direction {
		case libp2pnet.DirInbound:
			inbound++
		case libp2pnet.DirOutbound:
			outbound++
		}
	}
	c.metrics.InboundConnections(inbound)
	c.metrics.OutboundConnections(outbound)
}
