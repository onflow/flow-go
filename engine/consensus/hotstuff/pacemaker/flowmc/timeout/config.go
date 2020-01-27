package timeout

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// TimoutConfig contains the configuration parameters for ExponentialIncrease-LinearDecrease
// timeout.Controller
// - on timeout: increase timeout by multiplicative factor `timeoutIncrease` (user-specified)
//   this results in exponential growing timeout duration on multiple subsequent timeouts
// - on progress: decrease timeout by subtrahend `timeoutDecrease`
type Config struct {
	// replicaTimeout is the duration of a view before we time out [Milliseconds]
	// replicaTimeout is the only variable quantity
	replicaTimeout float64

	// replicaTimeout is the duration of a view before we time out [Milliseconds]
	minReplicaTimeout float64
	// voteAggregationTimeoutFraction is a fraction of replicaTimeout which is reserved for aggregating votes
	voteAggregationTimeoutFraction float64
	// timeoutDecrease: multiplicative factor for increasing timeout
	timeoutIncrease float64
	// timeoutDecrease linear subtrahend for timeout decrease [Milliseconds]
	timeoutDecrease float64
}

var DefaultConfig = Config{
	replicaTimeout:                 1200,
	minReplicaTimeout:              1200,
	voteAggregationTimeoutFraction: 0.5,
	timeoutIncrease:                1.5,
	timeoutDecrease:                800,
}

// NewTimoutConfig creates a new TimoutConfig.
// startReplicaTimeout: starting timeout value for replica round [Milliseconds];
// minReplicaTimeout: minimal timeout value for replica round [Milliseconds];
// voteAggregationTimeoutFraction: fraction of replicaTimeout which is reserved for aggregating votes;
// timeoutIncrease: multiplicative factor for increasing timeout;
// timeoutDecrease: linear subtrahend for timeout decrease [Milliseconds]
func NewConfig(startReplicaTimeout, minReplicaTimeout, voteAggregationTimeoutFraction, timeoutIncrease, timeoutDecrease float64) (*Config, error) {
	if startReplicaTimeout < minReplicaTimeout {
		msg := fmt.Sprintf("startReplicaTimeout (%f) cannot be smaller than minReplicaTimeout (%f)",
			startReplicaTimeout, minReplicaTimeout)
		return nil, &types.ErrorConfiguration{Msg: msg}
	}
	if minReplicaTimeout < 0 {
		return nil, &types.ErrorConfiguration{Msg: "minReplicaTimeout must non-negative"}
	}
	if voteAggregationTimeoutFraction <= 0 || 1 < voteAggregationTimeoutFraction {
		return nil, &types.ErrorConfiguration{Msg: "VoteAggregationTimeoutFraction must be in range (0,1]"}
	}
	if timeoutIncrease <= 1 {
		return nil, &types.ErrorConfiguration{Msg: "TimeoutIncrease must be strictly bigger than 1"}
	}
	if timeoutDecrease <= 0 {
		return nil, &types.ErrorConfiguration{Msg: "timeoutDecrease must positive"}
	}
	tc := Config{
		replicaTimeout:                 startReplicaTimeout,
		minReplicaTimeout:              minReplicaTimeout,
		voteAggregationTimeoutFraction: voteAggregationTimeoutFraction,
		timeoutIncrease:                timeoutIncrease,
		timeoutDecrease:                timeoutDecrease,
	}
	return &tc, nil
}
