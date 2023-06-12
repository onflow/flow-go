package network

import (
	"fmt"

	"github.com/onflow/flow-go/network/p2p"
)

// GossipSubRPCInspectorsConfig encompasses configuration related to gossipsub RPC message inspectors.
type GossipSubRPCInspectorsConfig struct {
	// GossipSubRPCInspectorNotificationCacheSize size of the queue for notifications about invalid RPC messages.
	GossipSubRPCInspectorNotificationCacheSize uint32 `mapstructure:"notification-cache-size"`
	// ValidationInspectorConfigs control message validation inspector validation configuration and limits.
	ValidationInspectorConfigs *GossipSubRPCValidationInspectorConfigs `mapstructure:"validation-inspector"`
	// MetricsInspectorConfigs control message metrics inspector configuration.
	MetricsInspectorConfigs *GossipSubRPCMetricsInspectorConfigs `mapstructure:"metrics-inspector"`
}

// Validate validates rpc inspectors configuration values.
func (c *GossipSubRPCInspectorsConfig) Validate() error {
	// validate all limit configuration values
	err := c.ValidationInspectorConfigs.GraftLimits.Validate()
	if err != nil {
		return err
	}
	err = c.ValidationInspectorConfigs.PruneLimits.Validate()
	if err != nil {
		return err
	}
	err = c.ValidationInspectorConfigs.IHaveLimits.Validate()
	if err != nil {
		return err
	}
	return nil
}

// GossipSubRPCValidationInspectorConfigs validation limits used for gossipsub RPC control message inspection.
type GossipSubRPCValidationInspectorConfigs struct {
	*ClusterPrefixedMessageConfig `mapstructure:"cluster-prefixed-messages"`
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int `mapstructure:"number-of-workers"`
	// CacheSize size of the queue used by worker pool for the control message validation inspector.
	CacheSize uint32 `mapstructure:"queue-cache-size"`
	// GraftLimits GRAFT control message validation limits.
	GraftLimits *CtrlMsgValidationConfig `mapstructure:"graft-limits"`
	// PruneLimits PRUNE control message validation limits.
	PruneLimits *CtrlMsgValidationConfig `mapstructure:"prune-limits"`
	// IHaveLimits IHAVE control message validation limits.
	IHaveLimits *CtrlMsgValidationConfig `mapstructure:"ihave-limits"`
	// IHaveSyncInspectSampleSizePercentage the percentage of topics to sample for sync pre-processing in float64 form.
	IHaveSyncInspectSampleSizePercentage float64 `mapstructure:"ihave-sync-inspection-sample-size-percentage"`
	// IHaveAsyncInspectSampleSizePercentage  the percentage of topics to sample for async pre-processing in float64 form.
	IHaveAsyncInspectSampleSizePercentage float64 `mapstructure:"ihave-async-inspection-sample-size-percentage"`
	// IHaveInspectionMaxSampleSize the max number of ihave messages in a sample to be inspected.
	IHaveInspectionMaxSampleSize float64 `mapstructure:"ihave-max-sample-size"`
}

// GetCtrlMsgValidationConfig returns the CtrlMsgValidationConfig for the specified p2p.ControlMessageType.
func (conf *GossipSubRPCValidationInspectorConfigs) GetCtrlMsgValidationConfig(controlMsg p2p.ControlMessageType) (*CtrlMsgValidationConfig, bool) {
	switch controlMsg {
	case p2p.CtrlMsgGraft:
		return conf.GraftLimits, true
	case p2p.CtrlMsgPrune:
		return conf.PruneLimits, true
	case p2p.CtrlMsgIHave:
		return conf.IHaveLimits, true
	default:
		return nil, false
	}
}

// AllCtrlMsgValidationConfig returns all control message validation configs in a list.
func (conf *GossipSubRPCValidationInspectorConfigs) AllCtrlMsgValidationConfig() CtrlMsgValidationConfigs {
	return CtrlMsgValidationConfigs{conf.GraftLimits, conf.PruneLimits, conf.IHaveLimits}
}

// CtrlMsgValidationConfigs list of *CtrlMsgValidationConfig
type CtrlMsgValidationConfigs []*CtrlMsgValidationConfig

// CtrlMsgValidationConfig configuration values for upper, lower threshold and rate limit.
type CtrlMsgValidationConfig struct {
	// ControlMsg the type of RPC control message.
	ControlMsg p2p.ControlMessageType `mapstructure:"control-message-type"`
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
	// rateLimiter basic limiter without lockout duration.
	rateLimiter p2p.BasicRateLimiter
}

// Validate validates control message validation limit values.
func (c *CtrlMsgValidationConfig) Validate() error {
	// check common config values used by all control message types
	switch {
	case c.RateLimit < 0:
		return NewInvalidLimitConfigErr(c.ControlMsg, fmt.Errorf("invalid rate limit value %d must be greater than 0", c.RateLimit))
	case c.HardThreshold <= 0:
		return NewInvalidLimitConfigErr(c.ControlMsg, fmt.Errorf("invalid hard threshold value %d must be greater than 0", c.HardThreshold))
	case c.SafetyThreshold <= 0:
		return NewInvalidLimitConfigErr(c.ControlMsg, fmt.Errorf("invalid safety threshold value %d must be greater than 0", c.SafetyThreshold))
	}
	return nil
}

// ClusterPrefixedMessageConfig configuration values for cluster prefixed control message validation.
type ClusterPrefixedMessageConfig struct {
	// ClusterPrefixHardThreshold the upper bound on the amount of cluster prefixed control messages that will be processed
	// before a node starts to get penalized. This allows LN nodes to process some cluster prefixed control messages during startup
	// when the cluster ID's provider is set asynchronously. It also allows processing of some stale messages that may be sent by nodes
	// that fall behind in the protocol. After the amount of cluster prefixed control messages processed exceeds this threshold the node
	// will be pushed to the edge of the network mesh.
	ClusterPrefixHardThreshold float64 `mapstructure:"hard-threshold"`
	// ClusterPrefixedControlMsgsReceivedCacheSize size of the cache used to track the amount of cluster prefixed topics received by peers.
	ClusterPrefixedControlMsgsReceivedCacheSize uint32 `mapstructure:"tracker-cache-size"`
	// ClusterPrefixedControlMsgsReceivedCacheDecay decay val used for the geometric decay of cache counters used to keep track of cluster prefixed topics received by peers.
	ClusterPrefixedControlMsgsReceivedCacheDecay float64 `mapstructure:"tracker-cache-decay"`
}

// GossipSubRPCMetricsInspectorConfigs rpc metrics observer inspector configuration.
type GossipSubRPCMetricsInspectorConfigs struct {
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int `mapstructure:"number-of-workers"`
	// CacheSize size of the queue used by worker pool for the control message metrics inspector.
	CacheSize uint32 `mapstructure:"cache-size"`
}
