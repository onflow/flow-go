package reactor

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/components/reactor/core"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/components/reactor/forkchoice"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/def"
	"github.com/juju/loggo"
)

var ConsensusLogger loggo.Logger

type Reactor struct {
	forkchoice forkchoice.ForkChoice

	forkchoiceRequests chan uint64
	newQCs             chan *def.QuorumCertificate
	newBlockProposals  chan *def.Block
}

func NewReactor(finalizer *core.ReactorCore, forkchoice forkchoice.ForkChoice) *Reactor {
	return &Reactor{
		forkchoice:         forkchoice,
		forkchoiceRequests: make(chan uint64, 10),
		newQCs:             make(chan *def.QuorumCertificate, 10),
		newBlockProposals:  make(chan *def.Block, 300),
	}
}

func (r *Reactor) OnForkChoiceTrigger(view uint64) {
	// inspired by https://content.pivotal.io/blog/a-channel-based-ring-buffer-in-go
	select {
	case r.forkchoiceRequests <- view:
	default:
		<-r.forkchoiceRequests
		r.forkchoiceRequests <- view
	}
}

func (r *Reactor) OnQcFromVotes(qc *def.QuorumCertificate) {
	select {
	case r.newQCs <- qc:
	default:
		<-r.newQCs
		r.newQCs <- qc
	}
}

func (r *Reactor) OnReceivedBlockProposal(block *def.Block) {
	// inspired by https://content.pivotal.io/blog/a-channel-based-ring-buffer-in-go
	select {
	case r.newBlockProposals <- block:
	default:
		<-r.newBlockProposals
		r.newBlockProposals <- block
	}
}

func (r *Reactor) Run() {
	go r.run()
}

func (r *Reactor) run() {
	for {
		select {
		case view := <-r.forkchoiceRequests:
			r.forkchoice.OnForkChoiceTrigger(view)
		case qc := <-r.newQCs:
			r.forkchoice.ProcessQcFromVotes(qc)
		case block := <-r.newBlockProposals:
			if r.forkchoice.IsProcessingNeeded(block.BlockMRH, block.View) {
				r.forkchoice.ProcessBlock(block)
			}
		}
	}
}
