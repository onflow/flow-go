package pacemaker

import (
	"fmt"
	"time"

	"go.uber.org/atomic"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker/timeout"
)

// NitroPaceMaker implements the hotstuff.PaceMaker
// Its an aggressive pacemaker with exponential increase on timeout as well as
// exponential decrease on progress. Progress is defined as entering view V
// for which the replica knows a QC with V = QC.view + 1
type NitroPaceMaker struct {
	currentView    uint64
	timeoutControl *timeout.Controller
	notifier       hotstuff.Consumer
	started        *atomic.Bool
}

func New(startView uint64, timeoutController *timeout.Controller, notifier hotstuff.Consumer) (*NitroPaceMaker, error) {
	if startView < 1 {
		return nil, &model.ErrorConfiguration{Msg: "Please start PaceMaker with view > 0. (View 0 is reserved for genesis block, which has no proposer)"}
	}
	pm := NitroPaceMaker{
		currentView:    startView,
		timeoutControl: timeoutController,
		notifier:       notifier,
		started:        atomic.NewBool(false),
	}
	return &pm, nil
}

// gotoView updates the current view to newView. Currently, the calling code
// ensures that the view number is STRICTLY monotonously increasing. The method
// gotoView panics as a last resort if FlowPaceMaker is modified to violate this condition.
// Hence, gotoView will _always_ return a NewViewEvent for an _increased_ view number.
func (p *NitroPaceMaker) gotoView(newView uint64) *model.NewViewEvent {
	if newView <= p.currentView {
		// This should never happen: in the current implementation, it is trivially apparent that
		// newView is _always_ larger than currentView. This check is to protect the code from
		// future modifications that violate the necessary condition for
		// STRICTLY monotonously increasing view numbers.
		panic(fmt.Sprintf("cannot move from view %d to %d: currentView must be strictly monotonously increasing", p.currentView, newView))
	}
	if newView > p.currentView+1 {
		p.notifier.OnSkippedAhead(newView)
	}
	p.currentView = newView
	timerInfo := p.timeoutControl.StartTimeout(model.ReplicaTimeout, newView)
	p.notifier.OnStartingTimeout(timerInfo)
	return &model.NewViewEvent{View: p.currentView}
}

// CurView returns the current view
func (p *NitroPaceMaker) CurView() uint64 {
	return p.currentView
}

func (p *NitroPaceMaker) TimeoutChannel() <-chan time.Time {
	return p.timeoutControl.Channel()
}

func (p *NitroPaceMaker) UpdateCurViewWithQC(qc *model.QuorumCertificate) (*model.NewViewEvent, bool) {
	if qc.View < p.currentView {
		return nil, false
	}
	// qc.view = p.currentView + k for k ≥ 0
	// 2/3 of replicas have already voted for round p.currentView + k, hence proceeded past currentView
	// => 2/3 of replicas are at least in view qc.view + 1.
	// => replica can skip ahead to view qc.view + 1
	p.timeoutControl.OnProgressBeforeTimeout()
	return p.gotoView(qc.View + 1), true
}

func (p *NitroPaceMaker) UpdateCurViewWithBlock(block *model.Block, isLeaderForNextView bool) (*model.NewViewEvent, bool) {
	// use block's QC to fast-forward if possible
	newViewOnQc, newViewOccurredOnQc := p.UpdateCurViewWithQC(block.QC)
	if block.View != p.currentView {
		return newViewOnQc, newViewOccurredOnQc
	}
	// block is for current view

	if p.timeoutControl.TimerInfo().Mode != model.ReplicaTimeout {
		// i.e. we are already on timeout.VoteCollectionTimeout.
		// This edge case can occur as follows:
		// * we previously already have processed a block for the current view
		//   and started the vote collection phase
		// In this case, we do NOT want to RE-start the vote collection timer
		// if we get a second block for the current View.
		return nil, false
	}
	newViewOnBlock, newViewOccurredOnBlock := p.actOnBlockForCurView(block, isLeaderForNextView)
	if !newViewOccurredOnBlock { // if processing current block didn't lead to NewView event,
		// the initial processing of the block's QC still might have changes the view:
		return newViewOnQc, newViewOccurredOnQc
	}
	// processing current block created NewView event, which is always newer than any potential newView event from processing the block's QC
	return newViewOnBlock, newViewOccurredOnBlock
}

func (p *NitroPaceMaker) actOnBlockForCurView(block *model.Block, isLeaderForNextView bool) (*model.NewViewEvent, bool) {
	if isLeaderForNextView {
		timerInfo := p.timeoutControl.StartTimeout(model.VoteCollectionTimeout, p.currentView)
		p.notifier.OnStartingTimeout(timerInfo)
		return nil, false
	}
	if block.QC.View+1 == p.currentView {
		// only decrease timeout if block has been build on a quorum from the previous view;
		// otherwise, the committee is still not synchronized (as the qc is from a view _prior_ to the previous one)
		p.timeoutControl.OnProgressBeforeTimeout()
	}
	return p.gotoView(p.currentView + 1), true
}

func (p *NitroPaceMaker) OnTimeout() *model.NewViewEvent {
	p.emitTimeoutNotifications(p.timeoutControl.TimerInfo())
	p.timeoutControl.OnTimeout()
	return p.gotoView(p.currentView + 1)
}

func (p *NitroPaceMaker) emitTimeoutNotifications(timeout *model.TimerInfo) {
	p.notifier.OnReachedTimeout(timeout)
}

func (p *NitroPaceMaker) Start() {
	if p.started.Swap(true) {
		return
	}
	timerInfo := p.timeoutControl.StartTimeout(model.ReplicaTimeout, p.currentView)
	p.notifier.OnStartingTimeout(timerInfo)
}

func (p *NitroPaceMaker) BlockRateDelay() time.Duration {
	return p.timeoutControl.BlockRateDelay()
}
