package p2pconf

import (
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// GossipSubRPCInspectorsConfig encompasses configuration related to gossipsub RPC message inspectors.
type GossipSubRPCInspectorsConfig struct {
	// GossipSubRPCValidationInspectorConfigs control message validation inspector validation configuration and limits.
	GossipSubRPCValidationInspectorConfigs `mapstructure:",squash"`
	// GossipSubRPCMetricsInspectorConfigs control message metrics inspector configuration.
	GossipSubRPCMetricsInspectorConfigs `mapstructure:",squash"`
	// GossipSubRPCInspectorNotificationCacheSize size of the queue for notifications about invalid RPC messages.
	GossipSubRPCInspectorNotificationCacheSize uint32 `mapstructure:"gossipsub-rpc-inspector-notification-cache-size"`
}

// GossipSubRPCValidationInspectorConfigs validation limits used for gossipsub RPC control message inspection.
type GossipSubRPCValidationInspectorConfigs struct {
	ClusterPrefixedMessageConfig `mapstructure:",squash"`
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int `validate:"gte=1" mapstructure:"gossipsub-rpc-validation-inspector-workers"`
	// CacheSize size of the queue used by worker pool for the control message validation inspector.
	CacheSize uint32 `validate:"gte=100" mapstructure:"gossipsub-rpc-validation-inspector-queue-cache-size"`
	// GraftLimits GRAFT control message validation limits.
	GraftLimits struct {
		HardThreshold   uint64 `validate:"gt=0" mapstructure:"gossipsub-rpc-graft-hard-threshold"`
		SafetyThreshold uint64 `validate:"gt=0" mapstructure:"gossipsub-rpc-graft-safety-threshold"`
		RateLimit       int    `validate:"gte=0" mapstructure:"gossipsub-rpc-graft-rate-limit"`
	} `mapstructure:",squash"`
	// PruneLimits PRUNE control message validation limits.
	PruneLimits struct {
		HardThreshold   uint64 `validate:"gt=0" mapstructure:"gossipsub-rpc-prune-hard-threshold"`
		SafetyThreshold uint64 `validate:"gt=0" mapstructure:"gossipsub-rpc-prune-safety-threshold"`
		RateLimit       int    `validate:"gte=0" mapstructure:"gossipsub-rpc-prune-rate-limit"`
	} `mapstructure:",squash"`
	// IHaveLimits IHAVE control message validation limits.
	IHaveLimits struct {
		HardThreshold   uint64 `validate:"gt=0" mapstructure:"gossipsub-rpc-ihave-hard-threshold"`
		SafetyThreshold uint64 `validate:"gt=0" mapstructure:"gossipsub-rpc-ihave-safety-threshold"`
		RateLimit       int    `validate:"gte=0" mapstructure:"gossipsub-rpc-ihave-rate-limit"`
	} `mapstructure:",squash"`
	// IHaveSyncInspectSampleSizePercentage the percentage of topics to sample for sync pre-processing in float64 form.
	IHaveSyncInspectSampleSizePercentage float64 `validate:"gte=.25" mapstructure:"ihave-sync-inspection-sample-size-percentage"`
	// IHaveAsyncInspectSampleSizePercentage  the percentage of topics to sample for async pre-processing in float64 form.
	IHaveAsyncInspectSampleSizePercentage float64 `validate:"gte=.10" mapstructure:"ihave-async-inspection-sample-size-percentage"`
	// IHaveInspectionMaxSampleSize the max number of ihave messages in a sample to be inspected.
	IHaveInspectionMaxSampleSize float64 `validate:"gte=100" mapstructure:"ihave-max-sample-size"`
}

// GetCtrlMsgValidationConfig returns the CtrlMsgValidationConfig for the specified p2p.ControlMessageType.
func (conf *GossipSubRPCValidationInspectorConfigs) GetCtrlMsgValidationConfig(controlMsg p2pmsg.ControlMessageType) (*CtrlMsgValidationConfig, bool) {
	switch controlMsg {
	case p2pmsg.CtrlMsgGraft:
		return &CtrlMsgValidationConfig{
			ControlMsg:      p2pmsg.CtrlMsgGraft,
			HardThreshold:   conf.GraftLimits.HardThreshold,
			SafetyThreshold: conf.GraftLimits.SafetyThreshold,
			RateLimit:       conf.GraftLimits.RateLimit,
		}, true
	case p2pmsg.CtrlMsgPrune:
		return &CtrlMsgValidationConfig{
			ControlMsg:      p2pmsg.CtrlMsgPrune,
			HardThreshold:   conf.PruneLimits.HardThreshold,
			SafetyThreshold: conf.PruneLimits.SafetyThreshold,
			RateLimit:       conf.PruneLimits.RateLimit,
		}, true
	case p2pmsg.CtrlMsgIHave:
		return &CtrlMsgValidationConfig{
			ControlMsg:      p2pmsg.CtrlMsgIHave,
			HardThreshold:   conf.IHaveLimits.HardThreshold,
			SafetyThreshold: conf.IHaveLimits.SafetyThreshold,
			RateLimit:       conf.IHaveLimits.RateLimit,
		}, true
	default:
		return nil, false
	}
}

// AllCtrlMsgValidationConfig returns all control message validation configs in a list.
func (conf *GossipSubRPCValidationInspectorConfigs) AllCtrlMsgValidationConfig() CtrlMsgValidationConfigs {
	return CtrlMsgValidationConfigs{&CtrlMsgValidationConfig{
		ControlMsg:      p2pmsg.CtrlMsgGraft,
		HardThreshold:   conf.GraftLimits.HardThreshold,
		SafetyThreshold: conf.GraftLimits.SafetyThreshold,
		RateLimit:       conf.GraftLimits.RateLimit,
	}, &CtrlMsgValidationConfig{
		ControlMsg:      p2pmsg.CtrlMsgPrune,
		HardThreshold:   conf.PruneLimits.HardThreshold,
		SafetyThreshold: conf.PruneLimits.SafetyThreshold,
		RateLimit:       conf.PruneLimits.RateLimit,
	}, &CtrlMsgValidationConfig{
		ControlMsg:      p2pmsg.CtrlMsgIHave,
		HardThreshold:   conf.IHaveLimits.HardThreshold,
		SafetyThreshold: conf.IHaveLimits.SafetyThreshold,
		RateLimit:       conf.IHaveLimits.RateLimit,
	}}
}

// CtrlMsgValidationConfigs list of *CtrlMsgValidationConfig
type CtrlMsgValidationConfigs []*CtrlMsgValidationConfig

// CtrlMsgValidationConfig configuration values for upper, lower threshold and rate limit.
type CtrlMsgValidationConfig struct {
	// ControlMsg the type of RPC control message.
	ControlMsg p2pmsg.ControlMessageType
	// HardThreshold specifies the hard limit for the size of an RPC control message.
	// While it is generally expected that RPC messages with a size greater than HardThreshold should be dropped,
	// there are exceptions. For instance, if the message is an 'iHave', blocking processing is performed
	// on a sample of the control message rather than dropping it.
	HardThreshold uint64 `mapstructure:"hard-threshold"`
	// SafetyThreshold specifies the lower limit for the size of the RPC control message, it is safe to skip validation for any RPC messages
	// with a size < SafetyThreshold. These messages will be processed as soon as possible.
	SafetyThreshold uint64 `mapstructure:"safety-threshold"`
	// RateLimit number of allowed messages per second, use 0 to disable rate limiting.
	RateLimit int `mapstructure:"rate-limit"`
}

// ClusterPrefixedMessageConfig configuration values for cluster prefixed control message validation.
type ClusterPrefixedMessageConfig struct {
	// ClusterPrefixHardThreshold the upper bound on the amount of cluster prefixed control messages that will be processed
	// before a node starts to get penalized. This allows LN nodes to process some cluster prefixed control messages during startup
	// when the cluster ID's provider is set asynchronously. It also allows processing of some stale messages that may be sent by nodes
	// that fall behind in the protocol. After the amount of cluster prefixed control messages processed exceeds this threshold the node
	// will be pushed to the edge of the network mesh.
	ClusterPrefixHardThreshold float64 `validate:"gt=0" mapstructure:"gossipsub-rpc-cluster-prefixed-hard-threshold"`
	// ClusterPrefixedControlMsgsReceivedCacheSize size of the cache used to track the amount of cluster prefixed topics received by peers.
	ClusterPrefixedControlMsgsReceivedCacheSize uint32 `validate:"gt=0" mapstructure:"gossipsub-cluster-prefix-tracker-cache-size"`
	// ClusterPrefixedControlMsgsReceivedCacheDecay decay val used for the geometric decay of cache counters used to keep track of cluster prefixed topics received by peers.
	ClusterPrefixedControlMsgsReceivedCacheDecay float64 `validate:"gt=0" mapstructure:"gossipsub-cluster-prefix-tracker-cache-decay"`
}

// GossipSubRPCMetricsInspectorConfigs rpc metrics observer inspector configuration.
type GossipSubRPCMetricsInspectorConfigs struct {
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int `validate:"gte=1" mapstructure:"gossipsub-rpc-metrics-inspector-workers"`
	// CacheSize size of the queue used by worker pool for the control message metrics inspector.
	CacheSize uint32 `validate:"gt=0" mapstructure:"gossipsub-rpc-metrics-inspector-cache-size"`
}
