package events

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/utils"
	"sync"
)

// PubSubEventProcessor implements pacemaker.events.Processor
// It allows thread-safe subscription to events
type PubSubEventProcessor struct {
	forkChoiceTriggerConsumers []ForkChoiceTriggerConsumer
	enteringViewConsumers      []EnteringViewConsumer
	passiveTillViewConsumers   []PassiveTillViewConsumer
	lock                       sync.RWMutex
}

func NewPubSubEventProcessor() *PubSubEventProcessor {
	return &PubSubEventProcessor{}
}

func (p *PubSubEventProcessor) OnForkChoiceTrigger(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.forkChoiceTriggerConsumers {
		subscriber.OnForkChoiceTrigger(view)
	}
}

func (p *PubSubEventProcessor) OnEnteringView(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.enteringViewConsumers {
		subscriber.OnEnteringView(view)
	}
}

func (p *PubSubEventProcessor) OnPassiveTillView(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.passiveTillViewConsumers {
		subscriber.OnPassiveTillView(view)
	}
}

// AddForkChoiceTriggerConsumer adds an ForkChoiceTriggerConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddForkChoiceTriggerConsumer(cons ForkChoiceTriggerConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.forkChoiceTriggerConsumers = append(p.forkChoiceTriggerConsumers, cons)
	return p
}

// AddEnteringViewConsumer adds an EnteringViewConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddEnteringViewConsumer(cons EnteringViewConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.enteringViewConsumers = append(p.enteringViewConsumers, cons)
	return p
}

// AddEnteringCatchupModeConsumer adds an EnteringCatchupModeConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddEnteringCatchupModeConsumer(cons PassiveTillViewConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.passiveTillViewConsumers = append(p.passiveTillViewConsumers, cons)
	return p
}
