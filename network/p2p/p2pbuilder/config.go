package p2pbuilder

import (
	"time"

	"github.com/onflow/flow-go/network/p2p"
)

// UnicastConfig configuration parameters for the unicast manager.
type UnicastConfig struct {
	StreamRetryInterval    time.Duration // retry interval for attempts on creating a stream to a remote peer.
	RateLimiterDistributor p2p.UnicastRateLimiterDistributor
}

// ConnectionGaterConfig configuration parameters for the connection gater.
type ConnectionGaterConfig struct {
	InterceptPeerDialFilters []p2p.PeerFilter
	InterceptSecuredFilters  []p2p.PeerFilter
}

// PeerManagerConfig configuration parameters for the peer manager.
type PeerManagerConfig struct {
	ConnectionPruning bool
	UpdateInterval    time.Duration
}
