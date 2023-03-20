package validation

import (
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/internal/ratelimit"
)

const (
	UpperThresholdMapKey  = "upperthreshold"
	SafetyThresholdMapKey = "safetythreshold"
	RateLimitMapKey       = "ratelimit"

	// DefaultGraftUpperThreshold upper bound for graft messages, RPC control messages with a count
	// above the upper threshold are automatically discarded.
	DefaultGraftUpperThreshold = 30
	// DefaultGraftSafetyThreshold a lower bound for graft messages, RPC control messages with a message count
	// lower than the safety threshold bypass validation.
	DefaultGraftSafetyThreshold = .5 * DefaultGraftUpperThreshold
	// DefaultGraftRateLimit the rate limit for graft control messages.
	// Currently, the default rate limit is equal to the upper threshold amount.
	// This will result in a rate limit of 30 grafts/sec.
	DefaultGraftRateLimit = DefaultGraftUpperThreshold

	// DefaultPruneUpperThreshold upper bound for prune messages, RPC control messages with a count
	// above the upper threshold are automatically discarded.
	DefaultPruneUpperThreshold = 30
	// DefaultPruneSafetyThreshold a lower bound for prune messages, RPC control messages with a message count
	// lower than the safety threshold bypass validation.
	DefaultPruneSafetyThreshold = .5 * DefaultPruneUpperThreshold
	// DefaultPruneRateLimit the rate limit for prune control messages.
	// Currently, the default rate limit is equal to the upper threshold amount.
	// This will result in a rate limit of 30 prunes/sec.
	DefaultPruneRateLimit = DefaultPruneUpperThreshold
)

// CtrlMsgValidationLimits limits used to construct control message validation configuration.
type CtrlMsgValidationLimits map[string]int

func (c CtrlMsgValidationLimits) UpperThreshold() uint64 {
	return uint64(c[UpperThresholdMapKey])
}

func (c CtrlMsgValidationLimits) SafetyThreshold() uint64 {
	return uint64(c[SafetyThresholdMapKey])
}

func (c CtrlMsgValidationLimits) RateLimit() int {
	return int(c[RateLimitMapKey])
}

// CtrlMsgValidationConfigs list of *CtrlMsgValidationConfig
type CtrlMsgValidationConfigs []*CtrlMsgValidationConfig

// CtrlMsgValidationConfig configuration values for upper, lower threshold and rate limit.
type CtrlMsgValidationConfig struct {
	// ControlMsg the type of RPC control message.
	ControlMsg p2p.ControlMessageType
	// UpperThreshold indicates the hard limit for size of the RPC control message
	// any RPC messages with size > UpperThreshold should be dropped.
	UpperThreshold uint64
	// SafetyThreshold lower limit for the size of the RPC control message, any RPC messages
	// with a size < SafetyThreshold can skip validation step to avoid resource wasting.
	SafetyThreshold uint64
	//RateLimit rate limit used for rate limiter, this is a per second limit.
	RateLimit int
	// RateLimiter basic limiter without lockout duration.
	RateLimiter p2p.BasicRateLimiter
}

// NewCtrlMsgValidationConfig ensures each config limit value is greater than 0 before returning a new CtrlMsgValidationConfig.
// errors returned:
//
//	ErrValidationLimit if any of the validation limits provided are less than 0.
func NewCtrlMsgValidationConfig(controlMsg p2p.ControlMessageType, cfgLimitValues CtrlMsgValidationLimits) (*CtrlMsgValidationConfig, error) {
	switch {
	case cfgLimitValues.RateLimit() <= 0:
		return nil, NewValidationLimitErr(controlMsg, RateLimitMapKey, uint64(cfgLimitValues.RateLimit()))
	case cfgLimitValues.UpperThreshold() <= 0:
		return nil, NewValidationLimitErr(controlMsg, UpperThresholdMapKey, cfgLimitValues.UpperThreshold())
	case cfgLimitValues.RateLimit() <= 0:
		return nil, NewValidationLimitErr(controlMsg, SafetyThresholdMapKey, cfgLimitValues.SafetyThreshold())
	default:
		return &CtrlMsgValidationConfig{
			ControlMsg:      controlMsg,
			UpperThreshold:  cfgLimitValues.UpperThreshold(),
			SafetyThreshold: cfgLimitValues.SafetyThreshold(),
			RateLimit:       cfgLimitValues.RateLimit(),
			RateLimiter:     ratelimit.NewControlMessageRateLimiter(rate.Limit(cfgLimitValues.RateLimit()), cfgLimitValues.RateLimit()),
		}, nil
	}
}
