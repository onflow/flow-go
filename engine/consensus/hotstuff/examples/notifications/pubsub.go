package notifications

import (
	"sync"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications"
)

// PubSubDistributor implements notifications.Distributor
// It allows thread-safe subscription of consumers to events
type PubSubDistributor struct {
	skippedAheadConsumers         []SkippedAheadConsumer
	enteringViewConsumers         []EnteringViewConsumer
	startingBlockTimeoutConsumers []StartingBlockTimeoutConsumer
	reachedBlockTimeoutConsumers  []ReachedBlockTimeoutConsumer
	startingVotesTimeoutConsumers []StartingVotesTimeoutConsumer
	reachedVotesTimeoutConsumers  []ReachedVotesTimeoutConsumer
	lock                          sync.RWMutex
}

func NewPubSubDistributor() notifications.Distributor {
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

func (p *PubSubDistributor) OnReachedBlockTimeout(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.reachedBlockTimeoutConsumers {
		subscriber.OnReachedBlockTimeout(view)
	}
}

func (p *PubSubDistributor) OnStartingVotesTimeout(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.startingVotesTimeoutConsumers {
		subscriber.OnStartingVotesTimeout(view)
	}
}

func (p *PubSubDistributor) OnReachedVotesTimeout(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.reachedVotesTimeoutConsumers {
		subscriber.OnReachedVotesTimeout(view)
	}
}

// AddSkippedAheadConsumer adds an SkippedAheadConsumer to the PubSubDistributor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubDistributor) AddSkippedAheadConsumer(cons SkippedAheadConsumer) *PubSubDistributor {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.skippedAheadConsumers = append(p.skippedAheadConsumers, cons)
	return p
}

// AddEnteringViewConsumer adds an EnteringViewConsumer to the PubSubDistributor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubDistributor) AddEnteringViewConsumer(cons EnteringViewConsumer) *PubSubDistributor {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.enteringViewConsumers = append(p.enteringViewConsumers, cons)
	return p
}

// AddStartingBlockTimeoutConsumer adds an StartingBlockTimeoutConsumer to the PubSubDistributor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubDistributor) AddStartingBlockTimeoutConsumer(cons StartingBlockTimeoutConsumer) *PubSubDistributor {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.startingBlockTimeoutConsumers = append(p.startingBlockTimeoutConsumers, cons)
	return p
}

// AddReachedBlockTimeoutConsumer adds an StartingBlockTimeoutConsumer to the PubSubDistributor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubDistributor) AddReachedBlockTimeoutConsumer(cons ReachedBlockTimeoutConsumer) *PubSubDistributor {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.reachedBlockTimeoutConsumers = append(p.reachedBlockTimeoutConsumers, cons)
	return p
}

// AddStartingVotesTimeoutConsumer adds an StartingVoteTimeoutConsumer to the PubSubDistributor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubDistributor) AddStartingVotesTimeoutConsumer(cons StartingVotesTimeoutConsumer) *PubSubDistributor {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.startingVotesTimeoutConsumers = append(p.startingVotesTimeoutConsumers, cons)
	return p
}

// AddReachedVotesTimeoutConsumer adds an StartingVoteTimeoutConsumer to the PubSubDistributor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubDistributor) AddReachedVotesTimeoutConsumer(cons ReachedVotesTimeoutConsumer) *PubSubDistributor {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.reachedVotesTimeoutConsumers = append(p.reachedVotesTimeoutConsumers, cons)
	return p
}
