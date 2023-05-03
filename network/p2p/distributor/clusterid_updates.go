package distributor

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
)

// ClusterIDUpdateDistributor is a component that distributes cluster ID updates to registered consumers.
// It is thread-safe and can be used concurrently from multiple goroutines.
type ClusterIDUpdateDistributor struct {
	lock      sync.RWMutex // protects the consumer field from concurrent updates
	consumers []p2p.ClusterIDUpdateConsumer
}

var _ p2p.ClusterIDUpdateDistributor = (*ClusterIDUpdateDistributor)(nil)

// DistributeClusterIDUpdate distributes the event to all the consumers.
func (c *ClusterIDUpdateDistributor) DistributeClusterIDUpdate(clusterIDS flow.ChainIDList) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	for _, consumer := range c.consumers {
		consumer.OnClusterIdsUpdated(clusterIDS)
	}
}

// AddConsumer adds a consumer to the distributor. The consumer will be called when the distributor distributes a new event.
func (c *ClusterIDUpdateDistributor) AddConsumer(consumer p2p.ClusterIDUpdateConsumer) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.consumers = append(c.consumers, consumer)
}

// NewClusterIDUpdateDistributor returns a new *ClusterIDUpdateDistributor.
func NewClusterIDUpdateDistributor() *ClusterIDUpdateDistributor {
	return &ClusterIDUpdateDistributor{
		lock:      sync.RWMutex{},
		consumers: make([]p2p.ClusterIDUpdateConsumer, 0),
	}
}
