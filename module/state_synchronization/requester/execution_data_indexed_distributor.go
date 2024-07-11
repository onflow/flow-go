package requester

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

// OnBlockHeaderReceivedConsumer is a callback that is called flow.Header is received for a new indexed block
type OnBlockHeaderReceivedConsumer func(*flow.Header)

// ExecutionDataIndexedDistributor subscribes to block header received events from the indexer and
// distributes them to subscribers
type ExecutionDataIndexedDistributor struct {
	consumers []OnBlockHeaderReceivedConsumer
	lock      sync.RWMutex
}

// NewExecutionDataIndexedDistributor creates a new instance of ExecutionDataIndexedDistributor
func NewExecutionDataIndexedDistributor() *ExecutionDataIndexedDistributor {
	return &ExecutionDataIndexedDistributor{}
}

// AddOnBlockIndexedConsumer adds a consumer to be notified when new block header is received
func (p *ExecutionDataIndexedDistributor) AddOnBlockIndexedConsumer(consumer OnBlockHeaderReceivedConsumer) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.consumers = append(p.consumers, consumer)
}

// OnBlockIndexed is called when new block header is received
func (p *ExecutionDataIndexedDistributor) OnBlockIndexed(header *flow.Header) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for _, consumer := range p.consumers {
		consumer(header)
	}
}
