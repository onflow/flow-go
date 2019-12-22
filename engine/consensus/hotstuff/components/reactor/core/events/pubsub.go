package events

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/utils"
	"sync"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/def"
)

// PubSubEventProcessor implements core.Processor
// It allows thread-safe subscription to events
type PubSubEventProcessor struct {
	missingBlockCons      []MissingBlockConsumer
	incorporatedBlockCons []IncorporatedBlockConsumer
	safeBlockCons         []SafeBlockConsumer
	finalizedBlockCons    []FinalizedConsumer
	doubleProposeCons     []DoubleProposalConsumer
	lock                  sync.RWMutex
}

func NewPubSubEventProcessor() *PubSubEventProcessor {
	return &PubSubEventProcessor{}
}

func (p *PubSubEventProcessor) OnMissingBlock(hash []byte, view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.missingBlockCons {
		subscriber.OnMissingBlock(hash, view)
	}
}

func (p *PubSubEventProcessor) OnBlockIncorporated(block *def.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.incorporatedBlockCons {
		subscriber.OnIncorporatedBlock(block)
	}
}

func (p *PubSubEventProcessor) OnSafeBlock(block *def.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.safeBlockCons {
		subscriber.OnSafeBlock(block)
	}
}

func (p *PubSubEventProcessor) OnFinalizedBlock(block *def.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.finalizedBlockCons {
		subscriber.OnFinalizedBlock(block)
	}
}

func (p *PubSubEventProcessor) OnDoubleProposeDetected(block1, block2 *def.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.doubleProposeCons {
		subscriber.OnDoubleProposeDetected(block1, block2)
	}
}

// AddMissingBlockConsumer adds a MissingBlockConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddMissingBlockConsumer(cons MissingBlockConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.missingBlockCons = append(p.missingBlockCons, cons)
	return p
}

// AddIncorporatedBlockConsumer adds a IncorporatedBlockConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddIncorporatedBlockConsumer(cons IncorporatedBlockConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.incorporatedBlockCons = append(p.incorporatedBlockCons, cons)
	return p
}

// AddSafeBlockConsumer adds a SafeBlockConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddSafeBlockConsumer(cons SafeBlockConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.safeBlockCons = append(p.safeBlockCons, cons)
	return p
}

// AddFinalizedConsumer adds a FinalizedConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddFinalizedConsumer(cons FinalizedConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.finalizedBlockCons = append(p.finalizedBlockCons, cons)
	return p
}

// AddDoubleProposalConsumer adds a DoubleProposalConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddDoubleProposalConsumer(cons DoubleProposalConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.doubleProposeCons = append(p.doubleProposeCons, cons)
	return p
}
