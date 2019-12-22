package events

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/utils"
	"sync"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/def"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/defConAct"
)

// PubSubEventProcessor implements voteAggregator.Processor
// It allows thread-safe subscription to events
type PubSubEventProcessor struct {
	doubleVoteConsumers  []DoubleVoteConsumer
	invalidVoteConsumers []InvalidVoteConsumer
	qcFromVotesConsumers []QcFromVotesConsumer
	lock                 sync.RWMutex
}

func New() *PubSubEventProcessor {
	return &PubSubEventProcessor{}
}

func (p *PubSubEventProcessor) OnDoubleVote(exhibitA *defConAct.Vote, exhibitB *defConAct.Vote) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.doubleVoteConsumers {
		subscriber.OnDoubleVote(exhibitA, exhibitB)
	}
}

func (p *PubSubEventProcessor) OnInvalidVote(vote *defConAct.Vote) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.invalidVoteConsumers {
		subscriber.OnInvalidVote(vote)
	}
}

func (p *PubSubEventProcessor) OnQcFromVotes(qc *def.QuorumCertificate) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.qcFromVotesConsumers {
		subscriber.OnQcFromVotes(qc)
	}
}

// AddDoubleVoteConsumer adds a DoubleVoteConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddDoubleVoteConsumer(cons DoubleVoteConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.doubleVoteConsumers = append(p.doubleVoteConsumers, cons)
	return p
}

// AddInvalidVoteConsumer adds a InvalidVoteConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddInvalidVoteConsumer(cons InvalidVoteConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.invalidVoteConsumers = append(p.invalidVoteConsumers, cons)
	return p
}

// AddQcFromVotesConsumer adds a QcFromVotesConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddQcFromVotesConsumer(cons QcFromVotesConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.qcFromVotesConsumers = append(p.qcFromVotesConsumers, cons)
	return p
}
