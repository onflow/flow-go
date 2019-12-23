package pacemaker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/components/pacemaker/events"
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/components/pacemaker/primary"
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"time"
)

type FlowMC struct {
	currentView uint64

	primarySelector primary.Selector
	eventProcessor events.Processor
	timeoutController TimoutController

	incorporatedQcEvents  chan *def.QuorumCertificate
	incorporatedBlockEvents  chan *def.Block

	// Replica's index among all other consensus nodes // ToDo: update to use identity component
	myNodeIndex uint
}

func NewBasicPacemaker() *FlowMC {
	return &FlowMC{}
}

func (p *FlowMC) OnBlockIncorporated(block *def.Block) {
	// inspired by https://content.pivotal.io/blog/a-channel-based-ring-buffer-in-go
	select {
	case p.incorporatedBlockEvents <- block:
	default:
		<-p.incorporatedBlockEvents
		p.incorporatedBlockEvents <- block
	}
}

func (p *FlowMC) OnQcFromVotesIncorporated(qc *def.QuorumCertificate) {
	select {
	case p.incorporatedQcEvents <- qc:
	default:
		<-p.incorporatedQcEvents
		p.incorporatedQcEvents <- qc
	}
}

func (p *FlowMC) runLeaderRound(view uint64) {
	p.eventProcessor.OnForkChoiceTrigger(view)
}

func (p *FlowMC) runReplicaRound(view uint64) {

}

// primaryForView returns true if I am primary for the specified view
func (p *FlowMC) primaryForView(view uint64) bool {
	// ToDo: update to use identity component
	return p.primarySelector.PrimaryAtView(p.currentView) == p.myNodeIndex
}

// OnReplicaTimeout is a hook which is called when the replica timeout occurs
func (p *FlowMC) onWaitingForBlockTimeout() {
	p.timeoutController.OnTimeout()
	p.eventProcessor.OnWaitingForBlockTimeout(p.currentView)

}

func (p *FlowMC) enterView(view uint64) {
	p.currentView = view
	p.eventProcessor.OnEnteringView(view)
}

func (p *FlowMC) onIncorporatedBlock(block *def.Block) bool {
	if block.QC.View >= block.View { // sanity check
		m := fmt.Sprintf("Internal error: received invalid block with block.qc.view=%d but block.view=%d", block.QC.View, block.View)
		panic(m)
	}
	if block.QC.View >= p.currentView {
		// fast forward
		p.enterView(block.QC.View + 1)
	} // Even if I fast-forwarded, I cannot be primary for the resulting p.currentView and receive a block from somebody else for this view
	if block.View == p.currentView {
		p.enterView(p.currentView + 1)
		return true
	}
	return false
}

func (p *FlowMC) onIncorporatedQC(qc *def.QuorumCertificate) bool {
	if qc.View >= p.currentView {
		// fast forward
		p.enterView(qc.View + 1)
	}
}

func (p *FlowMC) executeView(currentView uint64) uint64 {
	var timer *time.Timer

	p.eventProcessor.OnEnteringView(view)

	if p.primaryForView(view) {
		p.eventProcessor.OnForkChoiceTrigger(view)
	}

	// Replica
	ReplicaTimeoutLoop:
	for {
		select {
		case block := <-p.incorporatedBlockEvents:
			if p.onIncorporatedBlock(block) {
				break ReplicaTimeoutLoop
			}
		case qc := <-p.incorporatedQcEvents:
			//ToDo:
			// - updated highest QC
			// - as Primary: check if qc if for current round and  build block
			// - advance to QC.View + 1
		case <-timer.C:
			p.onWaitingForBlockTimeout()
			break ReplicaTimeoutLoop
		}
	}

}


func (p *FlowMC) run() {
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
				case block := <-p.incorporatedBlockEvents:
					 if p.onIncorporatedBlock(block) {
						 break ReplicaTimeoutLoop
					 }
				case qc := <-p.incorporatedQcEvents:
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
		case block := <-p.incorporatedBlockEvents:
			//ToDo: implement replica logic
		case qc := <-p.incorporatedQcEvents:
			//ToDo:
			// - updated highest QC
			// - as Primary: check if qc if for current round and  build block
			// - advance to QC.View + 1
		case <-timer.C:
			ReactorCoreLogger.Debugf("Timeout at View %d", r.currentView)
		}
	}
}

