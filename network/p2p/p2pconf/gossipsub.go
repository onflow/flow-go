package p2pconf

import (
	"time"
)

// ResourceManagerConfig returns the resource manager configuration for the libp2p node.
// The resource manager is used to limit the number of open connections and streams (as well as any other resources
// used by libp2p) for each peer.
type ResourceManagerConfig struct {
	MemoryLimitRatio          float64 `mapstructure:"libp2p-memory-limit-ratio"`             // maximum allowed fraction of memory to be allocated by the libp2p resources in (0,1]
	FileDescriptorsRatio      float64 `mapstructure:"libp2p-file-descriptors-ratio"`         // maximum allowed fraction of file descriptors to be allocated by the libp2p resources in (0,1]
	PeerBaseLimitConnsInbound int     `mapstructure:"libp2p-peer-base-limits-conns-inbound"` // the maximum amount of allowed inbound connections per peer
}

// GossipSubConfig is the configuration for the GossipSub pubsub implementation.
type GossipSubConfig struct {
	// GossipSubRPCInspectorsConfig configuration for all gossipsub RPC control message inspectors.
	GossipSubRPCInspectorsConfig `mapstructure:",squash"`

	// GossipSubTracerConfig is the configuration for the gossipsub tracer. GossipSub tracer is used to trace the local mesh events and peer scores.
	GossipSubTracerConfig `mapstructure:",squash"`

	// PeerScoring is whether to enable GossipSub peer scoring.
	PeerScoring bool `mapstructure:"gossipsub-peer-scoring-enabled"`

	SubscriptionProviderConfig SubscriptionProviderParameters
}

type SubscriptionProviderParameters struct {
	// SubscriptionUpdateInterval is the interval for updating the list of topics the node have subscribed to; as well as the list of all
	// peers subscribed to each of those topics. This is used to penalize peers that have an invalid subscription based on their role.
	SubscriptionUpdateInterval time.Duration `validate:"gt=0s" mapstructure:"gossipsub-subscription-provider-update-interval"`

	// CacheSize is the size of the cache that keeps the list of peers subscribed to each topic as the local node.
	// This is the local view of the current node towards the subscription status of other nodes in the system.
	// The cache must be large enough to accommodate the maximum number of nodes in the system, otherwise the view of the local node will be incomplete
	// due to cache eviction.
	CacheSize uint32 `validate:"gt=0" mapstructure:"gossipsub-subscription-provider-cache-size"`
}

// GossipSubTracerConfig is the config for the gossipsub tracer. GossipSub tracer is used to trace the local mesh events and peer scores.
type GossipSubTracerConfig struct {
	// LocalMeshLogInterval is the interval at which the local mesh is logged.
	LocalMeshLogInterval time.Duration `validate:"gt=0s" mapstructure:"gossipsub-local-mesh-logging-interval"`
	// ScoreTracerInterval is the interval at which the score tracer logs the peer scores.
	ScoreTracerInterval time.Duration `validate:"gt=0s" mapstructure:"gossipsub-score-tracer-interval"`
	// RPCSentTrackerCacheSize cache size of the rpc sent tracker used by the gossipsub mesh tracer.
	RPCSentTrackerCacheSize uint32 `validate:"gt=0" mapstructure:"gossipsub-rpc-sent-tracker-cache-size"`
	// RPCSentTrackerQueueCacheSize cache size of the rpc sent tracker queue used for async tracking.
	RPCSentTrackerQueueCacheSize uint32 `validate:"gt=0" mapstructure:"gossipsub-rpc-sent-tracker-queue-cache-size"`
	// RpcSentTrackerNumOfWorkers number of workers for rpc sent tracker worker pool.
	RpcSentTrackerNumOfWorkers int `validate:"gt=0" mapstructure:"gossipsub-rpc-sent-tracker-workers"`
}
