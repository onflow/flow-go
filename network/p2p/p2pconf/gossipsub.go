package p2pconf

import (
	"time"

	"github.com/go-playground/validator/v10"
)

// ResourceManagerConfig returns the resource manager configuration for the libp2p node.
// The resource manager is used to limit the number of open connections and streams (as well as any other resources
// used by libp2p) for each peer.
type ResourceManagerConfig struct {
	InboundStream             InboundStreamLimit `mapstructure:",squash"`
	MemoryLimitRatio          float64            `mapstructure:"libp2p-memory-limit-ratio"`             // maximum allowed fraction of memory to be allocated by the libp2p resources in (0,1]
	FileDescriptorsRatio      float64            `mapstructure:"libp2p-file-descriptors-ratio"`         // maximum allowed fraction of file descriptors to be allocated by the libp2p resources in (0,1]
	PeerBaseLimitConnsInbound int                `mapstructure:"libp2p-peer-base-limits-conns-inbound"` // the maximum amount of allowed inbound connections per peer
}

// InboundStreamLimit is the configuration for the inbound stream limit. The inbound stream limit is used to limit the
// number of inbound streams that can be opened by the node.
type InboundStreamLimit struct {
	// the system-wide limit on the number of inbound streams
	System int `validate:"gt=0" mapstructure:"libp2p-inbound-stream-limit-system"`

	// Transient is the transient limit on the number of inbound streams (applied to streams that are not associated with a peer or protocol yet)
	Transient int `validate:"gt=0" mapstructure:"libp2p-inbound-stream-limit-transient"`

	// Protocol is the limit on the number of inbound streams per protocol (over all peers).
	Protocol int `validate:"gt=0" mapstructure:"libp2p-inbound-stream-limit-protocol"`

	// Peer is the limit on the number of inbound streams per peer (over all protocols).
	Peer int `validate:"gt=0" mapstructure:"libp2p-inbound-stream-limit-peer"`

	// ProtocolPeer is the limit on the number of inbound streams per protocol per peer.
	ProtocolPeer int `validate:"gt=0" mapstructure:"libp2p-inbound-stream-limit-protocol-peer"`
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
	// SlowerDecayPenaltyThreshold defines the penalty level which the decay rate is reduced by `DecayRateDecrement` every time the penalty of a node falls below the threshold, thereby slowing down the decay process.
	// This mechanism ensures that malicious nodes experience longer decay periods, while honest nodes benefit from quicker decay.
	SlowerDecayPenaltyThreshold float64 `validate:"gt=-100,lt=0" mapstructure:"gossipsub-scoring-registry-slower-decay-threshold"`
	// DecayRateDecrement defines the value by which the decay rate is decreased every time the penalty is below the SlowerDecayPenaltyThreshold. A reduced decay rate extends the time it takes for penalties to diminish.
	DecayRateDecrement float64 `validate:"gt=0,lt=1" mapstructure:"gossipsub-scoring-registry-decay-rate-decrement"`
	// DecayAdjustInterval defines the interval at which the decay for a spam record is okay to be adjusted.
	DecayAdjustInterval time.Duration `validate:"required,ScoringRegistryDecayAdjustIntervalValidator" mapstructure:"gossipsub-scoring-registry-decay-adjust-interval"`
}

// ScoringRegistryDecayAdjustIntervalValidator ensures DecayAdjustInterval field is greater than 0.
func ScoringRegistryDecayAdjustIntervalValidator(fl validator.FieldLevel) bool {
	// Get the duration value from the field
	duration, ok := fl.Field().Interface().(time.Duration)
	if !ok {
		return false
	}
	return duration > 0
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
