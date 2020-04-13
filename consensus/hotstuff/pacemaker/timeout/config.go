package timeout

import (
	"fmt"
	"time"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
)

// TimoutConfig contains the configuration parameters for ExponentialIncrease-LinearDecrease
// timeout.Controller
// - on timeout: increase timeout by multiplicative factor `timeoutIncrease` (user-specified)
//   this results in exponential growing timeout duration on multiple subsequent timeouts
// - on progress: decrease timeout by subtrahend `timeoutDecrease`
type Config struct {
	// replicaTimeout is the duration of a view before we time out [Milliseconds]
	// replicaTimeout is the only variable quantity
	ReplicaTimeout float64

	// replicaTimeout is the duration of a view before we time out [Milliseconds]
	MinReplicaTimeout float64
	// voteAggregationTimeoutFraction is a fraction of replicaTimeout which is reserved for aggregating votes
	VoteAggregationTimeoutFraction float64
	// timeoutDecrease: multiplicative factor for increasing timeout
	TimeoutIncrease float64
	// timeoutDecrease linear subtrahend for timeout decrease [Milliseconds]
	TimeoutDecrease float64
}

var DefaultConfig = Config{
	ReplicaTimeout:                 1200,
	MinReplicaTimeout:              1200,
	VoteAggregationTimeoutFraction: 0.5,
	TimeoutIncrease:                1.5,
	TimeoutDecrease:                800,
}

// NewTimoutConfig creates a new TimoutConfig.
// startReplicaTimeout: starting timeout value for replica round [Milliseconds];
// minReplicaTimeout: minimal timeout value for replica round [Milliseconds];
// voteAggregationTimeoutFraction: fraction of replicaTimeout which is reserved for aggregating votes;
// timeoutIncrease: multiplicative factor for increasing timeout;
// timeoutDecrease: linear subtrahend for timeout decrease [Milliseconds]
func NewConfig(
	startReplicaTimeout time.Duration,
	minReplicaTimeout time.Duration,
	voteAggregationTimeoutFraction float64,
	timeoutIncrease float64,
	timeoutDecrease time.Duration,
) (Config, error) {
	if startReplicaTimeout < minReplicaTimeout {
		msg := fmt.Sprintf(
			"startReplicaTimeout (%dms) cannot be smaller than minReplicaTimeout (%dms)",
			startReplicaTimeout.Milliseconds(), minReplicaTimeout.Milliseconds())
		return Config{}, &model.ErrorConfiguration{Msg: msg}
	}
	if minReplicaTimeout < 0 {
		return Config{}, &model.ErrorConfiguration{Msg: "minReplicaTimeout must non-negative"}
	}
	if voteAggregationTimeoutFraction <= 0 || 1 < voteAggregationTimeoutFraction {
		return Config{}, &model.ErrorConfiguration{Msg: "VoteAggregationTimeoutFraction must be in range (0,1]"}
	}
	if timeoutIncrease <= 1 {
		return Config{}, &model.ErrorConfiguration{Msg: "TimeoutIncrease must be strictly bigger than 1"}
	}
	if timeoutDecrease <= 0 {
		return Config{}, &model.ErrorConfiguration{Msg: "timeoutDecrease must positive"}
	}
	tc := Config{
		ReplicaTimeout:                 float64(startReplicaTimeout.Milliseconds()),
		MinReplicaTimeout:              float64(minReplicaTimeout.Milliseconds()),
		VoteAggregationTimeoutFraction: voteAggregationTimeoutFraction,
		TimeoutIncrease:                timeoutIncrease,
		TimeoutDecrease:                float64(timeoutDecrease.Milliseconds()),
	}
	return tc, nil
}
