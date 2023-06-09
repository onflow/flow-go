package validation

import (
	"fmt"

	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/network/p2p"
	internal "github.com/onflow/flow-go/network/p2p/inspector/internal/ratelimit"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit"
)

const (
	// HardThresholdMapKey key used to set the  hard threshold config limit.
	HardThresholdMapKey = "hardthreshold"
	// SafetyThresholdMapKey key used to set the safety threshold config limit.
	SafetyThresholdMapKey = "safetythreshold"
	// RateLimitMapKey key used to set the rate limit config limit.
	RateLimitMapKey = "ratelimit"
	// DefaultGraftHardThreshold upper bound for graft messages, if the RPC control message GRAFTs exceed this threshold  the RPC control message automatically discarded.
	DefaultGraftHardThreshold = 30
	// DefaultGraftSafetyThreshold a lower bound for graft messages, if the amount of GRAFTs in an RPC control message is below this threshold those GRAFTs validation will be bypassed.
	DefaultGraftSafetyThreshold = .5 * DefaultGraftHardThreshold
	// DefaultGraftRateLimit the rate limit for graft control messages.
	// Currently, the default rate limit is equal to the hard threshold amount.
	// This will result in a rate limit of 30 grafts/sec.
	DefaultGraftRateLimit = DefaultGraftHardThreshold

	// DefaultPruneHardThreshold upper bound for prune messages, if the RPC control message PRUNEs exceed this threshold  the RPC control message automatically discarded.
	DefaultPruneHardThreshold = 30
	// DefaultPruneSafetyThreshold a lower bound for prune messages, if the amount of PRUNEs in an RPC control message is below this threshold those GRAFTs validation will be bypassed.
	DefaultPruneSafetyThreshold = .5 * DefaultPruneHardThreshold

	// DefaultClusterPrefixedMsgDropThreshold is the maximum number of cluster-prefixed control messages allowed to be processed
	// when the cluster IDs provider has not been set or a node is behind in the protocol state. If the number of cluster-prefixed
	// control messages in an RPC exceeds this threshold, the entire RPC will be dropped and the node should be penalized.
	DefaultClusterPrefixedMsgDropThreshold = 100
	// DefaultPruneRateLimit the rate limit for prune control messages.
	// Currently, the default rate limit is equal to the hard threshold amount.
	// This will result in a rate limit of 30 prunes/sec.
	DefaultPruneRateLimit = DefaultPruneHardThreshold

	// DefaultIHaveHardThreshold upper bound for ihave messages, the message count for ihave messages
	// exceeds the configured hard threshold only a sample size of the messages will be inspected. This
	// ensures liveness of the network because there is no expected max number of ihave messages than can be
	// received by a node.
	DefaultIHaveHardThreshold = 100
	// DefaultIHaveSafetyThreshold a lower bound for ihave messages, if the amount of iHaves in an RPC control message is below this threshold those GRAFTs validation will be bypassed.
	DefaultIHaveSafetyThreshold = .5 * DefaultIHaveHardThreshold
	// DefaultIHaveRateLimit rate limiting for ihave control messages is disabled.
	DefaultIHaveRateLimit = 0
	// DefaultIHaveSyncInspectSampleSizePercentage the default percentage of ihaves to use as the sample size for synchronous inspection 25%.
	DefaultIHaveSyncInspectSampleSizePercentage = .25
	// DefaultIHaveAsyncInspectSampleSizePercentage the default percentage of ihaves to use as the sample size for asynchronous inspection 10%.
	DefaultIHaveAsyncInspectSampleSizePercentage = .10
	// DefaultIHaveInspectionMaxSampleSize the max number of ihave messages in a sample to be inspected.
	DefaultIHaveInspectionMaxSampleSize = 100
)

// CtrlMsgValidationLimits limits used to construct control message validation configuration.
type CtrlMsgValidationLimits map[string]int

func (c CtrlMsgValidationLimits) HardThreshold() uint64 {
	return uint64(c[HardThresholdMapKey])
}

func (c CtrlMsgValidationLimits) SafetyThreshold() uint64 {
	return uint64(c[SafetyThresholdMapKey])
}

func (c CtrlMsgValidationLimits) RateLimit() int {
	return c[RateLimitMapKey]
}

// CtrlMsgValidationConfigs list of *CtrlMsgValidationConfig
type CtrlMsgValidationConfigs []*CtrlMsgValidationConfig

// CtrlMsgValidationConfigOption options to set config values for a specific control message type.
type CtrlMsgValidationConfigOption func(*CtrlMsgValidationConfig)

