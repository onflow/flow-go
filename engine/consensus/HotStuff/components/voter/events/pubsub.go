package voterEvents

import (
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/utils"
	"sync"

	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/defConAct"
)

// PubSubEventProcessor implements voteAggregator.Processor
// It allows thread-safe subscription to events
type PubSubEventProcessor struct {
	sentVoteConsumers []SentVoteConsumer
	lock              sync.RWMutex
}

func New() *PubSubEventProcessor {
	return &PubSubEventProcessor{}
}

func (p *PubSubEventProcessor) OnSentVote(vote *defConAct.Vote) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.sentVoteConsumers {
		subscriber.OnSentVote(vote)
	}
}

// AddSentVoteConsumer adds a SentVoteConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddSentVoteConsumer(cons SentVoteConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.sentVoteConsumers = append(p.sentVoteConsumers, cons)
	return p
}

