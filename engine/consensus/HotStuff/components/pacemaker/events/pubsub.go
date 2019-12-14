package events

import (
	"reflect"
	"sync"
)

// PubSubEventProcessor implements pacemaker.events.Processor
// It allows thread-safe subscription to events
type PubSubEventProcessor struct {
	blockProposalTriggerCons []BlockProposalTriggerConsumer
	enteringViewCons         []EnteringViewConsumer
	enteringCatchupModeCons  []EnteringCatchupModeConsumer
	enteringVotingModeCons   []EnteringVotingModeConsumer
	lock                     sync.RWMutex
}

func NewPubSubEventProcessor() *PubSubEventProcessor {
	return &PubSubEventProcessor{}
}

func (p *PubSubEventProcessor) OnBlockProposalTrigger(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.blockProposalTriggerCons {
		subscriber.OnBlockProposalTrigger(view)
	}
}

func (p *PubSubEventProcessor) OnEnteringView(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.enteringViewCons {
		subscriber.OnEnteringView(view)
	}
}

func (p *PubSubEventProcessor) OnEnteringCatchupMode(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.enteringCatchupModeCons {
		subscriber.OnEnteringCatchupMode(view)
	}
}

func (p *PubSubEventProcessor) OnEnteringVotingMode(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.enteringVotingModeCons {
		subscriber.OnEnteringVotingMode(view)
	}
}

// AddBlockProposalTrigger adds an BlockProposalTriggerConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddBlockProposalTrigger(cons BlockProposalTriggerConsumer) *PubSubEventProcessor {
	ensureNotNil(cons)
	p.lock.Lock()
	defer p.lock.Unlock()
	p.blockProposalTriggerCons = append(p.blockProposalTriggerCons, cons)
	return p
}

// AddEnteringViewConsumer adds an EnteringViewConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddEnteringViewConsumer(cons EnteringViewConsumer) *PubSubEventProcessor {
	ensureNotNil(cons)
	p.lock.Lock()
	defer p.lock.Unlock()
	p.enteringViewCons = append(p.enteringViewCons, cons)
	return p
}

// AddEnteringCatchupModeConsumer adds an EnteringCatchupModeConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddEnteringCatchupModeConsumer(cons EnteringCatchupModeConsumer) *PubSubEventProcessor {
	ensureNotNil(cons)
	p.lock.Lock()
	defer p.lock.Unlock()
	p.enteringCatchupModeCons = append(p.enteringCatchupModeCons, cons)
	return p
}

// AddEnteringVotingModeConsumer adds an EnteringVotingModeConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddEnteringVotingModeConsumer(cons EnteringVotingModeConsumer) *PubSubEventProcessor {
	ensureNotNil(cons)
	p.lock.Lock()
	defer p.lock.Unlock()
	p.enteringVotingModeCons = append(p.enteringVotingModeCons, cons)
	return p
}

func ensureNotNil(cons interface{}) {
	if cons == nil || reflect.ValueOf(cons).IsNil() {
		panic("Processor cannot be nil")
	}
}
