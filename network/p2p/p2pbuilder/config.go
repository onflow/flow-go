package p2pbuilder

import (
	"time"

	"github.com/onflow/flow-go/network/p2p"
)

// UnicastConfig configuration parameters for the unicast manager.
type UnicastConfig struct {
	// StreamRetryInterval is the initial delay between failing to establish a connection with another node and retrying. This
	// delay increases exponentially (exponential backoff) with the number of subsequent failures to establish a connection.
	StreamRetryInterval    time.Duration
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
