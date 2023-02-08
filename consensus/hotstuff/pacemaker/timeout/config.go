package timeout

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/updatable_configs"
)

// Config contains the configuration parameters for a Truncated Exponential Backoff,
// as implemented by the `timeout.Controller`
//   - On timeout: increase timeout by multiplicative factor `TimeoutAdjustmentFactor`. This
//     results in exponentially growing timeout duration on multiple subsequent timeouts.
//   - On progress: decrease timeout by multiplicative factor `TimeoutAdjustmentFactor.
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
	// BlockRateDelayMS is a delay to broadcast the proposal in order to control block production rate [MILLISECONDS]
	BlockRateDelayMS *atomic.Float64
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

	conf, err := NewConfig(
		minReplicaTimeout+blockRateDelay,
		maxReplicaTimeout,
		timeoutAdjustmentFactorFactor,
		happyPathMaxRoundFailures,
		blockRateDelay,
		maxRebroadcastInterval,
	)
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
func NewConfig(
	minReplicaTimeout time.Duration,
	maxReplicaTimeout time.Duration,
	timeoutAdjustmentFactor float64,
	happyPathMaxRoundFailures uint64,
	blockRateDelay time.Duration,
	maxRebroadcastInterval time.Duration,
) (Config, error) {
	if minReplicaTimeout <= 0 {
		return Config{}, model.NewConfigurationErrorf("minReplicaTimeout must be a positive number[milliseconds]")
	}
	if maxReplicaTimeout < minReplicaTimeout {
		return Config{}, model.NewConfigurationErrorf("maxReplicaTimeout cannot be smaller than minReplicaTimeout")
	}
	if timeoutAdjustmentFactor <= 1 {
		return Config{}, model.NewConfigurationErrorf("timeoutAdjustmentFactor must be strictly bigger than 1")
	}
	if err := validBlockRateDelay(blockRateDelay); err != nil {
		return Config{}, err
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
		BlockRateDelayMS:                    atomic.NewFloat64(float64(blockRateDelay.Milliseconds())),
	}
	return tc, nil
}

// validBlockRateDelay validates a block rate delay config.
// Returns model.ConfigurationError for invalid config inputs.
func validBlockRateDelay(blockRateDelay time.Duration) error {
	if blockRateDelay < 0 {
		return model.NewConfigurationErrorf("blockRateDelay must be must be non-negative")
	}
	return nil
}

// GetBlockRateDelay returns the block rate delay as a Duration. This is used by
// the dyamic config manager.
func (c *Config) GetBlockRateDelay() time.Duration {
	ms := c.BlockRateDelayMS.Load()
	return time.Millisecond * time.Duration(ms)
}

// SetBlockRateDelay sets the block rate delay. It is used to modify this config
// value while HotStuff is running.
// Returns updatable_configs.ValidationError if the new value is invalid.
func (c *Config) SetBlockRateDelay(delay time.Duration) error {
	if err := validBlockRateDelay(delay); err != nil {
		if model.IsConfigurationError(err) {
			return updatable_configs.NewValidationErrorf("invalid block rate delay: %w", err)
		}
		return fmt.Errorf("unexpected error validating block rate delay: %w", err)
	}
	// sanity check: log a warning if we set block rate delay above min timeout
	// it is valid to want to do this, to significantly slow the block rate, but
	// only in edge cases
	if c.MinReplicaTimeout < float64(delay.Milliseconds()) {
		log.Warn().Msgf("CAUTION: setting block rate delay to %s, above min timeout %dms - this will degrade performance!", delay.String(), int64(c.MinReplicaTimeout))
	}
	c.BlockRateDelayMS.Store(float64(delay.Milliseconds()))
	return nil
}
