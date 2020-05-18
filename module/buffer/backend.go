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
func (b *backend) add(originID flow.Identifier, header *flow.Header, payload interface{}) bool {

	b.mu.Lock()
	defer b.mu.Unlock()

	blockID := header.ID()

	_, exists := b.byID[blockID]
	if exists {
		return false
	}

	item := &item{
		header:   header,
		originID: originID,
		payload:  payload,
	}

	b.byID[blockID] = item
	b.byParent[header.ParentID] = append(b.byParent[header.ParentID], blockID)

	return true
}

func (b *backend) ByID(id flow.Identifier) (*item, bool) {

	b.mu.RLock()
	defer b.mu.RUnlock()

	item, exists := b.byID[id]
	if !exists {
		return nil, false
	}

	return item, true
}

// byParentID returns a list of cached blocks with the given parent. If no such
// blocks exist, returns false.
func (b *backend) byParentID(parentID flow.Identifier) ([]*item, bool) {

	b.mu.RLock()
	defer b.mu.RUnlock()

	forParent, exists := b.byParent[parentID]
	if !exists {
		return nil, false
	}

	items := make([]*item, 0, len(forParent))
	for _, blockID := range forParent {
		items = append(items, b.byID[blockID])
	}

	return items, true
}

// dropForParent removes all cached blocks with the given parent (non-recursively).
func (b *backend) dropForParent(parentID flow.Identifier) {

	b.mu.Lock()
	defer b.mu.Unlock()

	children, exists := b.byParent[parentID]
	if !exists {
		return
	}

	for _, childID := range children {
		delete(b.byID, childID)
	}
	delete(b.byParent, parentID)
}

// pruneByHeight prunes any items in the cache that have height below the
// given height. The pruning height should be the finalized height.
func (b *backend) pruneByHeight(height uint64) {

	b.mu.Lock()
	defer b.mu.Unlock()

	for id, item := range b.byID {
		if item.header.Height < height {
			delete(b.byID, id)
			delete(b.byParent, item.header.ParentID)
		}
	}
}
