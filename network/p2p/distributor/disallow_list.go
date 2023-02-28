package distributor

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
)

const (
	// disallowListDistributorWorkerCount is the number of workers used to process disallow list updates.
	disallowListDistributorWorkerCount = 1

	// DefaultDisallowListNotificationQueueCacheSize is the default size of the disallow list notification queue.
	DefaultDisallowListNotificationQueueCacheSize = 100
)

// DisallowListDistributor is a component that distributes disallow list updates to registered consumers in a
// non-blocking manner.
type DisallowListDistributor struct {
	subscribers []p2p.DisallowListConsumer
	mu          sync.RWMutex
}

var _ p2p.DisallowListConsumer = (*DisallowListDistributor)(nil)

func NewDisallowListDistributor() *DisallowListDistributor {
	return &DisallowListDistributor{}
}

// AddConsumer registers a consumer with the distributor. The distributor will call the consumer's
// OnNodeDisallowListUpdate method when the node disallow list is updated.
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
func (d *DisallowListDistributor) AddConsumer(consumer p2p.DisallowListConsumer) {
	d.mu.Lock()
	d.subscribers = append(d.subscribers, consumer)
	d.mu.Unlock()
}

// OnNodeDisallowListUpdate is called when the node disallow list is updated. It propagates the
// event to the subscriber, maintaining partial order of events.
func (d *DisallowListDistributor) OnNodeDisallowListUpdate(disallowList flow.IdentifierList) {
	d.mu.RLock()
	for _, sub := range d.subscribers {
		sub.OnNodeDisallowListUpdate(disallowList)
	}
	d.mu.RUnlock()
}
