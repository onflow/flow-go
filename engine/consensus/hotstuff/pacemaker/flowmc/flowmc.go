package flowmc

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/pacemaker/flowmc/timeout"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"go.uber.org/atomic"
)

// FlowMC is a basic implementation of hotstuff.PaceMaker
type FlowMC struct {
	currentView    uint64
	timeoutControl *timeout.Controller
	notifier       notifications.Distributor
	started        *atomic.Bool
}

func New(startView uint64, timeoutController *timeout.Controller, notifier notifications.Distributor) (hotstuff.PaceMaker, error) {
	if startView < 1 {
		return nil, &types.ErrorConfiguration{Msg: "Please start PaceMaker with view > 0. (View 0 is reserved for genesis block, which has no proposer)"}
	}
	pm := FlowMC{
		currentView:    startView,
		timeoutControl: timeoutController,
		notifier:       notifier,
		started:        atomic.NewBool(false),
	}
	return &pm, nil
}

// gotoView updates the current view to newView. Currently, the calling code
// ensures that the view number is STRICTLY monotonously increasing. The method
// gotoView panics as a last resort if FlowMC is modified to violate this condition.
// Hence, gotoView will _always_ return a NewViewEvent for an _increased_ view number.
func (p *FlowMC) gotoView(newView uint64) *types.NewViewEvent {
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
	p.notifier.OnStartingBlockTimeout(newView)
	p.timeoutControl.StartTimeout(types.ReplicaTimeout, newView)
	return &types.NewViewEvent{View: p.currentView}
}

// CurView returns the current view
func (p *FlowMC) CurView() uint64 {
	return p.currentView
}

func (p *FlowMC) TimeoutChannel() <-chan *types.Timeout {
	return p.timeoutControl.Channel()
}

func (p *FlowMC) UpdateCurViewWithQC(qc *types.QuorumCertificate) (*types.NewViewEvent, bool) {
	if qc.View < p.currentView {
		return nil, false
	}
	// qc.view = p.currentView + k for k ≥ 0
	// 2/3 of replicas have already voted for round p.currentView + k, hence proceeded past currentView
	// => 2/3 of replicas are at least in view qc.view + 1.
	// => replica can skip ahead to view qc.view + 1
	return p.gotoView(qc.View + 1), true
}

func (p *FlowMC) UpdateCurViewWithBlock(block *types.BlockProposal, isLeaderForNextView bool) (*types.NewViewEvent, bool) {
	// use block's QC to fast-forward if possible
	newViewOnQc, newViewOccuredOnQc := p.UpdateCurViewWithQC(block.QC())
	if block.View() != p.currentView {
		return newViewOnQc, newViewOccuredOnQc
	}
	// block is for current view

	if p.timeoutControl.Mode() != types.ReplicaTimeout {
		// i.e. we are already on timeout.VoteCollectionTimeout.
		// This edge case can occur as follows:
		// * we previously already have processed a block for the current view
		//   and started the vote collection phase
		// In this case, we do NOT want to RE-start the vote collection timer
		// if we get a second block for the current View.
		return nil, false
	}
	newViewOnBlock, newViewOccuredOnBlock := p.processBlockForCurView(block, isLeaderForNextView)
	if !newViewOccuredOnBlock { // if processing current block didn't lead to NewView event,
		// the initial processing of the block's QC still might have changes the view:
		return newViewOnQc, newViewOccuredOnQc
	}
	// processing current block created NewView event, which is always newer than any potential newView event from processing the block's QC
	return newViewOnBlock, newViewOccuredOnBlock
}

func (p *FlowMC) processBlockForCurView(block *types.BlockProposal, isLeaderForNextView bool) (*types.NewViewEvent, bool) {
	if isLeaderForNextView {
		p.timeoutControl.StartTimeout(types.VoteCollectionTimeout, p.currentView)
		p.notifier.OnStartingVotesTimeout(p.currentView)
		return nil, false
	}
	return p.gotoView(p.currentView + 1), true
}

func (p *FlowMC) OnTimeout(timeout *types.Timeout) (*types.NewViewEvent, error) {
	if err := p.ensureMatchingTimeout(timeout); err != nil {
		return nil, err
	}

	// timeout is for current condition
	p.emitTimeoutNotifications()
	return p.gotoView(p.currentView + 1), nil
}

func (p *FlowMC) ensureMatchingTimeout(timeout *types.Timeout) error {
	if timeout.View != p.currentView || p.timeoutControl.Mode() != timeout.Mode {
		// this indicates a bug in the usage of the PaceMaker
		return &types.ErrorInvalidTimeout{Timeout: timeout, CurrentView: p.currentView, CurrentMode: p.timeoutControl.Mode()}
	}
	return nil
}

func (p *FlowMC) emitTimeoutNotifications() {
	switch p.timeoutControl.Mode() {
	case types.ReplicaTimeout:
		p.notifier.OnReachedBlockTimeout(p.currentView)
	case types.VoteCollectionTimeout:
		p.notifier.OnReachedVotesTimeout(p.currentView)
	default: // this should never happen unless the number of TimeoutModes are extended without updating this code
		panic("unknown timeout mode " + string(p.timeoutControl.Mode()))
	}
}

func (p *FlowMC) Start() {
	if p.started.Swap(true) {
		return
	}
	p.notifier.OnStartingBlockTimeout(p.currentView)
	p.timeoutControl.StartTimeout(types.ReplicaTimeout, p.currentView)
}
