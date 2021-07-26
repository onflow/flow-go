package approvals

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

// ApprovalsCache encapsulates a map for storing result approvals indexed by approval partial ID
// to provide concurrent access.
type ApprovalsCache struct {
	cache map[flow.Identifier]*flow.ResultApproval
	lock  sync.RWMutex
}

func NewApprovalsCache(sizeHint uint) *ApprovalsCache {
	return &ApprovalsCache{
		cache: make(map[flow.Identifier]*flow.ResultApproval, sizeHint),
		lock:  sync.RWMutex{},
	}
}

// Put saves approval into cache; returns true iff approval was newly added
// the approvalID should be calculated as `approval.Body.PartialID()`
// it is taken as an input so that the caller can optimize by reusing the calculated approvalID.
func (c *ApprovalsCache) Put(approvalID flow.Identifier, approval *flow.ResultApproval) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, found := c.cache[approvalID]; !found {
		c.cache[approvalID] = approval
		return true
	}
	return false
}

// Get returns ResultApproval for the given ID (or nil if none is stored)
func (c *ApprovalsCache) Get(approvalID flow.Identifier) *flow.ResultApproval {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.cache[approvalID]
}

// All returns all stored approvals
func (c *ApprovalsCache) All() []*flow.ResultApproval {
	c.lock.RLock()
	defer c.lock.RUnlock()
	all := make([]*flow.ResultApproval, 0, len(c.cache))
	for _, approval := range c.cache {
		all = append(all, approval)
	}
	return all
}

// IncorporatedResultsCache encapsulates a map for storing IncorporatedResults
// to provide concurrent access.
type IncorporatedResultsCache struct {
	cache map[flow.Identifier]*flow.IncorporatedResult
	lock  sync.RWMutex
}

func NewIncorporatedResultsCache(sizeHint uint) *IncorporatedResultsCache {
	return &IncorporatedResultsCache{
		cache: make(map[flow.Identifier]*flow.IncorporatedResult, sizeHint),
		lock:  sync.RWMutex{},
	}
}

// Put saves IncorporatedResult into cache; returns true iff IncorporatedResult was newly added
func (c *IncorporatedResultsCache) Put(key flow.Identifier, incRes *flow.IncorporatedResult) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, found := c.cache[key]; !found {
		c.cache[key] = incRes
		return true
	}
	return false
}

// Get returns IncorporatedResult for the given ID (or nil if none is stored)
func (c *IncorporatedResultsCache) Get(key flow.Identifier) *flow.IncorporatedResult {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.cache[key]
}

// All returns all stored approvals
func (c *IncorporatedResultsCache) All() []*flow.IncorporatedResult {
	c.lock.RLock()
	defer c.lock.RUnlock()
	all := make([]*flow.IncorporatedResult, 0, len(c.cache))
	for _, approval := range c.cache {
		all = append(all, approval)
	}
	return all
}
