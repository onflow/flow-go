package notifications

import (
	"sync"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/utils"
)

// PubSubDistributor implements notifications.Distributor
// It allows thread-safe subscription of consumers to events
type PubSubDistributor struct {
	skippedAheadConsumers         []SkippedAheadConsumer
	enteringViewConsumers         []EnteringViewConsumer
	startingBlockTimeoutConsumers []StartingBlockTimeoutConsumer
	startingVoteTimeoutConsumers  []StartingVoteTimeoutConsumer
	lock                          sync.RWMutex
}

func NewPubSubDistributor() Distributor {
	return &PubSubDistributor{}
}

func (p *PubSubDistributor) OnSkippedAhead(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.skippedAheadConsumers {
		subscriber.OnSkippedAhead(view)
	}
}

func (p *PubSubDistributor) OnEnteringView(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.enteringViewConsumers {
		subscriber.OnEnteringView(view)
	}
}

func (p *PubSubDistributor) OnStartingBlockTimeout(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.startingBlockTimeoutConsumers {
		subscriber.OnStartingBlockTimeout(view)
	}
}

func (p *PubSubDistributor) OnStartingVotesTimeout(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.startingVoteTimeoutConsumers {
		subscriber.OnStartingVotesTimeout(view)
	}
}

// AddSkippedAheadConsumer adds an SkippedAheadConsumer to the PubSubDistributor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubDistributor) AddSkippedAheadConsumer(cons SkippedAheadConsumer) *PubSubDistributor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.skippedAheadConsumers = append(p.skippedAheadConsumers, cons)
	return p
}

// AddEnteringViewConsumer adds an EnteringViewConsumer to the PubSubDistributor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubDistributor) AddEnteringViewConsumer(cons EnteringViewConsumer) *PubSubDistributor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.enteringViewConsumers = append(p.enteringViewConsumers, cons)
	return p
}

// AddStartingBlockTimeoutConsumer adds an StartingBlockTimeoutConsumer to the PubSubDistributor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubDistributor) AddStartingBlockTimeoutConsumer(cons StartingBlockTimeoutConsumer) *PubSubDistributor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.startingBlockTimeoutConsumers = append(p.startingBlockTimeoutConsumers, cons)
	return p
}

// AddStartingVoteTimeoutConsumer adds an StartingVoteTimeoutConsumer to the PubSubDistributor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubDistributor) AddStartingVoteTimeoutConsumer(cons StartingVoteTimeoutConsumer) *PubSubDistributor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.startingVoteTimeoutConsumers = append(p.startingVoteTimeoutConsumers, cons)
	return p
}
