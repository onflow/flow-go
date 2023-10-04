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
	// TopicListUpdateInterval is the interval for updating the list of topics in the pubsub network that this node has subscribed to.
	// Note that we recommend setting this value larger than the PeerSubscriptionUpdateTTL; otherwise, the list of topics
	// will be updated as (or even more) frequently than the list of peers subscribed to a topic, which is not necessary.
	// Also, small values for this parameter can lead to a high contention with the reading threads.
	TopicListUpdateInterval time.Duration `validate:"gt=0s" mapstructure:"gossipsub-topic-list-update-interval"`

	// PeerSubscriptionUpdateTTL is the time-to-live for updating the list of topics a peer is subscribed to.
	// Note that we recommend setting this value smaller than the TopicListUpdateTTL; otherwise, the list of topics
	// will be updated as (or even more) frequently than the list of peers subscribed to a topic, which is not necessary.
	PeerSubscriptionUpdateTTL time.Duration `validate:"gt=0s" mapstructure:"gossipsub-peer-subscription-update-ttl"`
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
