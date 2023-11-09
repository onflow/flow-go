package timeoutcollector

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/tracker"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
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

// Track returns true if `weight` reaches or exceeds `minRequiredWeight` for the _first time_.
// All subsequent calls of `Track` (with any value) return false.
func (t *accumulatedWeightTracker) Track(weight uint64) bool {
	if weight < t.minRequiredWeight {
		return false
	}
	return t.done.CompareAndSwap(false, true)
}

// TimeoutProcessor implements the hotstuff.TimeoutProcessor interface.
// It processes timeout objects broadcast by other replicas of the consensus committee.
// TimeoutProcessor collects TOs for one view, eventually when enough timeout objects are contributed
// TimeoutProcessor will create a timeout certificate which can be used to advance round.
// Concurrency safe.
type TimeoutProcessor struct {
	log              zerolog.Logger
	view             uint64
	validator        hotstuff.Validator
	committee        hotstuff.Replicas
	sigAggregator    hotstuff.TimeoutSignatureAggregator
	notifier         hotstuff.TimeoutCollectorConsumer
	partialTCTracker accumulatedWeightTracker
	tcTracker        accumulatedWeightTracker
	newestQCTracker  *tracker.NewestQCTracker
}

var _ hotstuff.TimeoutProcessor = (*TimeoutProcessor)(nil)

// NewTimeoutProcessor creates new instance of TimeoutProcessor
// Returns the following expected errors for invalid inputs:
//   - model.ErrViewForUnknownEpoch if no epoch containing the given view is known
//
// All other errors should be treated as exceptions.
func NewTimeoutProcessor(log zerolog.Logger,
	committee hotstuff.Replicas,
	validator hotstuff.Validator,
	sigAggregator hotstuff.TimeoutSignatureAggregator,
	notifier hotstuff.TimeoutCollectorConsumer,
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
		log: log.With().
			Str("component", "hotstuff.timeout_processor").
			Uint64("view", view).
			Logger(),
		view:      view,
		committee: committee,
		validator: validator,
		notifier:  notifier,
		partialTCTracker: accumulatedWeightTracker{
			minRequiredWeight: timeoutThreshold,
			done:              *atomic.NewBool(false),
		},
		tcTracker: accumulatedWeightTracker{
			minRequiredWeight: qcThreshold,
			done:              *atomic.NewBool(false),
		},
		sigAggregator:   sigAggregator,
		newestQCTracker: tracker.NewNewestQCTracker(),
	}, nil
}

// Process performs processing of timeout object in concurrent safe way. This
// function is implemented to be called by multiple goroutines at the same time.
// Design of this function is event driven, as soon as we collect enough weight
// to create a TC or a partial TC we will immediately do so and submit it
// via callback for further processing.
// Expected error returns during normal operations:
//   - ErrTimeoutForIncompatibleView - submitted timeout for incompatible view
//   - model.InvalidTimeoutError - submitted invalid timeout(invalid structure or invalid signature)
//   - model.DuplicatedSignerError if a timeout from the same signer was previously already added
//     It does _not necessarily_ imply that the timeout is invalid or the sender is equivocating.
//
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
		return fmt.Errorf("validating timeout failed: %w", err)
	}
	if p.tcTracker.Done() {
		return nil
	}

	// CAUTION: for correctness it is critical that we update the `newestQCTracker` first, _before_ we add the
	// TO's signature to `sigAggregator`. Reasoning:
	//  * For a valid TC, we require that the TC includes a QC with view ≥ max{TC.NewestQCViews}.
	//  * The `NewestQCViews` is maintained by `sigAggregator`.
	//  * Hence, for any view `v ∈ NewestQCViews` that `sigAggregator` knows, a QC with equal or larger view is
	//    known to `newestQCTracker`. This is guaranteed if and only if `newestQCTracker` is updated first.
	p.newestQCTracker.Track(timeout.NewestQC)

	totalWeight, err := p.sigAggregator.VerifyAndAdd(timeout.SignerID, timeout.SigData, timeout.NewestQC.View)
	if err != nil {
		if model.IsInvalidSignerError(err) {
			return model.NewInvalidTimeoutErrorf(timeout, "invalid signer for timeout: %w", err)
		}
		if errors.Is(err, model.ErrInvalidSignature) {
			return model.NewInvalidTimeoutErrorf(timeout, "timeout is from valid signer but has cryptographically invalid signature: %w", err)
		}
		// model.DuplicatedSignerError is an expected error and just bubbled up the call stack.
		// It does _not necessarily_ imply that the timeout is invalid or the sender is equivocating.
		return fmt.Errorf("adding signature to aggregator failed: %w", err)
	}
	p.log.Debug().Msgf("processed timeout, total weight=(%d), required=(%d)", totalWeight, p.tcTracker.minRequiredWeight)

	if p.partialTCTracker.Track(totalWeight) {
		p.notifier.OnPartialTcCreated(p.view, p.newestQCTracker.NewestQC(), timeout.LastViewTC)
	}

	// Checking of conditions for building TC are satisfied when willBuildTC is true.
	// At this point, we have enough signatures to build a TC. Another routine
	// might just be at this point. To avoid duplicate work, Track returns true only once.
	willBuildTC := p.tcTracker.Track(totalWeight)
	if !willBuildTC {
		// either we do not have enough timeouts to build a TC, or another thread
		// has already passed this gate and created a TC
		return nil
	}

	tc, err := p.buildTC()
	if err != nil {
		return fmt.Errorf("internal error constructing TC: %w", err)
	}
	p.notifier.OnTcConstructedFromTimeouts(tc)

	return nil
}

