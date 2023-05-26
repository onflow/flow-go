package internal

import (
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
)

// DefaultRPCValidationConfig returns default RPC control message validation inspector config.
func DefaultRPCValidationConfig(opts ...queue.HeroStoreConfigOption) *validation.ControlMsgValidationInspectorConfig {
	graftCfg, _ := validation.NewCtrlMsgValidationConfig(p2p.CtrlMsgGraft, validation.CtrlMsgValidationLimits{
		validation.HardThresholdMapKey:   validation.DefaultGraftHardThreshold,
		validation.SafetyThresholdMapKey: validation.DefaultGraftSafetyThreshold,
		validation.RateLimitMapKey:       validation.DefaultGraftRateLimit,
	})
	pruneCfg, _ := validation.NewCtrlMsgValidationConfig(p2p.CtrlMsgPrune, validation.CtrlMsgValidationLimits{
		validation.HardThresholdMapKey:   validation.DefaultPruneHardThreshold,
		validation.SafetyThresholdMapKey: validation.DefaultPruneSafetyThreshold,
		validation.RateLimitMapKey:       validation.DefaultPruneRateLimit,
	})
	iHaveOpts := []validation.CtrlMsgValidationConfigOption{
		validation.WithIHaveSyncInspectSampleSizePercentage(validation.DefaultIHaveSyncInspectSampleSizePercentage),
		validation.WithIHaveAsyncInspectSampleSizePercentage(validation.DefaultIHaveAsyncInspectSampleSizePercentage),
		validation.WithIHaveInspectionMaxSampleSize(validation.DefaultIHaveInspectionMaxSampleSize),
	}
	iHaveCfg, _ := validation.NewCtrlMsgValidationConfig(p2p.CtrlMsgIHave, validation.CtrlMsgValidationLimits{
		validation.HardThresholdMapKey:   validation.DefaultIHaveHardThreshold,
		validation.SafetyThresholdMapKey: validation.DefaultIHaveSafetyThreshold,
		validation.RateLimitMapKey:       validation.DefaultIHaveRateLimit,
	}, iHaveOpts...)
	return &validation.ControlMsgValidationInspectorConfig{
		NumberOfWorkers:     validation.DefaultNumberOfWorkers,
		InspectMsgStoreOpts: opts,
		GraftValidationCfg:  graftCfg,
		PruneValidationCfg:  pruneCfg,
		IHaveValidationCfg:  iHaveCfg,
		ClusterPrefixedMessageConfig: &validation.ClusterPrefixedMessageConfig{
			ClusterPrefixHardThreshold:                   validation.DefaultClusterPrefixedMsgDropThreshold,
			ClusterPrefixedControlMsgsReceivedCacheDecay: validation.DefaultClusterPrefixedControlMsgsReceivedCacheDecay,
			ClusterPrefixedControlMsgsReceivedCacheSize:  validation.DefaultClusterPrefixedControlMsgsReceivedCacheSize,
		},
	}
}
