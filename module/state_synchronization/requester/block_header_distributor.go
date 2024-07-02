package requester

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

// OnBlockHeaderReceivedConsumer is a callback that is called flow.Header is received for a new indexed block
type OnBlockHeaderReceivedConsumer func(*flow.Header)

// BlockHeaderDistributor subscribes to block header received events from the indexer and
// distributes them to subscribers
type BlockHeaderDistributor struct {
	consumers []OnBlockHeaderReceivedConsumer
	lock      sync.RWMutex
}

// NewBlockHeaderDistributor creates a new instance of BlockHeaderDistributor
func NewBlockHeaderDistributor() *BlockHeaderDistributor {
	return &BlockHeaderDistributor{}
}

// AddOnBlockHeaderReceivedConsumer adds a consumer to be notified when new block header is received
func (p *BlockHeaderDistributor) AddOnBlockHeaderReceivedConsumer(consumer OnBlockHeaderReceivedConsumer) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.consumers = append(p.consumers, consumer)
}

// OnBlockHeaderReceived is called when new block header is received
func (p *BlockHeaderDistributor) OnBlockHeaderReceived(header *flow.Header) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for _, consumer := range p.consumers {
		consumer(header)
	}
}
