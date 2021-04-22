package approvals

import (
	"github.com/onflow/flow-go/model/flow"

	lru "github.com/hashicorp/golang-lru"
)

// ApprovalsCache is a wrapper over `lru.Cache` that provides needed api for processing result approvals
// Extends functionality of `lru.Cache` but mainly using grouping of calls and providing more user friendly interface.
type ApprovalsCache struct {
	cache *lru.Cache
}

func NewApprovalsCache(limit uint) *ApprovalsCache {
	cache, _ := lru.New(int(limit))
	return &ApprovalsCache{
		cache: cache,
	}
}

func (c *ApprovalsCache) TakeIf(approvalID flow.Identifier, predicate func(*flow.ResultApproval) bool) *flow.ResultApproval {
	approvalTmp, cached := c.cache.Peek(approvalID)
	if !cached {
		return nil
	}

	approval := approvalTmp.(*flow.ResultApproval)
	if !predicate(approval) {
		return nil
	}

	c.cache.Remove(approvalID)
	return approval
}

func (c *ApprovalsCache) Take(approvalID flow.Identifier) *flow.ResultApproval {
	approval, cached := c.cache.Peek(approvalID)
	if !cached {
		return nil
	}

	c.cache.Remove(approvalID)
	return approval.(*flow.ResultApproval)
}

func (c *ApprovalsCache) Ids() []flow.Identifier {
	keys := c.cache.Keys()
	result := make([]flow.Identifier, len(keys))
	for i, key := range keys {
		result[i] = key.(flow.Identifier)
	}
	return result
}

func (c *ApprovalsCache) Peek(approvalID flow.Identifier) *flow.ResultApproval {
	// check if we have it in the cache
	resource, cached := c.cache.Peek(approvalID)
	if cached {
		return resource.(*flow.ResultApproval)
	}

	return nil
}

func (c *ApprovalsCache) Get(approvalID flow.Identifier) *flow.ResultApproval {
	// check if we have it in the cache
	resource, cached := c.cache.Get(approvalID)
	if cached {
		return resource.(*flow.ResultApproval)
	}

	return nil
}

func (c *ApprovalsCache) Put(approval *flow.ResultApproval) {
	// cache the resource and eject least recently used one if we reached limit
	_ = c.cache.Add(approval.ID(), approval)
}
