package timeoutcollector

import (
	"errors"
	"fmt"
	"sync"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
)

// accumulatedWeightTracker tracks one-time event of reaching required weight
// Uses atomic flag to guarantee concurrency safety.
type accumulatedWeightTracker struct {
	minRequiredWeight uint64
	done              atomic.Bool
}

func (t *accumulatedWeightTracker) Done() bool {
	return t.done.Load()
}

// Track checks if required threshold was reached as one-time event and
// returns true whenever it's reached.
func (t *accumulatedWeightTracker) Track(weight uint64) bool {
	if weight < t.minRequiredWeight {
		return false
	}
	if t.done.CAS(false, true) {
		return true
	}
	return false
}

// NewestQCTracker is a helper structure which keeps track of the highest QC(by view)
// in concurrency safe way.
type NewestQCTracker struct {
	lock     sync.RWMutex
	newestQC *flow.QuorumCertificate
}

// Track updates local state of NewestQC if the provided instance is newer(by view)
// Concurrently safe
func (t *NewestQCTracker) Track(qc *flow.QuorumCertificate) {
	NewestQC := t.NewestQC()
	if NewestQC != nil && NewestQC.View >= qc.View {
		return
	}

	t.lock.Lock()
	defer t.lock.Unlock()
	if t.newestQC == nil || t.newestQC.View < qc.View {
		t.newestQC = qc
	}
}

// NewestQC returns the newest QC(by view) tracked.
// Concurrently safe
func (t *NewestQCTracker) NewestQC() *flow.QuorumCertificate {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.newestQC
}

// TimeoutProcessor implements the hotstuff.TimeoutProcessor interface.
// It processes timeout objects broadcast by other replicas of consensus committee.
// TimeoutProcessor collects TOs for one view, eventually when enough timeout objects are contributed
// TimeoutProcessor will create a timeout certificate which can be used to advance round.
// Concurrency safe.
type TimeoutProcessor struct {
	view               uint64
	validator          hotstuff.Validator
	committee          hotstuff.Replicas
	sigAggregator      hotstuff.TimeoutSignatureAggregator
	onPartialTCCreated hotstuff.OnPartialTCCreated
	onTCCreated        hotstuff.OnTCCreated
	partialTCTracker   accumulatedWeightTracker
	tcTracker          accumulatedWeightTracker
	NewestQCTracker    NewestQCTracker
}

var _ hotstuff.TimeoutProcessor = (*TimeoutProcessor)(nil)

// NewTimeoutProcessor creates new instance of TimeoutProcessor
// Returns the following expected errors for invalid inputs:
//   * model.ErrViewForUnknownEpoch if no epoch containing the given view is known
// All other errors should be treated as exceptions.
func NewTimeoutProcessor(committee hotstuff.Replicas,
	validator hotstuff.Validator,
	sigAggregator hotstuff.TimeoutSignatureAggregator,
	onPartialTCCreated hotstuff.OnPartialTCCreated,
	onTCCreated hotstuff.OnTCCreated,
) (*TimeoutProcessor, error) {
	view := sigAggregator.View()
	qcThreshold, err := committee.QuorumThresholdForView(view)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve QC weight threshold for view %d: %w", view, err)
	}
	timeoutThreshold, err := committee.TimeoutThresholdForView(view)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve timeout weight threshold for view %d: %w", view, err)
	}
	return &TimeoutProcessor{
		view:      view,
		committee: committee,
		validator: validator,
		partialTCTracker: accumulatedWeightTracker{
			minRequiredWeight: timeoutThreshold,
			done:              *atomic.NewBool(false),
		},
		tcTracker: accumulatedWeightTracker{
			minRequiredWeight: qcThreshold,
			done:              *atomic.NewBool(false),
		},
		onPartialTCCreated: onPartialTCCreated,
		onTCCreated:        onTCCreated,
		sigAggregator:      sigAggregator,
	}, nil
}

