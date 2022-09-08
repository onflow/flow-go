package timeout

import (
	"math"
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
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
	// MaxReplicaTimeout is the maximum value the timeout can increase to [MILLISECONDS]
	MaxReplicaTimeout float64
	// TimeoutIncrease: MULTIPLICATIVE factor for increasing timeout when view
	// change was triggered by a TC (unhappy path).
	TimeoutIncrease float64
	// TimeoutDecrease: MULTIPLICATIVE factor for decreasing timeout when view
	// change was triggered by observing a QC from the previous view (happy path).
	TimeoutDecrease float64
	// BlockRateDelayMS is a delay to broadcast the proposal in order to control block production rate [MILLISECONDS]
	BlockRateDelayMS float64
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
	maxReplicaTimeout := 150 * time.Second
	timeoutIncreaseFactor := 2.0
	blockRateDelay := 0 * time.Millisecond

	// the following demonstrates the computation of standard values
	conf, err := NewConfig(
		replicaTimeout,
		minReplicaTimeout+blockRateDelay,
		maxReplicaTimeout,
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
//   - startReplicaTimeout: starting timeout value for replica round [Milliseconds]
//     Consistency requirement: `startReplicaTimeout` cannot be smaller than `minReplicaTimeout`
//   - minReplicaTimeout: minimal timeout value for replica round [Milliseconds]
//     Consistency requirement: must be non-negative
//   - maxReplicaTimeout: maximal timeout value for replica round [Milliseconds]
//     Consistency requirement: must be non-negative and larger than minReplicaTimeout
//   - timeoutIncrease: multiplicative factor for increasing timeout
//     Consistency requirement: must be strictly larger than 1
//   - timeoutDecrease: multiplicative factor for timeout decrease
//     Consistency requirement: must be in open interval (0,1); boundary values not allowed
//   - blockRateDelay: a delay to delay the proposal broadcasting [Milliseconds]
//
// Returns `model.ConfigurationError` is any of the consistency requirements is violated.
func NewConfig(
	startReplicaTimeout time.Duration,
	minReplicaTimeout time.Duration,
	maxReplicaTimeout time.Duration,
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
	if maxReplicaTimeout < minReplicaTimeout {
		return Config{}, model.NewConfigurationErrorf("maxReplicaTimeout must be larger than minReplicaTimeout")
	}
	if timeoutIncrease <= 1 {
		return Config{}, model.NewConfigurationErrorf("TimeoutIncrease must be strictly bigger than 1")
	}
	if timeoutDecrease <= 0 || 1 <= timeoutDecrease {
		return Config{}, model.NewConfigurationErrorf("timeoutDecrease must be in range (0,1)")
	}
	if blockRateDelay < 0 {
		return Config{}, model.NewConfigurationErrorf("blockRateDelay must be must be non-negative")
	}

	tc := Config{
		ReplicaTimeout:    float64(startReplicaTimeout.Milliseconds()),
		MinReplicaTimeout: float64(minReplicaTimeout.Milliseconds()),
		MaxReplicaTimeout: float64(maxReplicaTimeout.Milliseconds()),
		TimeoutIncrease:   timeoutIncrease,
		TimeoutDecrease:   timeoutDecrease,
		BlockRateDelayMS:  float64(blockRateDelay.Milliseconds()),
	}
	return tc, nil
}

// StandardTimeoutDecreaseFactor calculates a standard value for TimeoutDecreaseFactor
// for an assumed max fraction of offline (byzantine) HotStuff committee members
func StandardTimeoutDecreaseFactor(maxFractionOfflineReplicas, timeoutIncreaseFactor float64) float64 {
	return math.Pow(timeoutIncreaseFactor, maxFractionOfflineReplicas/(maxFractionOfflineReplicas-1))
}
