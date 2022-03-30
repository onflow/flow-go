package timeoutcollector

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"go.uber.org/atomic"
)

type accumulatedWeightTracker struct {
	minRequiredWeight   uint64
	done                atomic.Bool
	onWeightAccumulated func()
}

func (t *accumulatedWeightTracker) Track(weight uint64) bool {
	if weight < t.minRequiredWeight {
		return false
	}
	if t.done.CAS(false, true) {
		t.onWeightAccumulated()
		return true
	}
	return false
}

type TimeoutObjectProcessor struct {
	view             uint64
	highQCViews      map[uint64]struct{}
	highestQC        *flow.QuorumCertificate
	partialTCTracker accumulatedWeightTracker
	tcTracker        accumulatedWeightTracker
}

func NewTimeoutObjectProcessor(view uint64, totalWeight uint64, onPartialTCCreated hotstuff.OnPartialTCCreated) *TimeoutObjectProcessor {
	return &TimeoutObjectProcessor{
		partialTCTracker: accumulatedWeightTracker{
			minRequiredWeight:   hotstuff.ComputeWeightThresholdForHonestMajority(totalWeight),
			done:                *atomic.NewBool(false),
			onWeightAccumulated: func() { onPartialTCCreated(view) },
		},
		tcTracker: accumulatedWeightTracker{
			minRequiredWeight:   hotstuff.ComputeWeightThresholdForBuildingQC(totalWeight),
			done:                *atomic.NewBool(false),
			onWeightAccumulated: func() {},
		},
	}
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
func (p *TimeoutObjectProcessor) Process(timeout *model.TimeoutObject) error {
	if p.tcTracker.done.Load() {
		return nil
	}

	p.highQCViews[timeout.HighestQC.View] = struct{}{}
	if p.highestQC.View < timeout.HighestQC.View {
		p.highestQC = timeout.HighestQC
	}

	return nil
}

func (p *TimeoutObjectProcessor) buildTC() (*flow.TimeoutCertificate, error) {
	return &flow.TimeoutCertificate{
		View:          p.view,
		TOHighQCViews: nil,
		TOHighestQC:   nil,
		SignerIDs:     nil,
		SigData:       nil,
	}, nil
}