// Process performs processing of timeout object in concurrent safe way. This
// function is implemented to be called by multiple goroutines at the same time.
// Design of this function is event driven, as soon as we collect enough weight
// to create a TC or a partial TC we will immediately do this and submit it
// via callback for further processing.
// Expected error returns during normal operations:
// * ErrTimeoutForIncompatibleView - submitted timeout for incompatible view
// * model.InvalidTimeoutError - submitted invalid timeout(invalid structure or invalid signature)
// * model.ErrViewForUnknownEpoch if no epoch containing the given view is known
// All other errors should be treated as exceptions.
func (p *TimeoutProcessor) Process(timeout *model.TimeoutObject) error {
	if p.view != timeout.View {
		return fmt.Errorf("received incompatible timeout, expected %d got %d: %w", p.view, timeout.View, ErrTimeoutForIncompatibleView)
	}

	if p.tcTracker.Done() {
		return nil
	}

	err := p.validateTimeout(timeout)
	if err != nil {
		return fmt.Errorf("received invalid timeout: %w", err)
	}

	totalWeight, err := p.sigAggregator.VerifyAndAdd(timeout.SignerID, timeout.SigData, timeout.NewestQC.View)
	if err != nil {
		return fmt.Errorf("could not process invalid signature: %w", err)
	}

	p.NewestQCTracker.Track(timeout.NewestQC)

	if p.partialTCTracker.Track(totalWeight) {
		p.onPartialTCCreated(p.view)
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

// validateTimeout performs validation of timeout object, verifies if timeout is correctly structured
// and included QC and TC is correctly structured and signed.
// ATTENTION: this function doesn't check if timeout signature is valid, this check happens in signature aggregator
// Expected error returns during normal operations:
// * model.InvalidTimeoutError - submitted invalid timeout(invalid structure or invalid signature)
// * model.ErrViewForUnknownEpoch if no epoch containing the given view is known
// All other errors should be treated as exceptions.
func (p *TimeoutProcessor) validateTimeout(timeout *model.TimeoutObject) error {
	// 1. check if it's correctly structured
	// (a) Every TO must contain a QC
	if timeout.NewestQC == nil {
		return model.NewInvalidTimeoutErrorf(timeout, "TimeoutObject without QC is invalid")
	}

	if timeout.View < timeout.NewestQC.View {
		return model.NewInvalidTimeoutErrorf(timeout, "TO's QC %d cannot be newer than the TO's view %d",
			timeout.NewestQC.View, timeout.View)
	}

	// (b) If a TC is included, the TC must be for the past round, no matter whether a QC
	//     for the last round is also included. In some edge cases, a node might observe
	//     _both_ QC and TC for the previous round, in which case it can include both.
	if timeout.LastViewTC != nil {
		if timeout.View != timeout.LastViewTC.View+1 {
			return model.NewInvalidTimeoutErrorf(timeout, "invalid TC for previous round")
		}
		if timeout.NewestQC.View < timeout.LastViewTC.NewestQC.View {
			return model.NewInvalidTimeoutErrorf(timeout, "timeout.NewestQC has older view that the QC in timeout.LastViewTC")
		}
	}
	// (c) The TO must contain a proof that sender legitimately entered timeout.View. Transitioning
	//     to round timeout.View is possible either by observing a QC or a TC for the previous round.
	//     If no QC is included, we require a TC to be present, which by check (1b) must be for
	//     the previous round.
	lastViewSuccessful := timeout.View == timeout.NewestQC.View+1
	if !lastViewSuccessful {
		// The TO's sender did _not_ observe a QC for round timeout.View-1. Hence, it should
		// include a TC for the previous round. Otherwise, the TO is invalid.
		if timeout.LastViewTC == nil {
			return model.NewInvalidTimeoutErrorf(timeout, "timeout must include TC")
		}
	}

	// 2. Check fi signer identity is valid
	_, err := p.committee.IdentityByEpoch(timeout.View, timeout.SignerID)
	if model.IsInvalidSignerError(err) {
		return model.NewInvalidTimeoutErrorf(timeout, "invalid signer for timeout: %w", err)
	}
	if err != nil {
		return fmt.Errorf("error retrieving signer Identity at view %d: %w", timeout.View, err)
	}

	// 3. Check if QC is valid
	err = p.validator.ValidateQC(timeout.NewestQC)
	if err != nil {
		if model.IsInvalidQCError(err) {
			return model.NewInvalidTimeoutErrorf(timeout, "included QC is invalid: %w", err)
		}
		if errors.Is(err, model.ErrViewForUnknownEpoch) {
			// We require each replica to be bootstrapped with a QC pointing to a finalized block. Therefore, we should know the
			// Epoch for any QC.View and TC.View we encounter. Receiving a `model.ErrViewForUnknownEpoch` is conceptually impossible,
			// i.e. a symptom of an internal bug or invalid bootstrapping information.
			return fmt.Errorf("no Epoch information availalbe for QC that was included in TO; symptom of internal bug or invalid bootstrapping information: %s", err.Error())
		}
		return fmt.Errorf("unexpected error when validating QC: %w", err)
	}

	// 4. If TC is included, it must be valid
	if timeout.LastViewTC != nil {
		err = p.validator.ValidateTC(timeout.LastViewTC)
		if err != nil {
			if model.IsInvalidTCError(err) {
				return model.NewInvalidTimeoutErrorf(timeout, "included TC is invalid: %w", err)
			}
			if errors.Is(err, model.ErrViewForUnknownEpoch) {
				// We require each replica to be bootstrapped with a QC pointing to a finalized block. Therefore, we should know the
				// Epoch for any QC.View and TC.View we encounter. Receiving a `model.ErrViewForUnknownEpoch` is conceptually impossible,
				// i.e. a symptom of an internal bug or invalid bootstrapping information.
				return fmt.Errorf("no Epoch information availalbe for TC that was included in TO; symptom of internal bug or invalid bootstrapping information: %s", err.Error())
			}
			return fmt.Errorf("unexpected error when validating TC: %w", err)
		}
	}
	return nil

}

// buildTC performs aggregation of signatures when we have collected enough
// weight for building TC. This function is run only once by single worker.
// Any error should be treated as exception.
func (p *TimeoutProcessor) buildTC() (*flow.TimeoutCertificate, error) {
	signers, highQCViews, aggregatedSig, err := p.sigAggregator.Aggregate()
	if err != nil {
		return nil, fmt.Errorf("could not aggregate multi message signature: %w", err)
	}

	signerIndices, err := p.signerIndicesFromIdentities(signers)
	if err != nil {
		return nil, fmt.Errorf("could not encode signer indices: %w", err)
	}

	return &flow.TimeoutCertificate{
		View:          p.view,
		NewestQCViews: highQCViews,
		NewestQC:      p.NewestQCTracker.NewestQC(),
		SignerIndices: signerIndices,
		SigData:       aggregatedSig,
	}, nil
}

// signerIndicesFromIdentities encodes identities into signer indices.
// Any error should be treated as exception.
func (p *TimeoutProcessor) signerIndicesFromIdentities(signerIDs flow.IdentifierList) ([]byte, error) {
	allIdentities, err := p.committee.IdentitiesByEpoch(p.view)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve identities for view %d: %w", p.view, err)
	}
	signerIndices, err := signature.EncodeSignersToIndices(allIdentities.NodeIDs(), signerIDs)
	if err != nil {
		return nil, fmt.Errorf("could not encode signer identifiers to indices: %w", err)
	}
	return signerIndices, nil
}
