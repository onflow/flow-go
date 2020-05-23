package timeout

import (
	"fmt"
	"time"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
)

// Config contains the configuration parameters for ExponentialIncrease-LinearDecrease
// timeout.Controller
// - on timeout: increase timeout by multiplicative factor `timeoutIncrease` (user-specified)
//   this results in exponential growing timeout duration on multiple subsequent timeouts
// - on progress: decrease timeout by subtrahend `timeoutDecrease`
type Config struct {
	// ReplicaTimeout is the duration of a view before we time out [Milliseconds]
	// ReplicaTimeout is the only variable quantity
	ReplicaTimeout float64

	// ReplicaTimeout is the duration of a view before we time out [Milliseconds]
	MinReplicaTimeout float64
	// VoteAggregationTimeoutFraction is a fraction of replicaTimeout which is reserved for aggregating votes
	VoteAggregationTimeoutFraction float64
	// TimeoutDecrease: multiplicative factor for increasing timeout
	TimeoutIncrease float64
	// TimeoutDecrease linear subtrahend for timeout decrease [Milliseconds]
	TimeoutDecrease float64
	// BlockRateDelayMS is a delay to broadcast the proposal in order to control block production rate [Milliseconds]
	BlockRateDelayMS float64
}

var DefaultConfig = Config{
	// the time for replica to wait for the proposal for the current view.
	// If hotstuff is running at full speed, 1200ms is enough, however, since a 1 second delay is added
	// to cap the block production rate, it is also added to the timeout.
	ReplicaTimeout: 1200,
	// the next timeout will increase when timeout was hit, and decrease if wasn't hit.
	// MinReplicaTimeout is the minimum the timeout can decrease to.
	MinReplicaTimeout: 1200,
	// The estimated time for the leader to receive majority votes after sending out its proposal is about
	// half of ReplicaTimeout
	VoteAggregationTimeoutFraction: 0.5,
	TimeoutIncrease:                1.5,
	TimeoutDecrease:                800,
	// no delay to broadcast a proposal
	BlockRateDelayMS: 0,
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
	timeoutDecrease time.Duration,
	blockRateDelay time.Duration,
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

	fraction := calcVoteAggregationTimeoutFraction(voteAggregationTimeoutFraction, startReplicaTimeout, blockRateDelay)

	tc := Config{
		ReplicaTimeout:                 float64(startReplicaTimeout.Milliseconds() + blockRateDelay.Milliseconds()),
		MinReplicaTimeout:              float64(minReplicaTimeout.Milliseconds() + blockRateDelay.Milliseconds()),
		VoteAggregationTimeoutFraction: fraction,
		TimeoutIncrease:                timeoutIncrease,
		TimeoutDecrease:                float64(timeoutDecrease.Milliseconds()),
		BlockRateDelayMS:               float64(blockRateDelay.Milliseconds()),
	}
	return tc, nil
}

func calcVoteAggregationTimeoutFraction(voteAggregationTimeoutFraction float64, startReplicaTimeout time.Duration, blockRateDelay time.Duration) float64 {
	return (voteAggregationTimeoutFraction*float64(startReplicaTimeout.Milliseconds()) + float64(blockRateDelay.Milliseconds())) /
		float64(startReplicaTimeout.Milliseconds()+blockRateDelay.Milliseconds())
}
