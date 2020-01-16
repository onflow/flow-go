package flowmc

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/pacemaker/primary"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"go.uber.org/atomic"
)

type FlowMC struct {
	currentView  uint64
	currentBlock *types.BlockProposal

	primarySelector primary.Selector
	timeout         Timeout
	eventProcessor  notifications.Distributor

	qcFromVotesIncorporatedEvents chan *types.QuorumCertificate
	blockIncorporatedEvents       chan *types.BlockProposal

	// stopSignal: channel is closed on FlowMC.Stop()
	stopSignaled *atomic.Bool
	stopSignal   chan struct{}
}

func New(startView uint64, eventProc notifications.Distributor) *FlowMC, error {
	if view < 1 {
		panic("Please start PaceMaker with view > 0. (View 0 is reserved for genesis block, which has no proposer)")
	}
	return &FlowMC{
		myNodeID:                      id,
		currentView:                   view,
		primarySelector:               primarySelector,
		timeout:                       DefaultTimout(),
		eventProcessor:                eventProc,
		qcFromVotesIncorporatedEvents: make(chan *types.QuorumCertificate, 10),
		blockIncorporatedEvents:       make(chan *types.BlockProposal, 300),
		stopSignaled:                  atomic.NewBool(false),
		stopSignal:                    make(chan struct{}),
	}
}

func (p *FlowMC) OnBlockIncorporated(block *types.BlockProposal) {
	// inspired by https://content.pivotal.io/blog/a-channel-based-ring-buffer-in-go
	select {
		case p.blockIncorporatedEvents <- block:
		default:
			bufferedBlock := <-p.blockIncorporatedEvents
			if bufferedBlock.QC().View > block.QC().View {
				// Edge-case: buffered block's qc has a higher view than the new block's qc.
				// Put the block whose qc has highest view in the buffer;
				// thereby we guarantee that we keep the block in the buffer whose qc has the highest view.
				p.blockIncorporatedEvents <- bufferedBlock
			} else {
				p.blockIncorporatedEvents <- block
			}
	}
}

func (p *FlowMC) OnQcFromVotesIncorporated(qc *types.QuorumCertificate) {
	select {
		case p.qcFromVotesIncorporatedEvents <- qc:
		default:
			bufferedQC := <-p.qcFromVotesIncorporatedEvents
			if bufferedQC.View > qc.View {
				// Edge-case: buffered qc has a higher view than the new qc
				// Put the qc with highest view in the buffer;
				// thereby we guarantee that we keep the qc with the highest view in the buffer.
				p.qcFromVotesIncorporatedEvents <- bufferedQC
			} else {
				p.qcFromVotesIncorporatedEvents <- qc
			}
	}
}

// primaryForView returns true if I am primary for the specified view
func (p *FlowMC) isPrimaryForView(view uint64) bool {
	// ToDo: update to use identity component
	return p.primarySelector.PrimaryAtView(p.currentView) == p.myNodeID
}

func (p *FlowMC) skipAhead(qc *types.QuorumCertificate) {
	if qc.View >= p.currentView {
		// qc.view = p.currentView + k for k ≥ 0
		// 2/3 of replicas have already voted for round p.currentView + k, hence proceeded past currentView
		// => 2/3 of replicas are at least in view qc.view + 1.
		// => replica can skip ahead to view qc.view + 1
		p.eventProcessor.OnPassiveTillView(qc.View)
		p.currentView = qc.View + 1
	}
}

func (p *FlowMC) processBlock(block *types.BlockProposal) {
	if block.View() != p.currentView {
		// ignore block from past views
		// ignore block is from future views as they could be a fast-forward attack
		return
	} // block is for current view

	if p.currentBlock != nil {
		// we have already seen a block for this view;
		// reporting double-proposals is not the job of the PaceMaker. Hence, we can safely ignore them.
		return
	}
	p.currentBlock = block

	// ToDo can we perform sanity check here that this never happens?
	// ```
	// if I am primary and block is not from me:
	//     panic // this should never happen
	// ```

	if !p.isPrimaryForView(p.currentView) {
		p.currentView += 1
		return
	}
	// Replica is primary at current view
	// => wait for votes with ReplicaTimeout
	p.timeout.StartTimeout(p.currentView, VoteCollectionTimeout)
}

func (p *FlowMC) ExecuteView() {
	p.currentBlock = nil
	p.timeout.StartTimeout(p.currentView, ReplicaTimeout)
	p.eventProcessor.OnEnteringView(p.currentView)
	if p.isPrimaryForView(p.currentView) {
		p.eventProcessor.OnForkChoiceTrigger(p.currentView)
	}

	startView := p.currentView       //view number at start of processing
	for startView == p.currentView { // process until view number changes as effect of any event
		timeoutChannel := p.timeout.Channel()
		select {
			case <-p.stopSignal:
				return
			case block := <-p.blockIncorporatedEvents:
				p.skipAhead(block.QC())
				p.processBlock(block)
			case qc := <-p.qcFromVotesIncorporatedEvents:
				p.skipAhead(qc)
			case <-timeoutChannel:
				p.timeout.OnTimeout()
				p.emitTimeoutEvent()
				p.currentView += 1
		}
	}

	// sanity check:
	if !(startView < p.currentView) { //view number must be strictly monotonously increasing
		panic("currentView is NOT strictly monotonously increasing")
	}
}

func (p *FlowMC) CurrentView() uint64 {
	return p.currentView
}

// OnReplicaTimeout is a hook which is called when the replica timeout occurs
func (p *FlowMC) emitTimeoutEvent() {
	switch p.timeout.Mode() {
	case VoteCollectionTimeout:
		p.eventProcessor.OnWaitingForVotesTimeout(p.currentView)
	case ReplicaTimeout:
		p.eventProcessor.OnWaitingForBlockTimeout(p.currentView)
	default:
		panic("unknown timeout mode")
	}
}

func (p *FlowMC) run() {
	for !p.stopSignaled.Load() {
		p.ExecuteView()
	}
}

func (p *FlowMC) Run() {
	go p.run()
}

func (p *FlowMC) Stop() {
	if !p.stopSignaled.Swap(true) {
		close(p.stopSignal)
	}
}
