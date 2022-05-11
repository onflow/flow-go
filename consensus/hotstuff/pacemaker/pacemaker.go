package pacemaker

import (
	"fmt"
	"time"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/model/flow"
)

// ActivePaceMaker implements the hotstuff.PaceMaker
// Its an aggressive pacemaker with exponential increase on timeout as well as
// exponential decrease on progress. Progress is defined as entering view V
// for which the replica knows a QC with V = QC.view + 1
type ActivePaceMaker struct {
	timeoutControl *timeout.Controller
	notifier       hotstuff.Consumer
	persist        hotstuff.Persister
	livenessData   *hotstuff.LivenessData
	started        *atomic.Bool
}

var _ hotstuff.PaceMaker = (*ActivePaceMaker)(nil)

// New creates a new ActivePaceMaker instance
// startView is the view for the pacemaker to start from
// timeoutController controls the timeout trigger.
// notifier provides callbacks for pacemaker events.
func New(livenessData *hotstuff.LivenessData,
	timeoutController *timeout.Controller,
	notifier hotstuff.Consumer,
	persist hotstuff.Persister) (*ActivePaceMaker, error) {
	if livenessData.CurrentView < 1 {
		return nil, model.NewConfigurationErrorf("Please start PaceMaker with view > 0. (View 0 is reserved for genesis block, which has no proposer)")
	}
	pm := ActivePaceMaker{
		livenessData:   livenessData,
		timeoutControl: timeoutController,
		notifier:       notifier,
		persist:        persist,
		started:        atomic.NewBool(false),
	}
	return &pm, nil
}

// updateLivenessData updates the current view, qc, tc. Currently, the calling code
// ensures that the view number is STRICTLY monotonously increasing. The method
// updateLivenessData panics as a last resort if ActivePaceMaker is modified to violate this condition.
// No errors are expected, any error should be threaded as exception
func (p *ActivePaceMaker) updateLivenessData(newView uint64, qc *flow.QuorumCertificate, tc *flow.TimeoutCertificate) error {
	if newView <= p.livenessData.CurrentView {
		// This should never happen: in the current implementation, it is trivially apparent that
		// newView is _always_ larger than currentView. This check is to protect the code from
		// future modifications that violate the necessary condition for
		// STRICTLY monotonously increasing view numbers.
		panic(fmt.Sprintf("cannot move from view %d to %d: currentView must be strictly monotonously increasing",
			p.livenessData.CurrentView, newView))
	}

	p.livenessData.CurrentView = newView
	if p.livenessData.HighestQC.View < qc.View {
		p.livenessData.HighestQC = qc
	}
	p.livenessData.LastViewTC = tc
	err := p.persist.PutLivenessData(p.livenessData)
	if err != nil {
		return fmt.Errorf("could not persist liveness data: %w", err)
	}

	timerInfo := p.timeoutControl.StartTimeout(model.ReplicaTimeout, newView)
	p.notifier.OnStartingTimeout(timerInfo)
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
// fast-forward its view.
func (p *ActivePaceMaker) ProcessQC(qc *flow.QuorumCertificate) (*model.NewViewEvent, error) {
	if qc.View < p.CurView() {
		return nil, nil
	}

	p.timeoutControl.OnProgressBeforeTimeout()

	// qc.view = p.currentView + k for k ≥ 0
	// 2/3 of replicas have already voted for round p.currentView + k, hence proceeded past currentView
	// => 2/3 of replicas are at least in view qc.view + 1.
	// => replica can skip ahead to view qc.view + 1
	newView := qc.View + 1
	err := p.updateLivenessData(newView, qc, nil)
	if err != nil {
		return nil, err
	}

	p.notifier.OnQcTriggeredViewChange(qc, newView)
	return &model.NewViewEvent{View: newView}, nil
}

func (p *ActivePaceMaker) ProcessTC(tc *flow.TimeoutCertificate) (*model.NewViewEvent, error) {
	if tc == nil || tc.View < p.CurView() {
		return nil, nil
	}

	newView := tc.View + 1
	err := p.updateLivenessData(newView, tc.TOHighestQC, tc)
	if err != nil {
		return nil, err
	}

	p.notifier.OnTcTriggeredViewChange(tc, newView)
	return &model.NewViewEvent{View: newView}, nil
}

func (p *ActivePaceMaker) OnPartialTC(newView uint64) {
	if p.CurView() == newView {
		p.timeoutControl.TriggerTimeout()
	}
}

// HighestQC returns QC with the highest view discovered by PaceMaker.
func (p *ActivePaceMaker) HighestQC() *flow.QuorumCertificate {
	return p.livenessData.HighestQC
}

// LastViewTC returns TC for last view, this could be nil if previous round
// has entered with a QC.
func (p *ActivePaceMaker) LastViewTC() *flow.TimeoutCertificate {
	return p.livenessData.LastViewTC
}

// Start starts the pacemaker
func (p *ActivePaceMaker) Start() {
	if p.started.Swap(true) {
		return
	}
	timerInfo := p.timeoutControl.StartTimeout(model.ReplicaTimeout, p.CurView())
	p.notifier.OnStartingTimeout(timerInfo)
}

// BlockRateDelay returns the delay for broadcasting its own proposals.
func (p *ActivePaceMaker) BlockRateDelay() time.Duration {
	return p.timeoutControl.BlockRateDelay()
}