// validateTimeout performs validation of timeout object, verifies if timeout is correctly structured
// and included QC and TC is correctly structured and signed.
// ATTENTION: this function does _not_ check whether the TO's `SignerID` is an authorized node nor if
// the signature is valid. These checks happen in signature aggregator.
// Expected error returns during normal operations:
// * model.InvalidTimeoutError - submitted invalid timeout
// All other errors should be treated as exceptions.
func (p *TimeoutProcessor) validateTimeout(timeout *model.TimeoutObject) error {
	// 1. check if it's correctly structured
	// (a) Every TO must contain a QC
	if timeout.NewestQC == nil {
		return model.NewInvalidTimeoutErrorf(timeout, "TimeoutObject without QC is invalid")
	}

	if timeout.View <= timeout.NewestQC.View {
		return model.NewInvalidTimeoutErrorf(timeout, "TO's QC %d cannot be newer than the TO's view %d",
			timeout.NewestQC.View, timeout.View)
	}

	// (b) If a TC is included, the TC must be for the past round, no matter whether a QC
	//     for the last round is also included. In some edge cases, a node might observe
	//     _both_ QC and TC for the previous round, in which case it can include both.
	if timeout.LastViewTC != nil {
		if timeout.View != timeout.LastViewTC.View+1 {
			return model.NewInvalidTimeoutErrorf(timeout, "invalid TC for non-previous view, expected view %d, got view %d", timeout.View-1, timeout.LastViewTC.View)
		}
		if timeout.NewestQC.View < timeout.LastViewTC.NewestQC.View {
			return model.NewInvalidTimeoutErrorf(timeout, "timeout.NewestQC is older (view=%d) than the QC in timeout.LastViewTC (view=%d)", timeout.NewestQC.View, timeout.LastViewTC.NewestQC.View)
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

	// 2. Check if QC is valid
	err := p.validator.ValidateQC(timeout.NewestQC)
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

	// 3. If TC is included, it must be valid
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
	signersData, aggregatedSig, err := p.sigAggregator.Aggregate()
	if err != nil {
		return nil, fmt.Errorf("could not aggregate multi message signature: %w", err)
	}

	// IMPORTANT: To properly verify an aggregated signature included in TC we need to provide list of signers with corresponding
	// messages(`TimeoutCertificate.NewestQCViews`) for each signer. If the one-to-once correspondence of view and signer is not maintained,
	// it won't be possible to verify the aggregated signature.
	// Aggregate returns an unordered set of signers together with additional data.
	// Due to implementation specifics of signer indices, the decoding step results in canonically ordered signer ids, which means
	// we need to canonically order the respective `newestQCView`, so we can properly map signer to `newestQCView` after decoding.

	// sort data in canonical order
	slices.SortFunc(signersData, func(lhs, rhs hotstuff.TimeoutSignerInfo) int {
		return order.IdentifierCanonical(lhs.Signer, rhs.Signer)
	})

	// extract signers and data separately
	signers := make([]flow.Identifier, 0, len(signersData))
	newestQCViews := make([]uint64, 0, len(signersData))
	for _, data := range signersData {
		signers = append(signers, data.Signer)
		newestQCViews = append(newestQCViews, data.NewestQCView)
	}

	signerIndices, err := p.signerIndicesFromIdentities(signers)
	if err != nil {
		return nil, fmt.Errorf("could not encode signer indices: %w", err)
	}

	// Note that `newestQC` can have a larger view than any of the views included in `newestQCViews`.
	// This is because for a TO currently being processes following two operations are executed in separate steps:
	// * updating the `newestQCTracker` with the QC from the TO
	// * adding the TO's signature to `sigAggregator`
	// Therefore, races are possible, where the `newestQCTracker` already knows of a QC with larger view
	// than the data stored in `sigAggregator`.
	newestQC := p.newestQCTracker.NewestQC()

	return &flow.TimeoutCertificate{
		View:          p.view,
		NewestQCViews: newestQCViews,
		NewestQC:      newestQC,
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
