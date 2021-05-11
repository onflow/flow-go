package approvals

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

// Cache is a utility structure that encapsulates map that stores result approvals
// and provides concurrent access to it.
type Cache struct {
	cache map[flow.Identifier]*flow.ResultApproval
	lock  sync.RWMutex
}

func NewApprovalsCache(capacity uint) *Cache {
	return &Cache{
		cache: make(map[flow.Identifier]*flow.ResultApproval, capacity),
		lock:  sync.RWMutex{},
	}
}

// Put saves approval into cache
func (c *Cache) Put(approval *flow.ResultApproval) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache[approval.Body.PartialID()] = approval
}

// Get returns approval that is saved in cache
func (c *Cache) Get(approvalID flow.Identifier) *flow.ResultApproval {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.cache[approvalID]
}

// All returns all stored approvals
func (c *Cache) All() []*flow.ResultApproval {
	c.lock.RLock()
	defer c.lock.RUnlock()
	all := make([]*flow.ResultApproval, 0, len(c.cache))
	for _, approval := range c.cache {
		all = append(all, approval)
	}
	return all
}
