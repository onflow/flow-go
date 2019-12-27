package blockProducerEvents

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/utils"
	"sync"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/def"
)

// PubSubEventProcessor implements BlockProducer.Processor
// It allows thread-safe subscription to events
type PubSubEventProcessor struct {
	blockProducerConsumers []ProducedBlockConsumer
	lock                   sync.RWMutex
}

func New() *PubSubEventProcessor {
	return &PubSubEventProcessor{}
}

func (p *PubSubEventProcessor) OnProducedBlock(block *def.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.blockProducerConsumers {
		subscriber.OnProducedBlock(block)
	}
}

// AddProducedBlockConsumer adds a ProducedBlockConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddProducedBlockConsumer(cons ProducedBlockConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.blockProducerConsumers = append(p.blockProducerConsumers, cons)
	return p
}
