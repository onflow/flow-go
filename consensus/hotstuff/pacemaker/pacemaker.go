package pacemaker

import (
	"fmt"
	"sync"
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/model/flow"
)

// ActivePaceMaker implements the hotstuff.PaceMaker
// Conceptually, we use the Pacemaker algorithm first proposed in [1] (specifically Jolteon) and described in more detail in [2].
// [1] https://arxiv.org/abs/2106.10362
// [2] https://developers.diem.com/papers/diem-consensus-state-machine-replication-in-the-diem-blockchain/2021-08-17.pdf (aka DiemBFT v4)
//
// To enter a new view `v`, the Pacemaker must observe a valid QC or TC for view `v-1`.
// The Pacemaker also controls when a node should locally time out for a given view.
// In contrast to the passive Pacemaker (previous implementation), locally timing a view
// does not cause a view change.
// A local timeout for a view `v` causes a node to:
// * never produce a vote for any proposal with view ≤ `v`, after the timeout
// * produce and broadcast a timeout object, which can form a part of the TC for the timed out view
type ActivePaceMaker struct {
	timeoutControl *timeout.Controller
	notifier       hotstuff.Consumer
	persist        hotstuff.Persister
	livenessData   *hotstuff.LivenessData
	started        sync.Once
}

var _ hotstuff.PaceMaker = (*ActivePaceMaker)(nil)

// New creates a new ActivePaceMaker instance
//   - startView is the view for the pacemaker to start with.
//   - timeoutController controls the timeout trigger.
//   - notifier provides callbacks for pacemaker events.
//
// Expected error conditions:
// * model.ConfigurationError if initial LivenessData is invalid
func New(timeoutController *timeout.Controller,
	notifier hotstuff.Consumer,
	persist hotstuff.Persister,
) (*ActivePaceMaker, error) {
	livenessData, err := persist.GetLivenessData()
	if err != nil {
		return nil, fmt.Errorf("could not recover liveness data: %w", err)
	}

	if livenessData.CurrentView < 1 {
		return nil, model.NewConfigurationErrorf("PaceMaker cannot start in view 0 (view zero is reserved for genesis block, which has no proposer)")
	}
	pm := ActivePaceMaker{
		livenessData:   livenessData,
		timeoutControl: timeoutController,
		notifier:       notifier,
		persist:        persist,
	}
	return &pm, nil
}

// updateLivenessData updates the current view, qc, tc. Currently, the calling code
// ensures that the view number is STRICTLY monotonously increasing. The method
// updateLivenessData panics as a last resort if ActivePaceMaker is modified to violate this condition.
// No errors are expected, any error should be treated as exception.
func (p *ActivePaceMaker) updateLivenessData(newView uint64, qc *flow.QuorumCertificate, tc *flow.TimeoutCertificate) error {
	if newView <= p.livenessData.CurrentView {
		// This should never happen: in the current implementation, it is trivially apparent that
		// newView is _always_ larger than currentView. This check is to protect the code from
		// future modifications that violate the necessary condition for
		// STRICTLY monotonously increasing view numbers.
		return fmt.Errorf("cannot move from view %d to %d: currentView must be strictly monotonously increasing",
			p.livenessData.CurrentView, newView)
	}

	p.livenessData.CurrentView = newView
	if p.livenessData.NewestQC.View < qc.View {
		p.livenessData.NewestQC = qc
	}
	p.livenessData.LastViewTC = tc
	err := p.persist.PutLivenessData(p.livenessData)
	if err != nil {
		return fmt.Errorf("could not persist liveness data: %w", err)
	}

	return nil
}

// updateNewestQC updates the highest QC tracked by view, iff `qc` has a larger view than
// the QC stored in the PaceMaker's `livenessData`. Otherwise, this method is a no-op.
// No errors are expected, any error should be treated as exception.
func (p *ActivePaceMaker) updateNewestQC(qc *flow.QuorumCertificate) error {
	if p.livenessData.NewestQC.View >= qc.View {
		return nil
	}

	p.livenessData.NewestQC = qc
	err := p.persist.PutLivenessData(p.livenessData)
	if err != nil {
		return fmt.Errorf("could not persist liveness data: %w", err)
	}

	return nil
}

