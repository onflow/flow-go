package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Distributor distributes notifications to a list of subscribers (event consumers).
//
// It allows thread-safe subscription of multiple consumers to events.
type Distributor struct {
	subscribers []hotstuff.Consumer
	lock        sync.RWMutex
}

var _ hotstuff.Consumer = &Distributor{}

func (p *Distributor) OnEventProcessed() {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnEventProcessed()
	}
}

func NewDistributor() *Distributor {
	return &Distributor{}
}

// AddConsumer adds an a event consumer to the Distributor
func (p *Distributor) AddConsumer(consumer hotstuff.Consumer) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.subscribers = append(p.subscribers, consumer)
}

func (p *Distributor) OnReceiveVote(currentView uint64, vote *model.Vote) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnReceiveVote(currentView, vote)
	}
}

func (p *Distributor) OnReceiveProposal(currentView uint64, proposal *model.Proposal) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnReceiveProposal(currentView, proposal)
	}
}

func (p *Distributor) OnEnteringView(view uint64, leader flow.Identifier) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnEnteringView(view, leader)
	}
}

func (p *Distributor) OnQcTriggeredViewChange(qc *flow.QuorumCertificate, newView uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnQcTriggeredViewChange(qc, newView)
	}
}

func (p *Distributor) OnProposingBlock(proposal *model.Proposal) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnProposingBlock(proposal)
	}
}

func (p *Distributor) OnVoting(vote *model.Vote) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnVoting(vote)
	}
}

func (p *Distributor) OnQcConstructedFromVotes(qc *flow.QuorumCertificate) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnQcConstructedFromVotes(qc)
	}
}

func (p *Distributor) OnStartingTimeout(timerInfo *model.TimerInfo) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnStartingTimeout(timerInfo)
	}
}

func (p *Distributor) OnReachedTimeout(timeout *model.TimerInfo) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnReachedTimeout(timeout)
	}
}

func (p *Distributor) OnQcIncorporated(qc *flow.QuorumCertificate) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnQcIncorporated(qc)
	}
}

func (p *Distributor) OnForkChoiceGenerated(curView uint64, selectedQC *flow.QuorumCertificate) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnForkChoiceGenerated(curView, selectedQC)
	}
}

func (p *Distributor) OnBlockIncorporated(block *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnBlockIncorporated(block)
	}
}

func (p *Distributor) OnFinalizedBlock(block *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnFinalizedBlock(block)
	}
}

func (p *Distributor) OnDoubleProposeDetected(block1, block2 *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnDoubleProposeDetected(block1, block2)
	}
}

func (p *Distributor) OnDoubleVotingDetected(vote1, vote2 *model.Vote) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnDoubleVotingDetected(vote1, vote2)
	}
}

func (p *Distributor) OnInvalidVoteDetected(vote *model.Vote) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnInvalidVoteDetected(vote)
	}
}

func (p *Distributor) OnVoteForInvalidBlockDetected(vote *model.Vote, invalidProposal *model.Proposal) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnVoteForInvalidBlockDetected(vote, invalidProposal)
	}
}
