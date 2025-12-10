package ingestion

import (
	"sync"

	"github.com/onflow/flow-go/engine/execution/storehouse"
)

// BlockExecutedNotifier is a thread-safe event distributor that notifies subscribers
// when blocks have been executed. It allows multiple consumers to subscribe to block execution events.
type BlockExecutedNotifier struct {
	consumers []storehouse.BlockExecutedConsumer
	mu        sync.RWMutex
}

// Ensure BlockExecutedNotifier implements storehouse.BlockExecutedNotifier
var _ storehouse.BlockExecutedNotifier = (*BlockExecutedNotifier)(nil)

// NewBlockExecutedNotifier creates a new BlockExecutedNotifier.
func NewBlockExecutedNotifier() *BlockExecutedNotifier {
	return &BlockExecutedNotifier{}
}

// AddConsumer adds a consumer to be notified when blocks are executed.
// This method is thread-safe.
func (n *BlockExecutedNotifier) AddConsumer(consumer storehouse.BlockExecutedConsumer) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.consumers = append(n.consumers, consumer)
}

// OnExecuted notifies all registered consumers that a block has been executed.
// This method is thread-safe and should be called from the ingestion machine.
func (n *BlockExecutedNotifier) OnExecuted() {
	n.mu.RLock()
	defer n.mu.RUnlock()
	for _, consumer := range n.consumers {
		consumer.OnExecuted()
	}
}