func (p *FlowMC) Run() {
	go p.run()
}



// TimoutController implements a timout with:
// - on timeout: increase timeout by multiplicative factor `timeoutIncrease` (user-specified)
//   this results in exponential growing timeout duration on multiple subsequent timeouts
// - on progress: decrease timeout by subtrahend `timeoutDecrease`
type TimoutController struct {
	// replicaTimeout is the duration of a view before we time out [Milliseconds]
	// replicaTimeout is the only variable quantity
	replicaTimeout float64

	// replicaTimeout is the duration of a view before we time out [Milliseconds]
	minReplicaTimeout float64
	// voteAggregationTimeoutFraction is a fraction of replicaTimeout which is reserved for aggregating votes
	voteAggregationTimeoutFraction float64
	// timeoutDecrease: multiplicative factor for increasing timeout
	timeoutIncrease float64
	// timeoutDecrease linear subtrahend for timeout decrease [Milliseconds]
	timeoutDecrease float64
}

func DefaultTimoutController() *TimoutController {
	return &TimoutController{
		replicaTimeout:                 800,
		minReplicaTimeout:              800,
		voteAggregationTimeoutFraction: 0.5,
		timeoutIncrease:                1.5,
		timeoutDecrease:                500,
	}
}

// NewTimoutController creates a new TimoutController.
// minReplicaTimeout: minimal timeout value for replica round [Milliseconds], also used as starting value;
// voteAggregationTimeoutFraction: fraction of replicaTimeout which is reserved for aggregating votes;
// timeoutIncrease: multiplicative factor for increasing timeout;
// timeoutDecrease: linear subtrahend for timeout decrease [Milliseconds]
func NewTimoutController(minReplicaTimeout, voteAggregationTimeoutFraction, timeoutIncrease, timeoutDecrease float64) *TimoutController {
	if !((0 < voteAggregationTimeoutFraction) && (voteAggregationTimeoutFraction <= 1)){
		panic("VoteAggregationTimeoutFraction must be in range (0,1]")
	}
	if timeoutIncrease <= 1 {
		panic("TimeoutIncrease must be strictly bigger than 1")
	}
	return &TimoutController{
		replicaTimeout:                 minReplicaTimeout,
		minReplicaTimeout:              ensurePositive(minReplicaTimeout, "MinReplicaTimeout"),
		voteAggregationTimeoutFraction: voteAggregationTimeoutFraction,
		timeoutIncrease:                timeoutIncrease,
		timeoutDecrease:                ensurePositive(timeoutDecrease, "TimeoutDecrease"),
	}
}

// ReplicaTimeout returns the duration of the current view before we time out
func (t *TimoutController) ReplicaTimeout() time.Duration {
	return time.Duration(t.replicaTimeout * 1E6)
}

// VoteAggregationTimeout returns the duration of Vote aggregation _after_ receiving a block
// during which the primary tries to aggregate votes for the view where it is leader
func (t *TimoutController) VoteAggregationTimeout() time.Duration {
	// time.Duration expects an int64 as input which specifies the duration in units of nanoseconds (1E-9)
	return time.Duration(t.replicaTimeout * 1E6 * t.voteAggregationTimeoutFraction)
}

// OnTimeout indicates to the TimoutController that the timeout was reached
func (t *TimoutController) OnTimeout()  {
	t.replicaTimeout *= t.timeoutIncrease
}

// OnProgressBeforeTimeout indicates to the TimoutController that progress was made _before_ the timeout was reached
func (t *TimoutController) OnProgressBeforeTimeout()  {
	t.replicaTimeout -= t.timeoutDecrease
	if t.replicaTimeout < t.minReplicaTimeout {
		t.replicaTimeout = t.minReplicaTimeout
	}
}

func ensurePositive(value float64, varName string) float64 {
	if value <= 0 {
		panic(varName + " must be positive")
	}
	return value
}
