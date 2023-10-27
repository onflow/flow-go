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

	// GossipSubScoringRegistryConfig is the configuration for the GossipSub score registry.
	GossipSubScoringRegistryConfig `mapstructure:",squash"`

	// PeerScoring is whether to enable GossipSub peer scoring.
	PeerScoring bool `mapstructure:"gossipsub-peer-scoring-enabled"`
}

// GossipSubScoringRegistryConfig is the configuration for the GossipSub score registry.
type GossipSubScoringRegistryConfig struct {
	// InitDecayLowerBound is the lower bound on the decay value for a spam record when initialized.
	// A random value in a range of InitDecayLowerBound and InitDecayUpperBound is used when initializing the decay
	// of a spam record.
	InitDecayLowerBound float64 `validate:"gt=0" mapstructure:"gossipsub-scoring-registry-init-decay-lowerbound"`
	// InitDecayUpperBound is the upper bound on the decay value for a spam record when initialized.
	InitDecayUpperBound float64 `validate:"lt=1" mapstructure:"gossipsub-scoring-registry-init-decay-upperbound"`
	// IncreaseDecayThreshold is the threshold for which when the negative penalty is below this value the decay threshold will be increased by some amount. This will
	// lead to malicious nodes having longer decays while honest nodes will have faster decays.
	IncreaseDecayThreshold float64 `validate:"gt=0" mapstructure:"gossipsub-scoring-registry-increase-decay-threshold"`
	// DecayThresholdIncrementer is the amount the decay will be increased when the negative penalty score falls below the IncreaseDecayThreshold.
	DecayThresholdIncrementer float64 `validate:"gt=-100,lt=0" mapstructure:"gossipsub-scoring-registry-decay-threshold-incrementer"`
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
