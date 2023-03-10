package validation

import (
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/internal/ratelimit"
)

type ControlMsg string

const (
	UpperThresholdMapKey  = "UpperThreshold"
	SafetyThresholdMapKey = "SafetyThreshold"
	RateLimitMapKey       = "RateLimit"

	ControlMsgIHave ControlMsg = "iHave"
	ControlMsgIWant ControlMsg = "iWant"
	ControlMsgGraft ControlMsg = "Graft"
	ControlMsgPrune ControlMsg = "Prune"

	DefaultGraftUpperThreshold  = 1000
	DefaultGraftSafetyThreshold = 100
	DefaultGraftRateLimit       = 1000

	DefaultPruneUpperThreshold  = 1000
	DefaultPruneSafetyThreshold = 20
	DefaultPruneRateLimit       = 1000
)

// CtrlMsgValidationLimits limits used to construct control message validation configuration.
type CtrlMsgValidationLimits map[string]int

func (c CtrlMsgValidationLimits) UpperThreshold() int {
	return c[UpperThresholdMapKey]
}

func (c CtrlMsgValidationLimits) SafetyThreshold() int {
	return c[SafetyThresholdMapKey]
}

func (c CtrlMsgValidationLimits) RateLimit() int {
	return c[RateLimitMapKey]
}

// CtrlMsgValidationConfigs list of *CtrlMsgValidationConfig
type CtrlMsgValidationConfigs []*CtrlMsgValidationConfig

// CtrlMsgValidationConfig configuration values for upper, lower threshold and rate limit.
type CtrlMsgValidationConfig struct {
	// ControlMsg the type of RPC control message.
	ControlMsg ControlMsg
	// UpperThreshold indicates the hard limit for size of the RPC control message
	// any RPC messages with size > UpperThreshold should be dropped.
	UpperThreshold int
	// SafetyThreshold lower limit for the size of the RPC control message, any RPC messages
	// with a size < SafetyThreshold can skip validation step to avoid resource wasting.
	SafetyThreshold int
	//RateLimit rate limit used for rate limiter, this is a per second limit.
	RateLimit int
	// RateLimiter basic limiter without lockout duration.
	RateLimiter p2p.BasicRateLimiter
}

// NewCtrlMsgValidationConfig ensures each config limit value is greater than 0 before returning a new CtrlMsgValidationConfig.
// errors returned:
//
//	ErrValidationLimit if any of the validation limits provided are less than 0.
func NewCtrlMsgValidationConfig(controlMsg ControlMsg, cfgLimitValues CtrlMsgValidationLimits) (*CtrlMsgValidationConfig, error) {
	switch {
	case cfgLimitValues.RateLimit() <= 0:
		return nil, NewValidationLimitErr(controlMsg, RateLimitMapKey, cfgLimitValues.RateLimit())
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
