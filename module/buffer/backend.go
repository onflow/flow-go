package buffer

import (
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
)

// item represents an item in the cache: a block header, payload, and the ID
// of the node that sent it to us. The payload is generic.
type item struct {
	originID flow.Identifier
	header   *flow.Header
	payload  interface{}
}

// backend implements a simple cache of pending blocks, indexed by parent ID.
type backend struct {
	mu sync.RWMutex
	// map of pending block IDs, keyed by parent ID for ByParentID lookups
	byParent map[flow.Identifier][]flow.Identifier
	// set of pending blocks, keyed by ID to avoid duplication
	byID map[flow.Identifier]*item
}

// newBackend returns a new pending block cache.
func newBackend() *backend {
	cache := &backend{
		byParent: make(map[flow.Identifier][]flow.Identifier),
		byID:     make(map[flow.Identifier]*item),
	}
	return cache
}

// add adds the item to the cache, returning false if it already exists and
// true otherwise.
func (c *backend) add(originID flow.Identifier, header *flow.Header, payload interface{}) bool {

	c.mu.Lock()
	defer c.mu.Unlock()

	blockID := header.ID()

	_, exists := c.byID[blockID]
	if exists {
		return false
	}

	item := &item{
		header:   header,
		originID: originID,
		payload:  payload,
	}

	c.byID[blockID] = item
	c.byParent[header.ParentID] = append(c.byParent[header.ParentID], blockID)

	return true
}

func (c *backend) ByID(id flow.Identifier) (*item, bool) {

	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.byID[id]
	if !exists {
		return nil, false
	}

	return item, true
}

// byParentID returns a list of cached blocks with the given parent. If no such
// blocks exist, returns false.
func (c *backend) byParentID(parentID flow.Identifier) ([]*item, bool) {

	c.mu.RLock()
	defer c.mu.RUnlock()

	forParent, exists := c.byParent[parentID]
	if !exists {
		return nil, false
	}

	items := make([]*item, 0, len(forParent))
	for _, blockID := range forParent {
		items = append(items, c.byID[blockID])
	}

	return items, true
}

// dropForParent removes all cached blocks with the given parent (non-recursively).
func (c *backend) dropForParent(parentID flow.Identifier) {

	c.mu.Lock()
	defer c.mu.Unlock()

	children, exists := c.byParent[parentID]
	if !exists {
		return
	}

	for _, childID := range children {
		delete(c.byID, childID)
	}
	delete(c.byParent, parentID)
}
