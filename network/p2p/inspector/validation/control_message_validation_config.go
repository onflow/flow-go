package validation

import (
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/internal/ratelimit"
)

const (
	// DiscardThresholdMapKey key used to set the  discard threshold config limit.
	DiscardThresholdMapKey = "discardthreshold"
	// SafetyThresholdMapKey key used to set the safety threshold config limit.
	SafetyThresholdMapKey = "safetythreshold"
	// RateLimitMapKey key used to set the rate limit config limit.
	RateLimitMapKey = "ratelimit"
	// IHaveSyncInspectSampleSizeDivisorMapKey key used to set iHave synchronous inspection sample size divisor.
	IHaveSyncInspectSampleSizeDivisorMapKey = "ihaveSyncInspectSampleSizeDivisor"
	// IHaveAsyncInspectSampleSizeDivisorMapKey key used to set iHave asynchronous inspection sample size divisor.
	IHaveAsyncInspectSampleSizeDivisorMapKey = "ihaveAsyncInspectSampleSizeDivisor"

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
	DefaultIHaveSafetyThreshold = .5 * DefaultPruneDiscardThreshold
	// DefaultIHaveRateLimit the rate limit for ihave control messages.
	// Currently, the default rate limit is equal to the discard threshold amount.
	// This will result in a rate limit of 30 prunes/sec.
	DefaultIHaveRateLimit = DefaultPruneDiscardThreshold
	// DefaultIHaveSyncInspectSampleSizeDivisor the default divisor used to get a sample of the ihave control messages for synchronous pre-processing.
	// When the total number of ihave's is greater than the configured discard threshold. The sample will be the total number of ihave's / 4 or 25%.
	DefaultIHaveSyncInspectSampleSizeDivisor = 4
	// DefaultIHaveAsyncInspectSampleSizeDivisor the default divisor used to get a sample of the ihave control messages for asynchronous processing.
	// The sample will be the total number of ihave's / 10 or 10%.
	DefaultIHaveAsyncInspectSampleSizeDivisor = 10
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

func (c CtrlMsgValidationLimits) IHaveSyncInspectSampleSizeMultiplier() int {
	return c[IHaveSyncInspectSampleSizeDivisorMapKey]
}

func (c CtrlMsgValidationLimits) IHaveAsyncInspectSampleSizeMultiplier() int {
	return c[IHaveAsyncInspectSampleSizeDivisorMapKey]
}

// CtrlMsgValidationConfigs list of *CtrlMsgValidationConfig
type CtrlMsgValidationConfigs []*CtrlMsgValidationConfig

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

	// RateLimiter basic limiter without lockout duration.
	RateLimiter p2p.BasicRateLimiter
}

// NewCtrlMsgValidationConfig ensures each config limit value is greater than 0 before returning a new CtrlMsgValidationConfig.
// errors returned:
//
//	ErrValidationLimit - if any of the validation limits provided are less than 0. This error is non-recoverable
//	and the node should crash if this error is encountered.
func NewCtrlMsgValidationConfig(controlMsg p2p.ControlMessageType, cfgLimitValues CtrlMsgValidationLimits) (*CtrlMsgValidationConfig, error) {
	switch {
	case cfgLimitValues.RateLimit() <= 0:
		return nil, NewInvalidLimitConfigErr(controlMsg, RateLimitMapKey, uint64(cfgLimitValues.RateLimit()))
	case cfgLimitValues.DiscardThreshold() <= 0:
		return nil, NewInvalidLimitConfigErr(controlMsg, DiscardThresholdMapKey, cfgLimitValues.DiscardThreshold())
	case cfgLimitValues.RateLimit() <= 0:
		return nil, NewInvalidLimitConfigErr(controlMsg, SafetyThresholdMapKey, cfgLimitValues.SafetyThreshold())
	default:
		return &CtrlMsgValidationConfig{
			ControlMsg:       controlMsg,
			DiscardThreshold: cfgLimitValues.DiscardThreshold(),
			SafetyThreshold:  cfgLimitValues.SafetyThreshold(),
			RateLimiter:      ratelimit.NewControlMessageRateLimiter(rate.Limit(cfgLimitValues.RateLimit()), cfgLimitValues.RateLimit()),
		}, nil
	}
}
