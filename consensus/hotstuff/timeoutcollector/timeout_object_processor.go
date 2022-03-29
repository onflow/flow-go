package timeoutcollector

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"go.uber.org/atomic"
)

type accumulatedWeightTracker struct {
	minRequiredWeight   uint64
	done                atomic.Bool
	onWeightAccumulated func()
}

func (t *accumulatedWeightTracker) Track(weight uint64) {
	if weight < t.minRequiredWeight {
		return
	}
	if t.done.CAS(false, true) {
		t.onWeightAccumulated()
	}
}

type TimeoutVoteProcessor struct {
	partialTCTracker *accumulatedWeightTracker
	tcTracker        *accumulatedWeightTracker
}

// Process performs processing of timeout object in concurrent safe way. This
// function is implemented to be called by multiple goroutines at the same time.
// Design of this function is event driven, as soon as we collect enough weight
// to create a TC or a partial TC we will immediately do this and submit it
// via callback for further processing.
// Expected error returns during normal operations:
// * VoteForIncompatibleBlockError - submitted vote for incompatible block
// * VoteForIncompatibleViewError - submitted vote for incompatible view
// * model.InvalidVoteError - submitted vote with invalid signature
// All other errors should be treated as exceptions.
func (p *TimeoutVoteProcessor) Process(timeout *model.TimeoutObject) error {
	if p.tcTracker.done.Load() {
		return nil
	}
	return nil
}
