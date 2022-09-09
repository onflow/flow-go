package timeout

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// Config contains the configuration parameters for Truncated Exponential Backoff.
// timeout.Controller
// - on timeout: increase timeout by multiplicative factor `timeoutIncrease` (user-specified)
//   this results in exponential growing timeout duration on multiple subsequent timeouts
// - on progress: MULTIPLICATIVE timeout decrease
type Config struct {
	// MinReplicaTimeout is the minimum the timeout can decrease to [MILLISECONDS]
	MinReplicaTimeout float64
	// MaxReplicaTimeout is the maximum value the timeout can increase to [MILLISECONDS]
	MaxReplicaTimeout float64
	// TimeoutIncrease: MULTIPLICATIVE factor for increasing timeout when view
	// change was triggered by a TC (unhappy path).
	TimeoutIncrease float64
	// HappyPathRounds is the number of rounds without progress where we still consider being
	// on hot path of execution. After exceeding this value we will start increasing timeout values.
	HappyPathRounds uint64
	// BlockRateDelayMS is a delay to broadcast the proposal in order to control block production rate [MILLISECONDS]
	BlockRateDelayMS float64
}

var DefaultConfig = NewDefaultConfig()

// NewDefaultConfig returns a default timeout configuration.
// We explicitly provide a method here, which demonstrates in-code how
// to compute standard values from some basic quantities.
func NewDefaultConfig() Config {
	// the lower bound on the replicaTimeout value, this is also the initial timeout with what replicas
	// will start their execution.
	// If HotStuff is running at full speed, 1200ms should be enough. However, we add some buffer.
	// This value is for instant message delivery.
	minReplicaTimeout := 3 * time.Second
	maxReplicaTimeout := 1 * time.Minute
	timeoutIncreaseFactor := 1.2
	// we consider that after 6 rounds we are not on hot path anymore, and we need to start increasing timeouts
	happyPathRounds := uint64(6)
	blockRateDelay := 0 * time.Millisecond

	// the following demonstrates the computation of standard values
	conf, err := NewConfig(
		minReplicaTimeout+blockRateDelay,
		maxReplicaTimeout,
		timeoutIncreaseFactor,
		happyPathRounds,
		blockRateDelay,
	)
	if err != nil {
		// we check in a unit test that this does not happen
		panic("Default config is not compliant with timeout Config requirements")
	}

	return conf
}

// NewConfig creates a new TimoutConfig.
//  * minReplicaTimeout: minimal timeout value for replica round [Milliseconds]
//    Consistency requirement: must be non-negative
//  * maxReplicaTimeout: maximal timeout value for replica round [Milliseconds]
//    Consistency requirement: must be non-negative and larger than minReplicaTimeout
//  * timeoutIncrease: multiplicative factor for increasing timeout
//    Consistency requirement: must be strictly larger than 1
//  * happyPathRounds: number of failed rounds after which we will increase timeout
//  * blockRateDelay: a delay to delay the proposal broadcasting [Milliseconds]
// Returns `model.ConfigurationError` is any of the consistency requirements is violated.
func NewConfig(
	minReplicaTimeout time.Duration,
	maxReplicaTimeout time.Duration,
	timeoutIncrease float64,
	happyPathRounds uint64,
	blockRateDelay time.Duration,
) (Config, error) {
	if minReplicaTimeout < 0 {
		return Config{}, model.NewConfigurationErrorf("minReplicaTimeout must non-negative")
	}
	if maxReplicaTimeout < minReplicaTimeout {
		return Config{}, model.NewConfigurationErrorf("maxReplicaTimeout must be larger than minReplicaTimeout")
	}
	if timeoutIncrease <= 1 {
		return Config{}, model.NewConfigurationErrorf("TimeoutIncrease must be strictly bigger than 1")
	}
	if blockRateDelay < 0 {
		return Config{}, model.NewConfigurationErrorf("blockRateDelay must be must be non-negative")
	}

	tc := Config{
		MinReplicaTimeout: float64(minReplicaTimeout.Milliseconds()),
		MaxReplicaTimeout: float64(maxReplicaTimeout.Milliseconds()),
		TimeoutIncrease:   timeoutIncrease,
		HappyPathRounds:   happyPathRounds,
		BlockRateDelayMS:  float64(blockRateDelay.Milliseconds()),
	}
	return tc, nil
}
