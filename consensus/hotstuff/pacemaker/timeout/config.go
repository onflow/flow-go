package timeout

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// Config contains the configuration parameters for a Truncated Exponential Backoff,
// as implemented by the `timeout.Controller`
//   - On timeout: increase timeout by multiplicative factor `TimeoutAdjustmentFactor`. This
//     results in exponentially growing timeout duration on multiple subsequent timeouts.
//   - On progress: decrease timeout by multiplicative factor `TimeoutAdjustmentFactor.
//
// Config is implemented such that it can be passed by value, while still supporting updates of
// `BlockRateDelayMS` at runtime (all configs share the same memory holding `BlockRateDelayMS`).
type Config struct {
	// MinReplicaTimeout is the minimum the timeout can decrease to [MILLISECONDS]
	MinReplicaTimeout float64
	// MaxReplicaTimeout is the maximum value the timeout can increase to [MILLISECONDS]
	MaxReplicaTimeout float64
	// TimeoutAdjustmentFactor: MULTIPLICATIVE factor for increasing timeout when view
	// change was triggered by a TC (unhappy path) or decreasing the timeout on progress
	TimeoutAdjustmentFactor float64
	// HappyPathMaxRoundFailures is the number of rounds without progress where we still consider being
	// on hot path of execution. After exceeding this value we will start increasing timeout values.
	HappyPathMaxRoundFailures uint64
	// MaxTimeoutObjectRebroadcastInterval is the maximum value for timeout object rebroadcast interval [MILLISECONDS]
	MaxTimeoutObjectRebroadcastInterval float64
}

var DefaultConfig = NewDefaultConfig()

// NewDefaultConfig returns a default timeout configuration.
// We explicitly provide a method here, which demonstrates in-code how
// to compute standard values from some basic quantities.
func NewDefaultConfig() Config {
	// minReplicaTimeout is the lower bound on the replica's timeout value, this is also the initial timeout with what replicas
	// will start their execution.
	// If HotStuff is running at full speed, 1200ms should be enough. However, we add some buffer.
	// This value is for instant message delivery.
	minReplicaTimeout := 3 * time.Second
	maxReplicaTimeout := 1 * time.Minute
	timeoutAdjustmentFactorFactor := 1.2
	// after 6 successively failed rounds, the pacemaker leaves the hot path and starts increasing timeouts (recovery mode)
	happyPathMaxRoundFailures := uint64(6)
	blockRateDelay := 0 * time.Millisecond
	maxRebroadcastInterval := 5 * time.Second

	conf, err := NewConfig(minReplicaTimeout+blockRateDelay, maxReplicaTimeout, timeoutAdjustmentFactorFactor, happyPathMaxRoundFailures, maxRebroadcastInterval)
	if err != nil {
		// we check in a unit test that this does not happen
		panic("Default config is not compliant with timeout Config requirements")
	}

	return conf
}

// NewConfig creates a new TimoutConfig.
//   - minReplicaTimeout: minimal timeout value for replica round [Milliseconds]
//     Consistency requirement: must be non-negative
//   - maxReplicaTimeout: maximal timeout value for replica round [Milliseconds]
//     Consistency requirement: must be non-negative and cannot be smaller than minReplicaTimeout
//   - timeoutAdjustmentFactor: multiplicative factor for adjusting timeout duration
//     Consistency requirement: must be strictly larger than 1
//   - happyPathMaxRoundFailures: number of successive failed rounds after which we will start increasing timeouts
//   - blockRateDelay: a delay to delay the proposal broadcasting [Milliseconds]
//     Consistency requirement: must be non-negative
//
// Returns `model.ConfigurationError` is any of the consistency requirements is violated.
func NewConfig(minReplicaTimeout time.Duration, maxReplicaTimeout time.Duration, timeoutAdjustmentFactor float64, happyPathMaxRoundFailures uint64, maxRebroadcastInterval time.Duration) (Config, error) {
	if minReplicaTimeout <= 0 {
		return Config{}, model.NewConfigurationErrorf("minReplicaTimeout must be a positive number[milliseconds]")
	}
	if maxReplicaTimeout < minReplicaTimeout {
		return Config{}, model.NewConfigurationErrorf("maxReplicaTimeout cannot be smaller than minReplicaTimeout")
	}
	if timeoutAdjustmentFactor <= 1 {
		return Config{}, model.NewConfigurationErrorf("timeoutAdjustmentFactor must be strictly bigger than 1")
	}
	if maxRebroadcastInterval <= 0 {
		return Config{}, model.NewConfigurationErrorf("maxRebroadcastInterval must be a positive number [milliseconds]")
	}

	tc := Config{
		MinReplicaTimeout:                   float64(minReplicaTimeout.Milliseconds()),
		MaxReplicaTimeout:                   float64(maxReplicaTimeout.Milliseconds()),
		TimeoutAdjustmentFactor:             timeoutAdjustmentFactor,
		HappyPathMaxRoundFailures:           happyPathMaxRoundFailures,
		MaxTimeoutObjectRebroadcastInterval: float64(maxRebroadcastInterval.Milliseconds()),
	}
	return tc, nil
}
