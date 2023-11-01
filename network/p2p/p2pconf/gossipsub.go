package p2pconf

import (
	"time"
)

// ResourceManagerConfig returns the resource manager configuration for the libp2p node.
// The resource manager is used to limit the number of open connections and streams (as well as any other resources
// used by libp2p) for each peer.
type ResourceManagerConfig struct {
	Streams                   StreamsLimit                 `mapstructure:"libp2p-streams-limit"`                  // the limit for streams
	InboundConn               ResourceManagerOverrideLimit `mapstructure:"libp2p-inbound-conns"`                  // the limit for inbound connections
	OutboundConn              ResourceManagerOverrideLimit `mapstructure:"libp2p-outbound-conns"`                 // the limit for outbound connections
	MemoryLimitRatio          float64                      `mapstructure:"libp2p-memory-limit-ratio"`             // maximum allowed fraction of memory to be allocated by the libp2p resources in (0,1]
	FileDescriptorsRatio      float64                      `mapstructure:"libp2p-file-descriptors-ratio"`         // maximum allowed fraction of file descriptors to be allocated by the libp2p resources in (0,1]
	PeerBaseLimitConnsInbound int                          `mapstructure:"libp2p-peer-base-limits-conns-inbound"` // the maximum amount of allowed inbound connections per peer
}

// StreamsLimit is the resource manager limit for streams.
type StreamsLimit struct {
	// Inbound is the limit for inbound streams.
	Inbound ResourceManagerOverrideLimit `mapstructure:"inbound"`
	// Outbound is the limit for outbound streams.
	Outbound ResourceManagerOverrideLimit `mapstructure:"outbound"`
}

// ResourceManagerOverrideLimit is the configuration for the resource manager override limits.
// Any value that is not set will be ignored and the default value will be used.
type ResourceManagerOverrideLimit struct {
	// System is the limit for the resource at the entire system. if not set, the default value will be used.
	// For a specific limit, the system-wide dictates the maximum allowed value across all peers and protocols at the entire node level.
	System int `validate:"gte=0" mapstructure:"system"`

	// Transient is the limit for resources that have not fully taken yet; if not set, the default value will be used.
	// For a specific limit, the transient limit dictates the maximum allowed value that are in transient state and not yet fully established
	// at the entire node level.
	Transient int `validate:"gte=0" mapstructure:"transient"`

	// Protocol is the limit at the protocol level; if not set, the default value will be used.
	// For a specific limit, the protocol limit dictates the maximum allowed value across the entire node at the protocol level.
	Protocol int `validate:"gte=0" mapstructure:"protocol"`

	// Peer is the limit at the peer level; if not set, the default value will be used.
	// For a specific limit, the peer limit dictates the maximum allowed value across the entire node for that peer.
	Peer int `validate:"gte=0" mapstructure:"peer"`

	// ProtocolPeer is the limit at the protocol and peer level; if not set, the default value will be used.
	// For a specific limit, the protocol peer limit dictates the maximum allowed value across the entire node for that peer on a specific protocol.
	ProtocolPeer int `validate:"gte=0" mapstructure:"peer-protocol"`
}

// GossipSubConfig is the configuration for the GossipSub pubsub implementation.
type GossipSubConfig struct {
	// GossipSubRPCInspectorsConfig configuration for all gossipsub RPC control message inspectors.
	GossipSubRPCInspectorsConfig `mapstructure:",squash"`

	// GossipSubTracerConfig is the configuration for the gossipsub tracer. GossipSub tracer is used to trace the local mesh events and peer scores.
	GossipSubTracerConfig `mapstructure:",squash"`

	// PeerScoring is whether to enable GossipSub peer scoring.
	PeerScoring bool `mapstructure:"gossipsub-peer-scoring-enabled"`
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
