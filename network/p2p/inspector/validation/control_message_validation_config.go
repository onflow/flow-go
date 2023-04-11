package validation

import (
	"fmt"

	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/network/p2p"
	internal "github.com/onflow/flow-go/network/p2p/inspector/internal/ratelimit"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit"
)

const (
	// DiscardThresholdMapKey key used to set the  discard threshold config limit.
	DiscardThresholdMapKey = "discardthreshold"
	// SafetyThresholdMapKey key used to set the safety threshold config limit.
	SafetyThresholdMapKey = "safetythreshold"
	// RateLimitMapKey key used to set the rate limit config limit.
	RateLimitMapKey = "ratelimit"
	// DefaultGraftDiscardThreshold upper bound for graft messages, RPC control messages with a count
	// above the discard threshold are automatically discarded.
	DefaultGraftDiscardThreshold = 30
	// DefaultGraftSafetyThreshold a lower bound for graft messages, RPC control messages with a message count
	// lower than the safety threshold bypass validation.
	DefaultGraftSafetyThreshold = .5 * DefaultGraftDiscardThreshold
	// DefaultGraftRateLimit the rate limit for graft control messages.
	// Currently, the default rate limit is equal to the discard threshold amount.
	// This will result in a rate limit of 30 grafts/sec.
	DefaultGraftRateLimit = DefaultGraftDiscardThreshold

	// DefaultPruneDiscardThreshold upper bound for prune messages, RPC control messages with a count
	// above the discard threshold are automatically discarded.
	DefaultPruneDiscardThreshold = 30
	// DefaultPruneSafetyThreshold a lower bound for prune messages, RPC control messages with a message count
	// lower than the safety threshold bypass validation.
	DefaultPruneSafetyThreshold = .5 * DefaultPruneDiscardThreshold
	// DefaultPruneRateLimit the rate limit for prune control messages.
	// Currently, the default rate limit is equal to the discard threshold amount.
	// This will result in a rate limit of 30 prunes/sec.
	DefaultPruneRateLimit = DefaultPruneDiscardThreshold

	// DefaultIHaveDiscardThreshold upper bound for ihave messages, RPC control messages with a count
	// above the discard threshold are automatically discarded.
	DefaultIHaveDiscardThreshold = 100
	// DefaultIHaveSafetyThreshold a lower bound for ihave messages, RPC control messages with a message count
	// lower than the safety threshold bypass validation.
	DefaultIHaveSafetyThreshold = .5 * DefaultIHaveDiscardThreshold
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

func (c CtrlMsgValidationLimits) DiscardThreshold() uint64 {
	return uint64(c[DiscardThresholdMapKey])
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
	// DiscardThreshold indicates the hard limit for size of the RPC control message
	// any RPC messages with size > DiscardThreshold should be dropped.
	DiscardThreshold uint64
	// SafetyThreshold lower limit for the size of the RPC control message, any RPC messages
	// with a size < SafetyThreshold can skip validation step to avoid resource wasting.
	SafetyThreshold uint64
	// IHaveSyncInspectSampleSizePercentage the percentage of topics to sample for sync pre-processing in float64 form.
	IHaveSyncInspectSampleSizePercentage float64
	// IHaveAsyncInspectSampleSizePercentage  the percentage of topics to sample for async pre-processing in float64 form.
	IHaveAsyncInspectSampleSizePercentage float64
	// IHaveInspectionMaxSampleSize the max number of ihave messages in a sample to be inspected.
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
	case cfgLimitValues.DiscardThreshold() <= 0:
		return nil, NewInvalidLimitConfigErr(controlMsg, DiscardThresholdMapKey, cfgLimitValues.DiscardThreshold())
	case cfgLimitValues.SafetyThreshold() <= 0:
		return nil, NewInvalidLimitConfigErr(controlMsg, SafetyThresholdMapKey, cfgLimitValues.SafetyThreshold())
	}

	conf := &CtrlMsgValidationConfig{
		ControlMsg:       controlMsg,
		DiscardThreshold: cfgLimitValues.DiscardThreshold(),
		SafetyThreshold:  cfgLimitValues.SafetyThreshold(),
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
