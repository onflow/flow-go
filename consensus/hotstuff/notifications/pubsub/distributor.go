package pubsub

import (
	"sync"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
)

// Distributor distributes notifications to a list of subscribers (event consumers).
//
// It allows thread-safe subscription of multiple consumers to events.
type Distributor struct {
	subscribers []hotstuff.Consumer
	lock        sync.RWMutex
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

func (p *Distributor) OnSkippedAhead(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnSkippedAhead(view)
	}
}

func (p *Distributor) OnEnteringView(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnEnteringView(view)
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

func (p *Distributor) OnQcIncorporated(qc *model.QuorumCertificate) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnQcIncorporated(qc)
	}
}

func (p *Distributor) OnForkChoiceGenerated(curView uint64, selectedQC *model.QuorumCertificate) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnForkChoiceGenerated(curView, selectedQC)
	}
}

func (p *Distributor) OnBlockProposed(proposal *model.Proposal) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnBlockProposed(proposal)
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
