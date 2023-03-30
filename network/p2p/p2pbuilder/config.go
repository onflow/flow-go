package p2pbuilder

import (
	"time"

	"github.com/onflow/flow-go/network/p2p"
)

// UnicastConfig configuration parameters for the unicast manager.
type UnicastConfig struct {
	// StreamRetryInterval is the initial delay between failing to establish a connection with another node and retrying. This
	// delay increases exponentially (exponential backoff) with the number of subsequent failures to establish a connection.
	StreamRetryInterval time.Duration
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
}

// GossipSubRPCValidationInspectorConfigs validation limits used for gossipsub RPC control message inspection.
type GossipSubRPCValidationInspectorConfigs struct {
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int
	// CacheSize size of the queue used by worker pool for the control message validation inspector.
	CacheSize uint32
	// GraftLimits GRAFT control message validation limits.
	GraftLimits map[string]int
	// PruneLimits PRUNE control message validation limits.
	PruneLimits map[string]int
}

// GossipSubRPCMetricsInspectorConfigs rpc metrics observer inspector configuration.
type GossipSubRPCMetricsInspectorConfigs struct {
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int
	// CacheSize size of the queue used by worker pool for the control message metrics inspector.
	CacheSize uint32
}
