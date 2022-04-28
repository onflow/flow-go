package timeoutcollector

import (
	"fmt"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"go.uber.org/atomic"
)

type accumulatedWeightTracker struct {
	minRequiredWeight uint64
	done              atomic.Bool
}

func (t *accumulatedWeightTracker) Done() bool {
	return t.done.Load()
}

func (t *accumulatedWeightTracker) Track(weight uint64) bool {
	if weight < t.minRequiredWeight {
		return false
	}
	if t.done.CAS(false, true) {
		return true
	}
	return false
}

type TimeoutProcessor struct {
	view             uint64
	validator        hotstuff.Validator
	partialTCTracker accumulatedWeightTracker
	tcTracker        accumulatedWeightTracker
	//sigAggregator         *TimeoutSignatureAggregator
	onPartialTCCreated hotstuff.OnPartialTCCreated
	onTCCreated        hotstuff.OnTCCreated
}

var _ hotstuff.TimeoutProcessor = (*TimeoutProcessor)(nil)

func NewTimeoutProcessor(view uint64,
	totalWeight uint64,
	onPartialTCCreated hotstuff.OnPartialTCCreated,
	onTCCreated hotstuff.OnTCCreated,
) *TimeoutProcessor {
	return &TimeoutProcessor{
		partialTCTracker: accumulatedWeightTracker{
			minRequiredWeight: hotstuff.ComputeWeightThresholdForHonestMajority(totalWeight),
			done:              *atomic.NewBool(false),
		},
		tcTracker: accumulatedWeightTracker{
			minRequiredWeight: hotstuff.ComputeWeightThresholdForBuildingQC(totalWeight),
			done:              *atomic.NewBool(false),
		},
		onPartialTCCreated: onPartialTCCreated,
		onTCCreated:        onTCCreated,
	}
}

// Process performs processing of timeout object in concurrent safe way. This
// function is implemented to be called by multiple goroutines at the same time.
// Design of this function is event driven, as soon as we collect enough weight
// to create a TC or a partial TC we will immediately do this and submit it
// via callback for further processing.
// Expected error returns during normal operations:
// * hotstuff.TimeoutForIncompatibleViewError - submitted timeout for incompatible view
// * model.InvalidTimeoutError - submitted invalid timeout(invalid structure or invalid signature)
// All other errors should be treated as exceptions.
func (p *TimeoutProcessor) Process(timeout *model.TimeoutObject) error {
	if p.view != timeout.View {
		return fmt.Errorf("received incompatible timeout, expected %d got %d", p.view, timeout.View)
	}

	if p.tcTracker.Done() {
		return nil
	}

	err := p.validateTimeout(timeout)
	if err != nil {
		// handle error
	}

	totalWeight := uint64(0)
	//totalWeight, err := p.sigAggregator.TrustedAdd(timeout.SignerID, timeout.SigData, msg)
	//if err != nil {
	//	// handle error
	//}

	if p.partialTCTracker.Track(totalWeight) {
		//p.onPartialTCCreated(p.tcBuilder.View())
	}

	// checking of conditions for building TC are satisfied
	// At this point, we have enough signatures to build a TC. Another routine
	// might just be at this point. To avoid duplicate work, only one routine can pass:
	if !p.tcTracker.Track(totalWeight) {
		return nil
	}

	tc, err := p.buildTC()
	if err != nil {
		return fmt.Errorf("internal error constructing TC: %w", err)
	}
	p.onTCCreated(tc)

	return nil
}

func (p *TimeoutProcessor) validateTimeout(timeout *model.TimeoutObject) error {
	panic("implement me")
}

func (p *TimeoutProcessor) buildTC() (*flow.TimeoutCertificate, error) {
	panic("implement me")
	//signers, aggregatedSig, err := p.sigAggregator.UnsafeAggregate()
	//if err != nil {
	//	return nil, fmt.Errorf("could not aggregate multi message signature: %w", err)
	//}
	//
	//tc := p.tcBuilder.Build(signers, aggregatedSig)
	//err = p.validator.ValidateTC(tc)
	//if err != nil {
	//	return nil, fmt.Errorf("constructed TC is invalid: %w", err)
	//}

	//return tc, nil
}
