package p2pconf

import (
	"time"

	"github.com/onflow/flow-go/network/p2p/scoring"
)

// ResourceManagerConfig returns the resource manager configuration for the libp2p node.
// The resource manager is used to limit the number of open connections and streams (as well as any other resources
// used by libp2p) for each peer.
type ResourceManagerConfig struct {
	Override             ResourceManagerOverrideScope `mapstructure:"limits-override"`        // override limits for specific peers, protocols, etc.
	MemoryLimitRatio     float64                      `mapstructure:"memory-limit-ratio"`     // maximum allowed fraction of memory to be allocated by the libp2p resources in (0,1]
	FileDescriptorsRatio float64                      `mapstructure:"file-descriptors-ratio"` // maximum allowed fraction of file descriptors to be allocated by the libp2p resources in (0,1]
}

type ResourceManagerOverrideScope struct {
	// System is the limit for the resource at the entire system.
	// For a specific limit, the system-wide dictates the maximum allowed value across all peers and protocols at the entire node level.
	System ResourceManagerOverrideLimit `mapstructure:"system"`

	// Transient is the limit for the resource at the transient scope. Transient limits are used for resources that have not fully established and are under negotiation.
	Transient ResourceManagerOverrideLimit `mapstructure:"transient"`

	// Protocol is the limit for the resource at the protocol scope, e.g., DHT, GossipSub, etc. It dictates the maximum allowed resource across all peers for that protocol.
	Protocol ResourceManagerOverrideLimit `mapstructure:"protocol"`

	// Peer is the limit for the resource at the peer scope. It dictates the maximum allowed resource for a specific peer.
	Peer ResourceManagerOverrideLimit `mapstructure:"peer"`

	// Connection is the limit for the resource for a pair of (peer, protocol), e.g., (peer1, DHT), (peer1, GossipSub), etc. It dictates the maximum allowed resource for a protocol and a peer.
	PeerProtocol ResourceManagerOverrideLimit `mapstructure:"peer-protocol"`
}

// ResourceManagerOverrideLimit is the configuration for the resource manager override limit at a certain scope.
// Any value that is not set will be ignored and the default value will be used.
type ResourceManagerOverrideLimit struct {
	// System is the limit for the resource at the entire system. if not set, the default value will be used.
	// For a specific limit, the system-wide dictates the maximum allowed value across all peers and protocols at the entire node scope.
	StreamsInbound int `validate:"gte=0" mapstructure:"streams-inbound"`

	// StreamsOutbound is the max number of outbound streams allowed, at the resource scope.
	StreamsOutbound int `validate:"gte=0" mapstructure:"streams-outbound"`

	// ConnectionsInbound is the max number of inbound connections allowed, at the resource scope.
	ConnectionsInbound int `validate:"gte=0" mapstructure:"connections-inbound"`

	// ConnectionsOutbound is the max number of outbound connections allowed, at the resource scope.
	ConnectionsOutbound int `validate:"gte=0" mapstructure:"connections-outbound"`

	// FD is the max number of file descriptors allowed, at the resource scope.
	FD int `validate:"gte=0" mapstructure:"fd"`

	// Memory is the max amount of memory allowed (bytes), at the resource scope.
	Memory int `validate:"gte=0" mapstructure:"memory-bytes"`
}

// GossipSubConfig keys.
const (
	RpcInspectorKey         = "rpc-inspector"
	RpcTracerKey            = "rpc-tracer"
	PeerScoringKey          = "peer-scoring-enabled"
	ScoreParamsKey          = "scoring-parameters"
	SubscriptionProviderKey = "subscription-provider"
)

var GossipSubConfigKeys = []string{RpcInspectorKey, RpcTracerKey, PeerScoringKey, ScoreParamsKey, SubscriptionProviderKey}

// GossipSubConfig is the configuration for the GossipSub pubsub implementation.
type GossipSubConfig struct {
	// RpcInspectorParameters configuration for all gossipsub RPC control message inspectors.
	RpcInspector RpcInspectorParameters `mapstructure:"rpc-inspector"`

	// GossipSubTracerParameters is the configuration for the gossipsub tracer. GossipSub tracer is used to trace the local mesh events and peer scores.
	RpcTracer GossipSubTracerParameters `mapstructure:"rpc-tracer"`

	// ScoringParameters is whether to enable GossipSub peer scoring.
	PeerScoringSwitch bool `mapstructure:"peer-scoring-enabled"`

	SubscriptionProvider SubscriptionProviderParameters `mapstructure:"subscription-provider"`

	ScoringParameters scoring.Parameters `mapstructure:"scoring-parameters"`
}

