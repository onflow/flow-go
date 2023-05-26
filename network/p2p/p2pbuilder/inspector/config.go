package inspector

import (
	"github.com/onflow/flow-go/network/p2p/distributor"
	"github.com/onflow/flow-go/network/p2p/inspector"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
)

// GossipSubRPCValidationInspectorConfigs validation limits used for gossipsub RPC control message inspection.
type GossipSubRPCValidationInspectorConfigs struct {
	*validation.ClusterPrefixedMessageConfig
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int
	// CacheSize size of the queue used by worker pool for the control message validation inspector.
	CacheSize uint32
	// GraftLimits GRAFT control message validation limits.
	GraftLimits map[string]int
	// PruneLimits PRUNE control message validation limits.
	PruneLimits map[string]int
	// IHaveLimitsConfig IHAVE control message validation limits configuration.
	IHaveLimitsConfig *GossipSubCtrlMsgIhaveLimitsConfig
}

// GossipSubRPCMetricsInspectorConfigs rpc metrics observer inspector configuration.
type GossipSubRPCMetricsInspectorConfigs struct {
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int
	// CacheSize size of the queue used by worker pool for the control message metrics inspector.
	CacheSize uint32
}

// GossipSubRPCInspectorsConfig encompasses configuration related to gossipsub RPC message inspectors.
type GossipSubRPCInspectorsConfig struct {
	// GossipSubRPCInspectorNotificationCacheSize size of the queue for notifications about invalid RPC messages.
	GossipSubRPCInspectorNotificationCacheSize uint32
	// ValidationInspectorConfigs control message validation inspector validation configuration and limits.
	ValidationInspectorConfigs *GossipSubRPCValidationInspectorConfigs
	// MetricsInspectorConfigs control message metrics inspector configuration.
	MetricsInspectorConfigs *GossipSubRPCMetricsInspectorConfigs
}

// GossipSubCtrlMsgIhaveLimitsConfig validation limit configs for ihave RPC control messages.
type GossipSubCtrlMsgIhaveLimitsConfig struct {
	// IHaveLimits IHAVE control message validation limits.
	IHaveLimits map[string]int
	// IHaveSyncInspectSampleSizePercentage the percentage of topics to sample for sync pre-processing in float64 form.
	IHaveSyncInspectSampleSizePercentage float64
	// IHaveAsyncInspectSampleSizePercentage  the percentage of topics to sample for async pre-processing in float64 form.
	IHaveAsyncInspectSampleSizePercentage float64
	// IHaveInspectionMaxSampleSize the max number of ihave messages in a sample to be inspected.
	IHaveInspectionMaxSampleSize float64
}

// IhaveConfigurationOpts returns list of options for the ihave configuration.
func (g *GossipSubCtrlMsgIhaveLimitsConfig) IhaveConfigurationOpts() []validation.CtrlMsgValidationConfigOption {
	return []validation.CtrlMsgValidationConfigOption{
		validation.WithIHaveSyncInspectSampleSizePercentage(g.IHaveSyncInspectSampleSizePercentage),
		validation.WithIHaveAsyncInspectSampleSizePercentage(g.IHaveAsyncInspectSampleSizePercentage),
		validation.WithIHaveInspectionMaxSampleSize(g.IHaveInspectionMaxSampleSize),
	}
}

// DefaultGossipSubRPCInspectorsConfig returns the default control message inspectors config.
func DefaultGossipSubRPCInspectorsConfig() *GossipSubRPCInspectorsConfig {
	return &GossipSubRPCInspectorsConfig{
		GossipSubRPCInspectorNotificationCacheSize: distributor.DefaultGossipSubInspectorNotificationQueueCacheSize,
		ValidationInspectorConfigs: &GossipSubRPCValidationInspectorConfigs{
			NumberOfWorkers: validation.DefaultNumberOfWorkers,
			CacheSize:       validation.DefaultControlMsgValidationInspectorQueueCacheSize,
			ClusterPrefixedMessageConfig: &validation.ClusterPrefixedMessageConfig{
				ClusterPrefixedControlMsgsReceivedCacheSize:  validation.DefaultClusterPrefixedControlMsgsReceivedCacheSize,
				ClusterPrefixedControlMsgsReceivedCacheDecay: validation.DefaultClusterPrefixedControlMsgsReceivedCacheDecay,
				ClusterPrefixHardThreshold:                   validation.DefaultClusterPrefixedMsgDropThreshold,
			},
			GraftLimits: map[string]int{
				validation.HardThresholdMapKey:   validation.DefaultGraftHardThreshold,
				validation.SafetyThresholdMapKey: validation.DefaultGraftSafetyThreshold,
				validation.RateLimitMapKey:       validation.DefaultGraftRateLimit,
			},
			PruneLimits: map[string]int{
				validation.HardThresholdMapKey:   validation.DefaultPruneHardThreshold,
				validation.SafetyThresholdMapKey: validation.DefaultPruneSafetyThreshold,
				validation.RateLimitMapKey:       validation.DefaultPruneRateLimit,
			},
			IHaveLimitsConfig: &GossipSubCtrlMsgIhaveLimitsConfig{
				IHaveLimits: validation.CtrlMsgValidationLimits{
					validation.HardThresholdMapKey:   validation.DefaultIHaveHardThreshold,
					validation.SafetyThresholdMapKey: validation.DefaultIHaveSafetyThreshold,
					validation.RateLimitMapKey:       validation.DefaultIHaveRateLimit,
				},
				IHaveSyncInspectSampleSizePercentage:  validation.DefaultIHaveSyncInspectSampleSizePercentage,
				IHaveAsyncInspectSampleSizePercentage: validation.DefaultIHaveAsyncInspectSampleSizePercentage,
				IHaveInspectionMaxSampleSize:          validation.DefaultIHaveInspectionMaxSampleSize,
			},
		},
		MetricsInspectorConfigs: &GossipSubRPCMetricsInspectorConfigs{
			NumberOfWorkers: inspector.DefaultControlMsgMetricsInspectorNumberOfWorkers,
			CacheSize:       inspector.DefaultControlMsgMetricsInspectorQueueCacheSize,
		},
	}
}
