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
type config struct {
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

var DefaultConfig = config{
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
func NewConfig(
	startReplicaTimeout time.Duration,
	minReplicaTimeout time.Duration,
	voteAggregationTimeoutFraction float64,
	timeoutIncrease float64,
	timeoutDecrease time.Duration,
) (config, error) {
	if startReplicaTimeout < minReplicaTimeout {
		msg := fmt.Sprintf(
			"startReplicaTimeout (%dms) cannot be smaller than minReplicaTimeout (%dms)",
			startReplicaTimeout.Milliseconds(), minReplicaTimeout.Milliseconds())
		return config{}, &model.ErrorConfiguration{Msg: msg}
	}
	if minReplicaTimeout < 0 {
		return config{}, &model.ErrorConfiguration{Msg: "minReplicaTimeout must non-negative"}
	}
	if voteAggregationTimeoutFraction <= 0 || 1 < voteAggregationTimeoutFraction {
		return config{}, &model.ErrorConfiguration{Msg: "VoteAggregationTimeoutFraction must be in range (0,1]"}
	}
	if timeoutIncrease <= 1 {
		return config{}, &model.ErrorConfiguration{Msg: "TimeoutIncrease must be strictly bigger than 1"}
	}
	if timeoutDecrease <= 0 {
		return config{}, &model.ErrorConfiguration{Msg: "timeoutDecrease must positive"}
	}
	tc := config{
		replicaTimeout:                 float64(startReplicaTimeout.Milliseconds()),
		minReplicaTimeout:              float64(minReplicaTimeout.Milliseconds()),
		voteAggregationTimeoutFraction: voteAggregationTimeoutFraction,
		timeoutIncrease:                timeoutIncrease,
		timeoutDecrease:                float64(timeoutDecrease.Milliseconds()),
	}
	return tc, nil
}
