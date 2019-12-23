package flowmc

import (
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/components/pacemaker/events"
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/components/pacemaker/primary"
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"
	"time"
)

type FlowMcPacemaker struct {
	currentView uint64

	primarySelector primary.Selector
	eventProcessor events.Processor
	timeout Timeout

	// Replica's index among all other consensus nodes // ToDo: update to use identity component
	myNodeID primary.ID
}

// primaryForView returns true if I am primary for the specified view
func (p *FlowMcPacemaker) primaryForView(view uint64) bool {
	// ToDo: update to use identity component
	return p.primarySelector.PrimaryAtView(p.currentView) == p.myNodeID
}

func (p *FlowMcPacemaker) skipAhead(qc *def.QuorumCertificate) {
	if qc.View >= p.currentView {
		// qc.view = p.currentView + k for k ≥ 0
		// 2/3 of replicas have already voted for round p.currentView + k, hence proceeded past currentView
		// => 2/3 of replicas are at least in view qc.view + 1.
		// => replica can skip ahead to view qc.view + 1
		p.eventProcessor.OnPassiveTillView(qc.View)
		p.currentView = qc.View + 1
	}
}

func (p *FlowMcPacemaker) processBlock(block *def.Block) {
	if block.View != p.currentView {
		// ignore block from past views
		// ignore block is from future views as they could be a fast-forward attack
		return
	} // block is for current view

	// ToDo can we perform sanity check here that this never happens?
	// ```
	// if I am primary and block is not from me:
	//     panic // this should never happen
	// ```

	if !p.primaryForView(p.currentView) {
		p.currentView += 1
		return
	}
	// Replica is primary at current view
	// => wait for votes with ReplicaTimeout
	p.timeout.StartTimeout(p.currentView, VoteCollectionTimeout)
}

func (p *FlowMcPacemaker) executeView() {
	p.eventProcessor.OnEnteringView(p.currentView)
	p.timeout.StartTimeout(p.currentView, ReplicaTimeout)
	if p.primaryForView(p.currentView) {
		p.eventProcessor.OnForkChoiceTrigger(p.currentView)
	}

	startView := p.currentView //view number at start of processing
	for startView == p.currentView { // process until view number changes as effect of any event
		timeoutChannel := p.timeout.Channel()
		select {
			case block := <-p.blockIncorporatedEvents:
				p.skipAhead(block.QC)
				p.processBlock(block)
			case qc := <-p.qcFromVotesIncorporatedEvents:
				p.skipAhead(qc)
			case <- timeoutChannel:
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

// OnReplicaTimeout is a hook which is called when the replica timeout occurs
func (p *FlowMcPacemaker) emitTimeoutEvent() {
	switch p.timeout.Mode() {
		case VoteCollectionTimeout:
			p.eventProcessor.OnWaitingForVotesTimeout(p.currentView)
		case ReplicaTimeout:
			p.eventProcessor.OnWaitingForBlockTimeout(p.currentView)
		default:
			panic("unknown timeout mode")
	}
}

func (p *FlowMcPacemaker) run() {
	var timer *time.Timer
	for {
		p.currentView += 1
		p.eventProcessor.OnEnteringView(p.currentView)

		if p.primaryForView(p.currentView) {
			p.eventProcessor.OnForkChoiceTrigger(p.currentView)
		}

		// Replica
		ReplicaTimeoutLoop:
			for {
				select {
				case block := <-p.blockIncorporatedEvents:
					 if p.onIncorporatedBlock(block) {
						 break ReplicaTimeoutLoop
					 }
				case qc := <-p.qcFromVotesIncorporatedEvents:
					//ToDo:
					// - updated highest QC
					// - as Primary: check if qc if for current round and  build block
					// - advance to QC.View + 1
				case <-timer.C:
					p.onWaitingForBlockTimeout()
					break ReplicaTimeoutLoop
				}
			}

		timer = time.NewTimer(p.currentTimeout)
		select {
		case block := <-p.blockIncorporatedEvents:
			//ToDo: implement replica logic
		case qc := <-p.qcFromVotesIncorporatedEvents:
			//ToDo:
			// - updated highest QC
			// - as Primary: check if qc if for current round and  build block
			// - advance to QC.View + 1
		case <-timer.C:
			ReactorCoreLogger.Debugf("Timeout at View %d", r.currentView)
		}
	}
}

func (p *FlowMcPacemaker) Run() {
	go p.run()
}

