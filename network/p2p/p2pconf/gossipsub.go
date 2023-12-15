package p2pconf

import (
	"time"
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

// GossipSubParameters keys.
const (
	RpcInspectorKey         = "rpc-inspector"
	RpcTracerKey            = "rpc-tracer"
	PeerScoringEnabledKey   = "peer-scoring-enabled"
	ScoreParamsKey          = "scoring-parameters"
	SubscriptionProviderKey = "subscription-provider"
)

// GossipSubParameters is the configuration for the GossipSub pubsub implementation.
type GossipSubParameters struct {
	// RpcInspectorParameters configuration for all gossipsub RPC control message inspectors.
	RpcInspector RpcInspectorParameters `mapstructure:"rpc-inspector"`

	// GossipSubScoringRegistryConfig is the configuration for the GossipSub score registry.
	// GossipSubTracerParameters is the configuration for the gossipsub tracer. GossipSub tracer is used to trace the local mesh events and peer scores.
	RpcTracer GossipSubTracerParameters `mapstructure:"rpc-tracer"`

	// ScoringParameters is whether to enable GossipSub peer scoring.
	PeerScoringEnabled   bool                           `mapstructure:"peer-scoring-enabled"`
	SubscriptionProvider SubscriptionProviderParameters `mapstructure:"subscription-provider"`
	ScoringParameters    ScoringParameters              `mapstructure:"scoring-parameters"`
}

const (
	AppSpecificScoreRegistryKey = "app-specific-score"
	SpamRecordCacheKey          = "spam-record-cache"
	DecayIntervalKey            = "decay-interval"
)

// ScoringParameters are the parameters for the score option.
// Parameters are "numerical values" that are used to compute or build components that compute the score of a peer in GossipSub system.
type ScoringParameters struct {
	AppSpecificScore AppSpecificScoreParameters `validate:"required" mapstructure:"app-specific-score"`
	SpamRecordCache  SpamRecordCacheParameters  `validate:"required" mapstructure:"spam-record-cache"`
	// DecayInterval is the interval at which the counters associated with a peer behavior in GossipSub system are decayed.
	DecayInterval time.Duration `validate:"gt=0s" mapstructure:"decay-interval"`
}

const (
	ScoreUpdateWorkerNumKey        = "score-update-worker-num"
	ScoreUpdateRequestQueueSizeKey = "score-update-request-queue-size"
	ScoreTTLKey                    = "score-ttl"
)

// AppSpecificScoreParameters is the parameters for the GossipSubAppSpecificScoreRegistry.
// Parameters are "numerical values" that are used to compute or build components that compute or maintain the application specific score of peers.
type AppSpecificScoreParameters struct {
	// ScoreUpdateWorkerNum is the number of workers in the worker pool for handling the application specific score update of peers in a non-blocking way.
	ScoreUpdateWorkerNum int `validate:"gt=0" mapstructure:"score-update-worker-num"`

	// ScoreUpdateRequestQueueSize is the size of the worker pool for handling the application specific score update of peers in a non-blocking way.
	ScoreUpdateRequestQueueSize uint32 `validate:"gt=0" mapstructure:"score-update-request-queue-size"`

	// ScoreTTL is the time to live of the application specific score of a peer; the registry keeps a cached copy of the
	// application specific score of a peer for this duration. When the duration expires, the application specific score
	// of the peer is updated asynchronously. As long as the update is in progress, the cached copy of the application
	// specific score of the peer is used even if it is expired.
	ScoreTTL time.Duration `validate:"required" mapstructure:"score-ttl"`
}

const (
	PenaltyDecaySlowdownThresholdKey = "penalty-decay-slowdown-threshold"
	DecayRateReductionFactorKey      = "penalty-decay-rate-reduction-factor"
	PenaltyDecayEvaluationPeriodKey  = "penalty-decay-evaluation-period"
)

type SpamRecordCacheParameters struct {
	// CacheSize is size of the cache used to store the spam records of peers.
	// The spam records are used to penalize peers that send invalid messages.
	CacheSize uint32 `validate:"gt=0" mapstructure:"cache-size"`

	// PenaltyDecaySlowdownThreshold defines the penalty level which the decay rate is reduced by `DecayRateReductionFactor` every time the penalty of a node falls below the threshold, thereby slowing down the decay process.
	// This mechanism ensures that malicious nodes experience longer decay periods, while honest nodes benefit from quicker decay.
	PenaltyDecaySlowdownThreshold float64 `validate:"lt=0" mapstructure:"penalty-decay-slowdown-threshold"`

	// DecayRateReductionFactor defines the value by which the decay rate is decreased every time the penalty is below the PenaltyDecaySlowdownThreshold. A reduced decay rate extends the time it takes for penalties to diminish.
	DecayRateReductionFactor float64 `validate:"gt=0,lt=1" mapstructure:"penalty-decay-rate-reduction-factor"`

	// PenaltyDecayEvaluationPeriod defines the interval at which the decay for a spam record is okay to be adjusted.
	PenaltyDecayEvaluationPeriod time.Duration `validate:"gt=0" mapstructure:"penalty-decay-evaluation-period"`
}

// SubscriptionProviderParameters keys.
const (
	UpdateIntervalKey = "update-interval"
	CacheSizeKey      = "cache-size"
)

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
