package events

import (
	"sync"

	"github.com/onflow/flow-go/engine/collection"
	"github.com/onflow/flow-go/model/flow"
)

// ClusterEventsDistributor distributes cluster events to a list of subscribers.
type ClusterEventsDistributor struct {
	subscribers []collection.ClusterEvents
	mu          sync.RWMutex
}

var _ collection.ClusterEvents = (*ClusterEventsDistributor)(nil)

// NewClusterEventsDistributor returns a new events *ClusterEventsDistributor.
func NewClusterEventsDistributor() *ClusterEventsDistributor {
	return &ClusterEventsDistributor{}
}

func (d *ClusterEventsDistributor) AddConsumer(consumer collection.ClusterEvents) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.subscribers = append(d.subscribers, consumer)
}

// ActiveClustersChanged distributes events to all subscribers.
func (d *ClusterEventsDistributor) ActiveClustersChanged(list flow.ChainIDList) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, sub := range d.subscribers {
		sub.ActiveClustersChanged(list)
	}
}