// CtrlMsgValidationConfig configuration values for upper, lower threshold and rate limit.
type CtrlMsgValidationConfig struct {
	// ControlMsg the type of RPC control message.
	ControlMsg p2p.ControlMessageType
	// HardThreshold specifies the hard limit for the size of an RPC control message.
	// While it is generally expected that RPC messages with a size greater than HardThreshold should be dropped,
	// there are exceptions. For instance, if the message is an 'iHave', blocking processing is performed
	// on a sample of the control message rather than dropping it.
	HardThreshold uint64
	// SafetyThreshold specifies the lower limit for the size of the RPC control message, it is safe to skip validation for any RPC messages
	// with a size < SafetyThreshold. These messages will be processed as soon as possible.
	SafetyThreshold uint64
	// IHaveSyncInspectSampleSizePercentage the percentage of topics to sample for synchronous pre-processing of 'iHave' control messages. 'iHave' control messages
	// don't have an upper bound on the amount of 'iHaves' expected from a peer during normal operation. Due to this fact it is important to validate a sample percentage
	// of 'iHave' messages to ensure liveness of the network.
	IHaveSyncInspectSampleSizePercentage float64
	// IHaveAsyncInspectSampleSizePercentage  the percentage of topics to sample for asynchronous processing of 'iHave' control messages. 'iHave' control messages
	// don't have an upper bound on the amount of 'iHaves' expected from a peer during normal operation. Due to this fact it is important to validate a sample percentage
	// of 'iHave' messages to ensure liveness of the network.
	IHaveAsyncInspectSampleSizePercentage float64
	// IHaveInspectionMaxSampleSize the maximum size of the sample set of 'iHave' messages that will be validated.
	IHaveInspectionMaxSampleSize float64
	// RateLimiter basic limiter without lockout duration.
	RateLimiter p2p.BasicRateLimiter
}

// WithIHaveSyncInspectSampleSizePercentage option to set the IHaveSyncInspectSampleSizePercentage for ihave control message config.
func WithIHaveSyncInspectSampleSizePercentage(percentage float64) CtrlMsgValidationConfigOption {
	return func(config *CtrlMsgValidationConfig) {
		config.IHaveSyncInspectSampleSizePercentage = percentage
	}
}

// WithIHaveAsyncInspectSampleSizePercentage option to set the IHaveAsyncInspectSampleSizePercentage for ihave control message config.
func WithIHaveAsyncInspectSampleSizePercentage(percentage float64) CtrlMsgValidationConfigOption {
	return func(config *CtrlMsgValidationConfig) {
		config.IHaveAsyncInspectSampleSizePercentage = percentage
	}
}

// WithIHaveInspectionMaxSampleSize  option to set the IHaveInspectionMaxSampleSize for ihave control message config.
func WithIHaveInspectionMaxSampleSize(maxSampleSize float64) CtrlMsgValidationConfigOption {
	return func(config *CtrlMsgValidationConfig) {
		config.IHaveInspectionMaxSampleSize = maxSampleSize
	}
}

// NewCtrlMsgValidationConfig validates each config value before returning a new CtrlMsgValidationConfig.
// errors returned:
//
//	ErrValidationLimit - if any of the validation limits provided are less than 0. This error is non-recoverable
//	and the node should crash if this error is encountered.
func NewCtrlMsgValidationConfig(controlMsg p2p.ControlMessageType, cfgLimitValues CtrlMsgValidationLimits, opts ...CtrlMsgValidationConfigOption) (*CtrlMsgValidationConfig, error) {
	// check common config values used by all control message types
	switch {
	case cfgLimitValues.RateLimit() < 0:
		return nil, NewInvalidLimitConfigErr(controlMsg, RateLimitMapKey, uint64(cfgLimitValues.RateLimit()))
	case cfgLimitValues.HardThreshold() <= 0:
		return nil, NewInvalidLimitConfigErr(controlMsg, HardThresholdMapKey, cfgLimitValues.HardThreshold())
	case cfgLimitValues.SafetyThreshold() <= 0:
		return nil, NewInvalidLimitConfigErr(controlMsg, SafetyThresholdMapKey, cfgLimitValues.SafetyThreshold())
	}

	conf := &CtrlMsgValidationConfig{
		ControlMsg:      controlMsg,
		HardThreshold:   cfgLimitValues.HardThreshold(),
		SafetyThreshold: cfgLimitValues.SafetyThreshold(),
	}

	if cfgLimitValues.RateLimit() == 0 {
		// setup noop rate limiter if rate limiting is disabled
		conf.RateLimiter = ratelimit.NewNoopRateLimiter()
	} else {
		conf.RateLimiter = internal.NewControlMessageRateLimiter(rate.Limit(cfgLimitValues.RateLimit()), cfgLimitValues.RateLimit())
	}

	// options are used to set specialty config values for specific control message types
	for _, opt := range opts {
		opt(conf)
	}

	// perform any control message specific config validation
	switch controlMsg {
	case p2p.CtrlMsgIHave:
		switch {
		case conf.IHaveSyncInspectSampleSizePercentage <= 0:
			return nil, fmt.Errorf("invalid IHaveSyncInspectSampleSizePercentage config value must be greater than 0: %f", conf.IHaveSyncInspectSampleSizePercentage)
		case conf.IHaveAsyncInspectSampleSizePercentage <= 0:
			return nil, fmt.Errorf("invalid IHaveAsyncInspectSampleSizePercentage config value must be greater than 0: %f", conf.IHaveAsyncInspectSampleSizePercentage)
		case conf.IHaveInspectionMaxSampleSize <= 0:
			return nil, fmt.Errorf("invalid IHaveInspectionMaxSampleSize config value must be greater than 0: %f", conf.IHaveInspectionMaxSampleSize)
		}
	}

	return conf, nil
}
