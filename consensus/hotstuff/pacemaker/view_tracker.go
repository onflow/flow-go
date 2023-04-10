package pacemaker

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// viewTracker is a sub-component of the PaceMaker, which encapsulates the logic for tracking
// and updating the current view. For crash resilience, the viewTracker persists its latest
// internal state.
//
// In addition, viewTracker maintains and persists a proof to show that it entered the current
// view according to protocol rules. To enter a new view `v`, the Pacemaker must observe a
// valid QC or TC for view `v-1`. Per convention, the proof has the following structure:
//   - If the current view was entered by observing a QC, this QC is returned by `NewestQC()`.
//     Furthermore, `LastViewTC()` returns nil.
//   - If the current view was entered by observing a TC, `NewestQC()` returns the newest QC
//     known. `LastViewTC()` returns the TC that triggered the view change
type viewTracker struct {
	livenessData hotstuff.LivenessData
	persist      hotstuff.Persister
}

// newViewTracker instantiates a viewTracker.
func newViewTracker(persist hotstuff.Persister) (viewTracker, error) {
	livenessData, err := persist.GetLivenessData()
	if err != nil {
		return viewTracker{}, fmt.Errorf("could not load liveness data: %w", err)
	}

	if livenessData.CurrentView < 1 {
		return viewTracker{}, model.NewConfigurationErrorf("PaceMaker cannot start in view 0 (view zero is reserved for genesis block, which has no proposer)")
	}

	return viewTracker{
		livenessData: *livenessData,
		persist:      persist,
	}, nil
}

// CurView returns the current view.
func (vt *viewTracker) CurView() uint64 {
	return vt.livenessData.CurrentView
}

// NewestQC returns the QC with the highest view known.
func (vt *viewTracker) NewestQC() *flow.QuorumCertificate {
	return vt.livenessData.NewestQC
}

// LastViewTC returns TC for last view, this is nil if ond only of the current view
// was entered with a QC.
func (vt *viewTracker) LastViewTC() *flow.TimeoutCertificate {
	return vt.livenessData.LastViewTC
}

// ProcessQC ingests a QC, which might advance the current view. Panics for nil input!
// QCs with views smaller or equal to the newest QC known are a no-op. ProcessQC returns
// the resulting view after processing the QC.
// No errors are expected, any error should be treated as exception.
func (vt *viewTracker) ProcessQC(qc *flow.QuorumCertificate) (uint64, error) {
	view := vt.livenessData.CurrentView
	if qc.View < view {
		// If the QC is for a past view, our view does not change. Nevertheless, the QC might be
		// newer than the newest QC we know, since view changes can happen through TCs as well.
		// While not very likely, is is possible that individual replicas know newer QCs than the
		// ones previously included in TCs. E.g. a primary that crashed before it could construct
		// its block is has rebooted and is now sharing its newest QC as part of a TimeoutObject.
		err := vt.updateNewestQC(qc)
		if err != nil {
			return view, fmt.Errorf("could not update tracked newest QC: %w", err)
		}
		return view, nil
	}

	// supermajority of replicas have already voted during round `qc.view`, hence it is safe to proceed to subsequent view
	newView := qc.View + 1
	err := vt.updateLivenessData(newView, qc, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to update liveness data: %w", err)
	}
	return newView, nil
}

// ProcessTC ingests a TC, which might advance the current view. A nil TC is accepted as
// input, so that callers may pass in e.g. `Proposal.LastViewTC`, which may or may not have
// a value. It returns the resulting view after processing the TC and embedded QC.
// No errors are expected, any error should be treated as exception
func (vt *viewTracker) ProcessTC(tc *flow.TimeoutCertificate) (uint64, error) {
	view := vt.livenessData.CurrentView
	if tc == nil {
		return view, nil
	}

	if tc.View < view {
		// TC and the embedded QC are for a past view, hence our view does not change. Nevertheless,
		// the QC might be newer than the newest QC we know. While not very likely, is is possible
		// that individual replicas know newer QCs than the ones previously included in any TCs.
		// E.g. a primary that crashed before it could construct its block is has rebooted and
		// now contributed its newest QC to this TC.
		err := vt.updateNewestQC(tc.NewestQC)
		if err != nil {
			return 0, fmt.Errorf("could not update tracked newest QC: %w", err)
		}
		return view, nil
	}

	// supermajority of replicas have already reached their timeout for view `tc.View`, hence it is safe to proceed to subsequent view
	newView := tc.View + 1
	err := vt.updateLivenessData(newView, tc.NewestQC, tc)
	if err != nil {
		return 0, fmt.Errorf("failed to update liveness data: %w", err)
	}
	return newView, nil
}

// updateLivenessData updates the current view, qc, tc. We want to avoid unnecessary data-base
// writes, which we enforce by requiring that the view number is STRICTLY monotonously increasing.
// Otherwise, an exception is returned. No errors are expected, any error should be treated as exception.
func (vt *viewTracker) updateLivenessData(newView uint64, qc *flow.QuorumCertificate, tc *flow.TimeoutCertificate) error {
	if newView <= vt.livenessData.CurrentView {
		// This should never happen: in the current implementation, it is trivially apparent that
		// newView is _always_ larger than currentView. This check is to protect the code from
		// future modifications that violate the necessary condition for
		// STRICTLY monotonously increasing view numbers.
		return fmt.Errorf("cannot move from view %d to %d: currentView must be strictly monotonously increasing",
			vt.livenessData.CurrentView, newView)
	}

	vt.livenessData.CurrentView = newView
	if vt.livenessData.NewestQC.View < qc.View {
		vt.livenessData.NewestQC = qc
	}
	vt.livenessData.LastViewTC = tc
	err := vt.persist.PutLivenessData(&vt.livenessData)
	if err != nil {
		return fmt.Errorf("could not persist liveness data: %w", err)
	}
	return nil
}

// updateNewestQC updates the highest QC tracked by view, iff `qc` has a larger
// view than the newest stored QC. Otherwise, this method is a no-op.
// No errors are expected, any error should be treated as exception.
func (vt *viewTracker) updateNewestQC(qc *flow.QuorumCertificate) error {
	if vt.livenessData.NewestQC.View >= qc.View {
		return nil
	}

	vt.livenessData.NewestQC = qc
	err := vt.persist.PutLivenessData(&vt.livenessData)
	if err != nil {
		return fmt.Errorf("could not persist liveness data: %w", err)
	}

	return nil
}
