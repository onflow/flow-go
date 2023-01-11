package timeout

import (
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/updatable_configs"
)

// Config contains the configuration parameters for ExponentialIncrease-LinearDecrease
// timeout.Controller
//   - on timeout: increase timeout by multiplicative factor `timeoutIncrease` (user-specified)
//     this results in exponential growing timeout duration on multiple subsequent timeouts
//   - on progress: MULTIPLICATIVE timeout decrease
type Config struct {
	// ReplicaTimeout is the duration of a view before we time out [MILLISECONDS]
	// ReplicaTimeout is the only variable quantity
	ReplicaTimeout float64
	// MinReplicaTimeout is the minimum the timeout can decrease to [MILLISECONDS]
	MinReplicaTimeout float64
	// VoteAggregationTimeoutFraction is the FRACTION of ReplicaTimeout which the Primary
	// will maximally wait to collect enough votes before building a block (with an old qc)
	VoteAggregationTimeoutFraction float64
	// TimeoutDecrease: MULTIPLICATIVE factor for increasing timeout on timeout
	TimeoutIncrease float64
	// TimeoutDecrease: MULTIPLICATIVE factor for decreasing timeout on progress
	TimeoutDecrease float64
	// BlockRateDelayMS is a delay to broadcast the proposal in order to control block production rate [MILLISECONDS]
	BlockRateDelayMS *atomic.Float64
}

var DefaultConfig = NewDefaultConfig()

// NewDefaultConfig returns a default timeout configuration.
// We explicitly provide a method here, which demonstrates in-code how
// to compute standard values from some basic quantities.
func NewDefaultConfig() Config {
	// the replicas will start with 60 second time out to allow all other replicas to come online
	// once the replica's views are synchronized, the timeout will decrease to more reasonable values
	replicaTimeout := 60 * time.Second

	// the lower bound on the replicaTimeout value
	// If HotStuff is running at full speed, 1200ms should be enough. However, we add some buffer.
	// This value is for instant message delivery.
	minReplicaTimeout := 2 * time.Second
	timeoutIncreaseFactor := 2.0
	blockRateDelay := 0 * time.Millisecond

	// the following demonstrates the computation of standard values
	conf, err := NewConfig(
		replicaTimeout,
		minReplicaTimeout+blockRateDelay,
		StandardVoteAggregationTimeoutFraction(minReplicaTimeout, blockRateDelay), // resulting value here is 0.5
		timeoutIncreaseFactor,
		StandardTimeoutDecreaseFactor(1.0/3.0, timeoutIncreaseFactor), // resulting value is 1/sqrt(2)
		blockRateDelay,
	)
	if err != nil {
		// we check in a unit test that this does not happen
		panic("Default config is not compliant with timeout Config requirements")
	}

	return conf
}

// NewConfig creates a new TimoutConfig.
// startReplicaTimeout: starting timeout value for replica round [Milliseconds];
// minReplicaTimeout: minimal timeout value for replica round [Milliseconds];
// voteAggregationTimeoutFraction: fraction of replicaTimeout which is reserved for aggregating votes;
// timeoutIncrease: multiplicative factor for increasing timeout;
// timeoutDecrease: linear subtrahend for timeout decrease [Milliseconds]
// blockRateDelay: a delay to delay the proposal broadcasting
func NewConfig(
	startReplicaTimeout time.Duration,
	minReplicaTimeout time.Duration,
	voteAggregationTimeoutFraction float64,
	timeoutIncrease float64,
	timeoutDecrease float64,
	blockRateDelay time.Duration,
) (Config, error) {
	if startReplicaTimeout < minReplicaTimeout {
		return Config{}, model.NewConfigurationErrorf("startReplicaTimeout (%dms) cannot be smaller than minReplicaTimeout (%dms)",
			startReplicaTimeout.Milliseconds(), minReplicaTimeout.Milliseconds())
	}
	if minReplicaTimeout < 0 {
		return Config{}, model.NewConfigurationErrorf("minReplicaTimeout must non-negative")
	}
	if voteAggregationTimeoutFraction <= 0 || 1 < voteAggregationTimeoutFraction {
		return Config{}, model.NewConfigurationErrorf("VoteAggregationTimeoutFraction must be in range (0,1]")
	}
	if timeoutIncrease <= 1 {
		return Config{}, model.NewConfigurationErrorf("TimeoutIncrease must be strictly bigger than 1")
	}
	if timeoutDecrease <= 0 || 1 <= timeoutDecrease {
		return Config{}, model.NewConfigurationErrorf("timeoutDecrease must be in range (0,1)")
	}
	if err := validBlockRateDelay(blockRateDelay); err != nil {
		return Config{}, err
	}

	tc := Config{
		ReplicaTimeout:                 float64(startReplicaTimeout.Milliseconds()),
		MinReplicaTimeout:              float64(minReplicaTimeout.Milliseconds()),
		VoteAggregationTimeoutFraction: voteAggregationTimeoutFraction,
		TimeoutIncrease:                timeoutIncrease,
		TimeoutDecrease:                timeoutDecrease,
		BlockRateDelayMS:               atomic.NewFloat64(float64(blockRateDelay.Milliseconds())),
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

// StandardVoteAggregationTimeoutFraction calculates a standard value for the VoteAggregationTimeoutFraction in case a block delay is used.
// The motivation for the standard value is as follows:
//   - the next primary receives the block it ideally would extend at some time t
//   - the best guess the primary has, when other nodes would receive the block is at time t as well
//   - the primary needs to get its block to the other replicas, before they time out:
//     the primary uses its own timeout as estimator for the other replicas' timeout
func StandardVoteAggregationTimeoutFraction(minReplicaTimeout time.Duration, blockRateDelay time.Duration) float64 {
	standardVoteAggregationTimeoutFraction := 0.5
	minReplicaTimeoutMS := float64(minReplicaTimeout.Milliseconds())
	blockRateDelayMS := float64(blockRateDelay.Milliseconds())
	return (standardVoteAggregationTimeoutFraction*minReplicaTimeoutMS + blockRateDelayMS) / (minReplicaTimeoutMS + blockRateDelayMS)
}

// StandardTimeoutDecreaseFactor calculates a standard value for TimeoutDecreaseFactor
// for an assumed max fraction of offline (byzantine) HotStuff committee members
func StandardTimeoutDecreaseFactor(maxFractionOfflineReplicas, timeoutIncreaseFactor float64) float64 {
	return math.Pow(timeoutIncreaseFactor, maxFractionOfflineReplicas/(maxFractionOfflineReplicas-1))
}
