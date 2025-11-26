package buffer

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

// item represents an item in the cache: the main block and auxiliary data that is
// needed to implement indexed lookups by parent ID and pruning by view.
type item[T extractProposalHeader] struct {
	view     uint64
	parentID flow.Identifier
	block    flow.Slashable[T]
}

// extractProposalHeader is a type constraint for the generic type which allows to extract flow.ProposalHeader
// from the underlying type.
type extractProposalHeader interface {
	ProposalHeader() *flow.ProposalHeader
}

// backend implements a simple cache of pending blocks, indexed by parent ID and pruned by view.
type backend[T extractProposalHeader] struct {
	mu sync.RWMutex
	// map of pending header IDs, keyed by parent ID for ByParentID lookups
	blocksByParent map[flow.Identifier][]flow.Identifier
	// set of pending blocks, keyed by ID to avoid duplication
	blocksByID map[flow.Identifier]*item[T]
}

// newBackend returns a new pending header cache.
func newBackend[T extractProposalHeader]() *backend[T] {
	cache := &backend[T]{
		blocksByParent: make(map[flow.Identifier][]flow.Identifier),
		blocksByID:     make(map[flow.Identifier]*item[T]),
	}
	return cache
}

// add adds the item to the cache, returning false if it already exists and
// true otherwise.
func (b *backend[T]) add(block flow.Slashable[T]) bool {
	header := block.Message.ProposalHeader().Header
	blockID := header.ID()

	b.mu.Lock()
	defer b.mu.Unlock()

	_, exists := b.blocksByID[blockID]
	if exists {
		return false
	}

	item := &item[T]{
		view:     header.View,
		parentID: header.ParentID,
		block:    block,
	}

	b.blocksByID[blockID] = item
	b.blocksByParent[item.parentID] = append(b.blocksByParent[item.parentID], blockID)

	return true
}

func (b *backend[T]) byID(id flow.Identifier) (*item[T], bool) {
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
func (b *backend[T]) byParentID(parentID flow.Identifier) ([]*item[T], bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	forParent, exists := b.blocksByParent[parentID]
	if !exists {
		return nil, false
	}

	items := make([]*item[T], 0, len(forParent))
	for _, blockID := range forParent {
		items = append(items, b.blocksByID[blockID])
	}

	return items, true
}

// dropForParent removes all cached blocks with the given parent (non-recursively).
func (b *backend[T]) dropForParent(parentID flow.Identifier) {
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
func (b *backend[T]) pruneByView(view uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for id, item := range b.blocksByID {
		if item.view <= view {
			delete(b.blocksByID, id)
			delete(b.blocksByParent, item.parentID)
		}
	}
}

// size returns the number of elements stored in teh backend
func (b *backend[T]) size() uint {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return uint(len(b.blocksByID))
}
