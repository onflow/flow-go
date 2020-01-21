package flowmc

import (
	"time"

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

func (p *FlowMC) gotoView(newView uint64) {
	if newView > p.currentView+1 {
		p.notifier.OnSkippedAhead(newView)
	}
	p.currentView = newView
	p.notifier.OnStartingBlockTimeout(newView)
	p.timeoutControl.StartTimeout(timeout.ReplicaTimeout)
}

// CurView returns the current view
func (p *FlowMC) CurView() uint64 {
	return p.currentView
}

func (p *FlowMC) TimeoutChannel() <-chan time.Time {
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
	newView := qc.View + 1
	p.gotoView(newView)
	return &types.NewViewEvent{View: newView}, true
}

func (p *FlowMC) UpdateCurViewWithBlock(block *types.BlockProposal, isLeaderForNextView bool) (*types.NewViewEvent, bool) {
	// use block's QC to fast-forward if possible
	newViewOnQc, newViewOccuredOnQc := p.UpdateCurViewWithQC(block.QC())
	if block.View() != p.currentView {
		return newViewOnQc, newViewOccuredOnQc
	}
	// block is for current view

	if p.timeoutControl.Mode() != timeout.ReplicaTimeout {
		// i.e. we are already on timeout.VoteCollection.
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
		p.timeoutControl.StartTimeout(timeout.VoteCollection)
		p.notifier.OnStartingVotesTimeout(p.currentView)
		return nil, false
	}

	newView := p.currentView + 1
	p.gotoView(newView)
	return &types.NewViewEvent{View: newView}, true
}

func (p *FlowMC) OnTimeout() (*types.NewViewEvent, bool) {
	// block is for current view
	switch p.timeoutControl.Mode() {
	case timeout.ReplicaTimeout:
		p.notifier.OnReachedBlockTimeout(p.currentView)
	case timeout.VoteCollection:
		p.notifier.OnReachedVotesTimeout(p.currentView)
	default: // this should never happen unless the number of TimeoutModes are extended without updating this code
		panic("unknown timeout mode " + string(p.timeoutControl.Mode()))
	}

	newView := p.currentView + 1
	p.gotoView(newView)
	return &types.NewViewEvent{View: newView}, true
}

func (p *FlowMC) Start() {
	if p.started.Swap(true) {
		return
	}
	p.gotoView(p.currentView)
}
