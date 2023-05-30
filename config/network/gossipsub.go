package network

import (
	"time"
)

// ResourceManagerConfig returns the resource manager configuration for the libp2p node.
// The resource manager is used to limit the number of open connections and streams (as well as any other resources
// used by libp2p) for each peer.
type ResourceManagerConfig struct {
	MemoryLimitRatio          float64 `mapstructure:"memory-limit-ratio"`             // maximum allowed fraction of memory to be allocated by the libp2p resources in (0,1]
	FileDescriptorsRatio      float64 `mapstructure:"file-descriptors-ratio"`         // maximum allowed fraction of file descriptors to be allocated by the libp2p resources in (0,1]
	PeerBaseLimitConnsInbound int     `mapstructure:"peer-base-limits-conns-inbound"` // the maximum amount of allowed inbound connections per peer
}

// GossipSubConfig is the configuration for the GossipSub pubsub implementation.
type GossipSubConfig struct {
	// LocalMeshLogInterval is the interval at which the local mesh is logged.
	LocalMeshLogInterval time.Duration `mapstructure:"local-mesh-logging-interval"`
	// ScoreTracerInterval is the interval at which the score tracer logs the peer scores.
	ScoreTracerInterval time.Duration `mapstructure:"score-tracer-interval"`
	// PeerScoring is whether to enable GossipSub peer scoring.
	PeerScoring bool `mapstructure:"peer-scoring-enabled"`
	// RpcInspector configuration for all gossipsub RPC control message inspectors.
	RpcInspector *GossipSubRPCInspectorsConfig `mapstructure:"rpc-inspectors"`
}
