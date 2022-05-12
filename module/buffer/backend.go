package buffer

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
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
	blocksByParent map[flow.Identifier][]flow.Identifier
	// set of pending blocks, keyed by ID to avoid duplication
	blocksByID map[flow.Identifier]*item
}

// newBackend returns a new pending block cache.
func newBackend() *backend {
	cache := &backend{
		blocksByParent: make(map[flow.Identifier][]flow.Identifier),
		blocksByID:     make(map[flow.Identifier]*item),
	}
	return cache
}

// add adds the item to the cache, returning false if it already exists and
// true otherwise.
func (b *backend) add(originID flow.Identifier, header *flow.Header, payload interface{}) bool {

	b.mu.Lock()
	defer b.mu.Unlock()

	blockID := header.ID()

	_, exists := b.blocksByID[blockID]
	if exists {
		return false
	}

	item := &item{
		header:   header,
		originID: originID,
		payload:  payload,
	}

	b.blocksByID[blockID] = item
	b.blocksByParent[header.ParentID] = append(b.blocksByParent[header.ParentID], blockID)

	return true
}

func (b *backend) byID(id flow.Identifier) (*item, bool) {

	b.mu.RLock()
	defer b.mu.RUnlock()

	item, exists := b.blocksByID[id]
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

	forParent, exists := b.blocksByParent[parentID]
	if !exists {
		return nil, false
	}

	items := make([]*item, 0, len(forParent))
	for _, blockID := range forParent {
		items = append(items, b.blocksByID[blockID])
	}

	return items, true
}

// dropForParent removes all cached blocks with the given parent (non-recursively).
func (b *backend) dropForParent(parentID flow.Identifier) {

	b.mu.Lock()
	defer b.mu.Unlock()

	children, exists := b.blocksByParent[parentID]
	if !exists {
		return
	}

	for _, childID := range children {
		delete(b.blocksByID, childID)
	}
	delete(b.blocksByParent, parentID)
}

// pruneByView prunes any items in the cache that have view less than or
// equal to the given view. The pruning view should be the finalized view.
func (b *backend) pruneByView(view uint64) {

	b.mu.Lock()
	defer b.mu.Unlock()

	for id, item := range b.blocksByID {
		if item.header.View <= view {
			delete(b.blocksByID, id)
			delete(b.blocksByParent, item.header.ParentID)
		}
	}
}

// size returns the number of elements stored in teh backend
func (b *backend) size() uint {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return uint(len(b.blocksByID))
}
