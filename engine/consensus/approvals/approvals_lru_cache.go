package approvals

import (
	"sync"

	"github.com/hashicorp/golang-lru/simplelru"

	"github.com/onflow/flow-go/model/flow"
)

// LruCache is a wrapper over `simplelru.LRUCache` that provides needed api for processing result approvals
// Extends functionality of `simplelru.LRUCache` by introducing additional index for quicker access.
type LruCache struct {
	lru  simplelru.LRUCache
	lock sync.RWMutex
	// secondary index by result id, since multiple approvals could
	// reference same result
	byResultID map[flow.Identifier]map[flow.Identifier]struct{}
}

func NewApprovalsLRUCache(limit uint) *LruCache {
	byResultID := make(map[flow.Identifier]map[flow.Identifier]struct{})
	// callback has to be called while we are holding lock
	lru, _ := simplelru.NewLRU(int(limit), func(key interface{}, value interface{}) {
		approval := value.(*flow.ResultApproval)
		delete(byResultID[approval.Body.ExecutionResultID], approval.Body.PartialID())
		if len(byResultID[approval.Body.ExecutionResultID]) == 0 {
			delete(byResultID, approval.Body.ExecutionResultID)
		}
	})
	return &LruCache{
		lru:        lru,
		byResultID: byResultID,
	}
}

func (c *LruCache) Peek(approvalID flow.Identifier) *flow.ResultApproval {
	c.lock.RLock()
	defer c.lock.RUnlock()
	// check if we have it in the cache
	resource, cached := c.lru.Peek(approvalID)
	if cached {
		return resource.(*flow.ResultApproval)
	}

	return nil
}

func (c *LruCache) Get(approvalID flow.Identifier) *flow.ResultApproval {
	c.lock.Lock()
	defer c.lock.Unlock()
	// check if we have it in the cache
	resource, cached := c.lru.Get(approvalID)
	if cached {
		return resource.(*flow.ResultApproval)
	}

	return nil
}

func (c *LruCache) TakeByResultID(resultID flow.Identifier) []*flow.ResultApproval {
	c.lock.Lock()
	defer c.lock.Unlock()

	ids, ok := c.byResultID[resultID]
	if !ok {
		return nil
	}

	approvals := make([]*flow.ResultApproval, 0, len(ids))
	for approvalID := range ids {
		// check if we have it in the cache
		if resource, ok := c.lru.Peek(approvalID); ok {
			// no need to cleanup secondary index since it will be
			// cleaned up in evict callback
			_ = c.lru.Remove(approvalID)
			approvals = append(approvals, resource.(*flow.ResultApproval))
		}
	}

	return approvals
}

func (c *LruCache) Put(approval *flow.ResultApproval) {
	approvalID := approval.Body.PartialID()
	resultID := approval.Body.ExecutionResultID
	c.lock.Lock()
	defer c.lock.Unlock()
	// cache the resource and eject least recently used one if we reached limit
	_ = c.lru.Add(approvalID, approval)
	_, ok := c.byResultID[resultID]
	if !ok {
		c.byResultID[resultID] = map[flow.Identifier]struct{}{approvalID: {}}
	} else {
		c.byResultID[resultID][approvalID] = struct{}{}
	}
}
