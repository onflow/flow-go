package approvals

import (
	"github.com/onflow/flow-go/model/flow"

	lru "github.com/hashicorp/golang-lru"
)

type ApprovalsCache struct {
	cache *lru.Cache
}

func NewApprovalsCache(limit uint) *ApprovalsCache {
	cache, _ := lru.New(int(limit))
	return &ApprovalsCache{
		cache: cache,
	}
}

func (c *ApprovalsCache) Ids() []flow.Identifier {
	keys := c.cache.Keys()
	result := make([]flow.Identifier, len(keys))
	for i, key := range keys {
		result[i] = key.(flow.Identifier)
	}
	return result
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
