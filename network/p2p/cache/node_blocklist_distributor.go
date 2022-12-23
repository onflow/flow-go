package cache

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
)

type OnNodeBlockListConsumer = func(blockList flow.IdentifierList)

// NodeBlockListDistributor subscribes to changes in the NodeBlocklistWrapper block list.
type NodeBlockListDistributor struct {
	nodeBlockListConsumers []OnNodeBlockListConsumer
	lock                   sync.RWMutex
}

var _ p2p.NodeBlockListConsumer = (*NodeBlockListDistributor)(nil)

func NewNodeBlockListDistributor() *NodeBlockListDistributor {
	return &NodeBlockListDistributor{
		nodeBlockListConsumers: make([]OnNodeBlockListConsumer, 0),
	}
}

func (n *NodeBlockListDistributor) AddConsumer(consumer OnNodeBlockListConsumer) {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.nodeBlockListConsumers = append(n.nodeBlockListConsumers, consumer)
}

func (n *NodeBlockListDistributor) OnNodeBlockListUpdate(blockList flow.IdentifierList) {
	n.lock.RLock()
	defer n.lock.RUnlock()
	for _, consumer := range n.nodeBlockListConsumers {
		consumer(blockList)
	}
}
