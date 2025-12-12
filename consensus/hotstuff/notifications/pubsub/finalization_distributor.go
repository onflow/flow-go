package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

type OnBlockFinalizedConsumer = func(block *model.Block)
type OnBlockIncorporatedConsumer = func(block *model.Block)

// FinalizationDistributor ingests events from HotStuff's logic for tracking forks + finalization
// and distributes them to consumers. This logic generally runs inside all nodes (irrespectively whether
// they are active consensus participants or only consensus followers).
// Concurrently safe.
type FinalizationDistributor struct {
	blockFinalizedConsumers    []OnBlockFinalizedConsumer
	blockIncorporatedConsumers []OnBlockIncorporatedConsumer
	lock                       sync.RWMutex
}

var _ hotstuff.FinalizationConsumer = (*FinalizationDistributor)(nil)
var _ hotstuff.FinalizationRegistrar = (*FinalizationDistributor)(nil)

func NewFinalizationDistributor() *FinalizationDistributor {
	return &FinalizationDistributor{}
}

func (d *FinalizationDistributor) AddOnBlockFinalizedConsumer(consumer OnBlockFinalizedConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.blockFinalizedConsumers = append(d.blockFinalizedConsumers, consumer)
}

func (d *FinalizationDistributor) AddOnBlockIncorporatedConsumer(consumer OnBlockIncorporatedConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.blockIncorporatedConsumers = append(d.blockIncorporatedConsumers, consumer)
}

func (d *FinalizationDistributor) AddFinalizationConsumer(consumer hotstuff.FinalizationConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.blockFinalizedConsumers = append(d.blockFinalizedConsumers, consumer.OnFinalizedBlock)
	d.blockIncorporatedConsumers = append(d.blockIncorporatedConsumers, consumer.OnBlockIncorporated)
}

func (d *FinalizationDistributor) OnBlockIncorporated(block *model.Block) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, consumer := range d.blockIncorporatedConsumers {
		consumer(block)
	}
}

func (d *FinalizationDistributor) OnFinalizedBlock(block *model.Block) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, consumer := range d.blockFinalizedConsumers {
		consumer(block)
	}
}
