package approvals

import (
	"sync"

	"github.com/hashicorp/golang-lru/simplelru"

	"github.com/onflow/flow-go/model/flow"
)

// ApprovalsCache is a wrapper over `simplelru.LRUCache` that provides needed api for processing result approvals
// Extends functionality of `simplelru.LRUCache` by introducing additional index for quicker access.
type ApprovalsCache struct {
	lru  simplelru.LRUCache
	lock sync.RWMutex
	// secondary index by result id, since multiple approvals could
	// reference same result
	byResultID map[flow.Identifier]flow.IdentifierList
}

func NewApprovalsCache(limit uint) *ApprovalsCache {
	byResultID := make(map[flow.Identifier]flow.IdentifierList)
	// callback has to be called while we are holding lock
	lru, _ := simplelru.NewLRU(int(limit), func(key interface{}, value interface{}) {
		approval := value.(*flow.ResultApproval)
		delete(byResultID, approval.Body.ExecutionResultID)
	})
	return &ApprovalsCache{
		lru:        lru,
		byResultID: byResultID,
	}
}

func (c *ApprovalsCache) ByResultID(resultID flow.Identifier) flow.IdentifierList {
	c.lock.RLock()
	defer c.lock.RUnlock()
	ids, ok := c.byResultID[resultID]
	if ok {
		return ids.Copy()
	}
	return nil
}

func (c *ApprovalsCache) Peek(approvalID flow.Identifier) *flow.ResultApproval {
	c.lock.RLock()
	defer c.lock.RUnlock()
	// check if we have it in the cache
	resource, cached := c.lru.Peek(approvalID)
	if cached {
		return resource.(*flow.ResultApproval)
	}

	return nil
}

func (c *ApprovalsCache) Get(approvalID flow.Identifier) *flow.ResultApproval {
	c.lock.Lock()
	defer c.lock.Unlock()
	// check if we have it in the cache
	resource, cached := c.lru.Get(approvalID)
	if cached {
		return resource.(*flow.ResultApproval)
	}

	return nil
}

func (c *ApprovalsCache) Take(approvalID flow.Identifier) *flow.ResultApproval {
	c.lock.Lock()
	defer c.lock.Unlock()

	// check if we have it in the cache
	if resource, ok := c.lru.Peek(approvalID); ok {
		_ = c.lru.Remove(approvalID)
		return resource.(*flow.ResultApproval)
	}

	return nil
}

func (c *ApprovalsCache) Put(approval *flow.ResultApproval) {
	approvalID := approval.ID()
	resultID := approval.Body.ExecutionResultID
	c.lock.Lock()
	defer c.lock.Unlock()
	// cache the resource and eject least recently used one if we reached limit
	_ = c.lru.Add(approvalID, approval)
	ids, ok := c.byResultID[resultID]
	if !ok {
		c.byResultID[resultID] = []flow.Identifier{approvalID}
	} else {
		c.byResultID[resultID] = append(ids, approvalID)
	}
}