// CurView returns the current view
func (p *ActivePaceMaker) CurView() uint64 {
	return p.livenessData.CurrentView
}

// TimeoutChannel returns the timeout channel for current active timeout.
// Note the returned timeout channel returns only one timeout, which is the current
// timeout.
// To get the timeout for the next timeout, you need to call TimeoutChannel() again.
func (p *ActivePaceMaker) TimeoutChannel() <-chan time.Time {
	return p.timeoutControl.Channel()
}

// ProcessQC notifies the pacemaker with a new QC, which might allow pacemaker to
// fast-forward its view. In contrast to `ProcessTC`, this function does _not_ handle `nil` inputs.
// No errors are expected, any error should be treated as exception
func (p *ActivePaceMaker) ProcessQC(qc *flow.QuorumCertificate) (*model.NewViewEvent, error) {
	if qc.View < p.CurView() {
		err := p.updateNewestQC(qc)
		if err != nil {
			return nil, fmt.Errorf("could not update tracked newest QC: %w", err)
		}
		return nil, nil
	}

	p.timeoutControl.OnProgressBeforeTimeout()

	// supermajority of replicas have already voted during round `qc.view`, hence it is safe to proceed to subsequent view
	newView := qc.View + 1
	err := p.updateLivenessData(newView, qc, nil)
	if err != nil {
		return nil, err
	}

	p.notifier.OnQcTriggeredViewChange(qc, newView)

	timerInfo := p.timeoutControl.StartTimeout(newView)
	p.notifier.OnStartingTimeout(timerInfo)

	return &model.NewViewEvent{
		View:      timerInfo.View,
		StartTime: timerInfo.StartTime,
		Duration:  timerInfo.Duration,
	}, nil
}

// ProcessTC notifies the Pacemaker of a new timeout certificate, which may allow
// Pacemaker to fast-forward its current view.
// A nil TC is an expected valid input, so that callers may pass in e.g. `Proposal.LastViewTC`,
// which may or may not have a value.
// No errors are expected, any error should be treated as exception
func (p *ActivePaceMaker) ProcessTC(tc *flow.TimeoutCertificate) (*model.NewViewEvent, error) {
	if tc == nil {
		return nil, nil
	}

	if tc.View < p.CurView() {
		err := p.updateNewestQC(tc.NewestQC)
		if err != nil {
			return nil, fmt.Errorf("could not update tracked newest QC: %w", err)
		}
		return nil, nil
	}

	p.timeoutControl.OnTimeout()

	// supermajority of replicas have already reached their timeout for view `tc.View`, hence it is safe to proceed to subsequent view
	newView := tc.View + 1
	err := p.updateLivenessData(newView, tc.NewestQC, tc)
	if err != nil {
		return nil, err
	}

	p.notifier.OnTcTriggeredViewChange(tc, newView)

	timerInfo := p.timeoutControl.StartTimeout(newView)
	p.notifier.OnStartingTimeout(timerInfo)

	return &model.NewViewEvent{
		View:      timerInfo.View,
		StartTime: timerInfo.StartTime,
		Duration:  timerInfo.Duration,
	}, nil
}

// NewestQC returns QC with the highest view discovered by PaceMaker.
func (p *ActivePaceMaker) NewestQC() *flow.QuorumCertificate {
	return p.livenessData.NewestQC
}

// LastViewTC returns TC for last view, this will be nil only if the current view
// was entered with a QC.
func (p *ActivePaceMaker) LastViewTC() *flow.TimeoutCertificate {
	return p.livenessData.LastViewTC
}

// Start starts the pacemaker by starting the initial timer for the current view.
// Start should only be called once - subsequent calls are a no-op.
func (p *ActivePaceMaker) Start() {
	p.started.Do(func() {
		timerInfo := p.timeoutControl.StartTimeout(p.CurView())
		p.notifier.OnStartingTimeout(timerInfo)
	})
}

// BlockRateDelay returns the delay for broadcasting its own proposals.
func (p *ActivePaceMaker) BlockRateDelay() time.Duration {
	return p.timeoutControl.BlockRateDelay()
}