// SubscriptionProviderParameters keys.
const (
	UpdateIntervalKey = "update-interval"
	CacheSizeKey      = "cache-size"
)

var SubscriptionProviderParametersKeys = []string{UpdateIntervalKey, CacheSizeKey}

type SubscriptionProviderParameters struct {
	// UpdateInterval is the interval for updating the list of topics the node have subscribed to; as well as the list of all
	// peers subscribed to each of those topics. This is used to penalize peers that have an invalid subscription based on their role.
	UpdateInterval time.Duration `validate:"gt=0s" mapstructure:"update-interval"`

	// CacheSize is the size of the cache that keeps the list of peers subscribed to each topic as the local node.
	// This is the local view of the current node towards the subscription status of other nodes in the system.
	// The cache must be large enough to accommodate the maximum number of nodes in the system, otherwise the view of the local node will be incomplete
	// due to cache eviction.
	CacheSize uint32 `validate:"gt=0" mapstructure:"cache-size"`
}

// GossipSubTracerParameters keys.
const (
	LocalMeshLogIntervalKey         = "local-mesh-logging-interval"
	ScoreTracerIntervalKey          = "score-tracer-interval"
	RPCSentTrackerCacheSizeKey      = "rpc-sent-tracker-cache-size"
	RPCSentTrackerQueueCacheSizeKey = "rpc-sent-tracker-queue-cache-size"
	RPCSentTrackerNumOfWorkersKey   = "rpc-sent-tracker-workers"
)

var TracerParametersKeys = []string{LocalMeshLogIntervalKey,
	ScoreTracerIntervalKey,
	RPCSentTrackerCacheSizeKey,
	RPCSentTrackerQueueCacheSizeKey,
	RPCSentTrackerNumOfWorkersKey}

// GossipSubTracerParameters is the config for the gossipsub tracer. GossipSub tracer is used to trace the local mesh events and peer scores.
type GossipSubTracerParameters struct {
	// LocalMeshLogInterval is the interval at which the local mesh is logged.
	LocalMeshLogInterval time.Duration `validate:"gt=0s" mapstructure:"local-mesh-logging-interval"`
	// ScoreTracerInterval is the interval at which the score tracer logs the peer scores.
	ScoreTracerInterval time.Duration `validate:"gt=0s" mapstructure:"score-tracer-interval"`
	// RPCSentTrackerCacheSize cache size of the rpc sent tracker used by the gossipsub mesh tracer.
	RPCSentTrackerCacheSize uint32 `validate:"gt=0" mapstructure:"rpc-sent-tracker-cache-size"`
	// RPCSentTrackerQueueCacheSize cache size of the rpc sent tracker queue used for async tracking.
	RPCSentTrackerQueueCacheSize uint32 `validate:"gt=0" mapstructure:"rpc-sent-tracker-queue-cache-size"`
	// RpcSentTrackerNumOfWorkers number of workers for rpc sent tracker worker pool.
	RpcSentTrackerNumOfWorkers int `validate:"gt=0" mapstructure:"rpc-sent-tracker-workers"`
}

// ResourceScope is the scope of the resource, e.g., system, transient, protocol, peer, peer-protocol.
type ResourceScope string

func (r ResourceScope) String() string {
	return string(r)
}

const (
	// ResourceScopeSystem is the system scope; the system scope dictates the maximum allowed value across all peers and protocols at the entire node level.
	ResourceScopeSystem ResourceScope = "system"
	// ResourceScopeTransient is the transient scope; the transient scope is used for resources that have not fully established and are under negotiation.
	ResourceScopeTransient ResourceScope = "transient"
	// ResourceScopeProtocol is the protocol scope; the protocol scope dictates the maximum allowed resource across all peers for that protocol.
	ResourceScopeProtocol ResourceScope = "protocol"
	// ResourceScopePeer is the peer scope; the peer scope dictates the maximum allowed resource for a specific peer.
	ResourceScopePeer ResourceScope = "peer"
	// ResourceScopePeerProtocol is the peer-protocol scope; the peer-protocol scope dictates the maximum allowed resource for a protocol and a peer.
	ResourceScopePeerProtocol ResourceScope = "peer-protocol"
)
