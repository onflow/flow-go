package p2pconfig

import (
	"time"

	"github.com/onflow/flow-go/network/netconf"
	"github.com/onflow/flow-go/network/p2p"
)

// UnicastConfig configuration parameters for the unicast protocol.
type UnicastConfig struct {
	netconf.UnicastConfig

	// RateLimiterDistributor distributor that distributes notifications whenever a peer is rate limited to all consumers.
	RateLimiterDistributor p2p.UnicastRateLimiterDistributor
}

// ConnectionGaterConfig configuration parameters for the connection gater.
type ConnectionGaterConfig struct {
	// InterceptPeerDialFilters list of peer filters used to filter peers on outgoing connections in the InterceptPeerDial callback.
	InterceptPeerDialFilters []p2p.PeerFilter
	// InterceptSecuredFilters list of peer filters used to filter peers and accept or reject inbound connections in InterceptSecured callback.
	InterceptSecuredFilters []p2p.PeerFilter
}

// PeerManagerConfig configuration parameters for the peer manager.
type PeerManagerConfig struct {
	// ConnectionPruning enables connection pruning in the connection manager.
	ConnectionPruning bool
	// UpdateInterval interval used by the libp2p node peer manager component to periodically request peer updates.
	UpdateInterval time.Duration
	// ConnectorFactory is a factory function to create a new connector.
	ConnectorFactory p2p.ConnectorFactory
}

// PeerManagerDisableConfig returns a configuration that disables the peer manager.
func PeerManagerDisableConfig() *PeerManagerConfig {
	return &PeerManagerConfig{
		ConnectionPruning: false,
		UpdateInterval:    0,
		ConnectorFactory:  nil,
	}
}
