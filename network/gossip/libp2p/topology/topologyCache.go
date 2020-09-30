package topology

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

type TopologyCache interface {
	Topology
	Invalidate()
}

type TopologyCacheImpl struct {
	sync.Mutex
	cachedList flow.IdentityList
	Topology
}

func NewTopologyCacheImpl(topology Topology) *TopologyCacheImpl {
	return &TopologyCacheImpl{
		Topology: topology,
	}
}
func (cache *TopologyCacheImpl) Invalidate() {
	cache.Lock()
	defer cache.Unlock()
	cache.cachedList = nil
}

func (cache *TopologyCacheImpl) Subset(idList flow.IdentityList, fanout uint) (flow.IdentityList, error) {
	cache.Lock()
	defer cache.Unlock()

	// if previous result was cached, return it
	if cache.cachedList != nil {
		return cache.cachedList, nil
	}
	// else derive new subset
	subset, err := cache.Topology.Subset(idList, fanout)
	if err != nil {
		cache.cachedList = nil
		return nil, err
	}
	// cache new subset
	cache.cachedList = subset
	// return new subset
	return subset, nil
}


