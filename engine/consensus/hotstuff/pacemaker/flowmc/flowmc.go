package flowmc

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/pacemaker/flowmc/timeout"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/utils"
)

// FlowMC is a basic implementation of hotstuff.PaceMaker
type FlowMC struct {
	currentView       uint64
	timeoutControl    *timeout.Controller
	eventProcessor    notifications.Distributor
	onWaitingForVotes bool
	started           bool
}

func New(startView uint64, timeoutController *timeout.Controller, eventProc notifications.Distributor) (hotstuff.PaceMaker, error) {
	if startView < 1 {
		return nil, &types.ErrorConfiguration{Msg: "Please start PaceMaker with view > 0. (View 0 is reserved for genesis block, which has no proposer)"}
	}
	if utils.IsNil(timeoutController) {
		timeoutController = timeout.DefaultController()
	}
	if utils.IsNil(eventProc) {
		return nil, &types.ErrorConfiguration{Msg: "notifications.Distributor cannot be nil"}
	}
	pm := FlowMC{
		currentView:    startView,
		timeoutControl: timeoutController,
		eventProcessor: eventProc,
		started:        false,
	}
	return &pm, nil
}

func (p *FlowMC) startView(newView uint64) {
	if newView > p.currentView+1 {
		p.eventProcessor.OnSkippedAhead(newView)
	}
	p.currentView = newView
	p.onWaitingForVotes = false
	p.eventProcessor.OnStartingBlockTimeout(newView)
	p.timeoutControl.StartTimeout(timeout.ReplicaTimeout)
}

// CurView returns the current view
func (p *FlowMC) CurView() uint64 {
	return p.currentView
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
	p.startView(newView)
	return &types.NewViewEvent{View: newView}, true
}

func (p *FlowMC) UpdateCurViewWithBlock(block *types.BlockProposal, isLeaderForNextView bool) (*types.NewViewEvent, bool) {
	if block.View() < p.currentView {
		return nil, false
	}
	if block.View() > p.currentView {
		qcNve, qcNveOccured := p.UpdateCurViewWithQC(block.QC())
		if !qcNveOccured {
			return nil, false
		}
		blockNve, blockNveOccured := p.UpdateCurViewWithBlock(block, isLeaderForNextView)
		if blockNveOccured {
			return blockNve, blockNveOccured
		}
		return qcNve, qcNveOccured
	}
	// block is for current view
	if p.onWaitingForVotes {
		return nil, false
	}

	if isLeaderForNextView {
		p.onWaitingForVotes = true
		p.timeoutControl.StartTimeout(timeout.VoteCollectionTimeout)
		p.eventProcessor.OnStartingVotesTimeout(p.currentView)
		return nil, false
	}

	newView := p.currentView + 1
	p.startView(newView)
	return &types.NewViewEvent{View: newView}, true
}

func (p *FlowMC) OnTimeout() (*types.NewViewEvent, bool) {
	if p.onWaitingForVotes {
		p.eventProcessor.OnReachedVotesTimeout(p.currentView)
	} else {
		p.eventProcessor.OnReachedBlockTimeout(p.currentView)
	}

	newView := p.currentView + 1
	p.startView(newView)
	return &types.NewViewEvent{View: newView}, true
}

func (p *FlowMC) Start() {
	if p.started {
		return
	}
	p.startView(p.currentView)
}
