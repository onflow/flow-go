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
	// LocalMeshLogInterval is the interval at which the local mesh is logged.
	LocalMeshLogInterval time.Duration `mapstructure:"gossipsub-local-mesh-logging-interval"`
	// ScoreTracerInterval is the interval at which the score tracer logs the peer scores.
	ScoreTracerInterval time.Duration `mapstructure:"gossipsub-score-tracer-interval"`
	// RPCSentTrackerCacheSize cache size of the rpc sent tracker used by the gossipsub mesh tracer.
	RPCSentTrackerCacheSize uint32 `mapstructure:"gossipsub-rpc-sent-tracker-cache-size"`
	// PeerScoring is whether to enable GossipSub peer scoring.
	PeerScoring bool `mapstructure:"gossipsub-peer-scoring-enabled"`
}
