package storehouse

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

type blockUpdates struct {
	View    uint64
	Parent  flow.Identifier
	Updates map[flow.RegisterID]flow.RegisterValue
}

type ForkAwareStore struct {
	lock         sync.RWMutex
	prunedView   uint64                                           // for lowest view to lookup register update
	blockUpdates map[flow.Identifier]*blockUpdates                // index block updates by block ID
	blockViews   map[uint64]flow.Identifier                       // for finding blockID by view, useful for state extraction
	children     map[flow.Identifier]map[flow.Identifier]struct{} // for pruning conflicting forks
}

// 3 <- 4 <- 5 <- 8
//        ^- 6 <- 7
//					   ^- 9

func (f *ForkAwareStore) GetRegsiter(view uint64, id flow.RegisterID) (flow.RegisterValue, error) {
	f.lock.RLock()
	defer f.lock.RUnlock()

	if view <= f.prunedView {
		return nil, fmt.Errorf("view is pruned")
	}

	blockID, ok := f.blockViews[view]
	if !ok {
		return nil, fmt.Errorf("block not executed")
	}

	blockUpdates, ok := f.blockUpdates[blockID]
	if !ok {
		return nil, fmt.Errorf("block not exists while view exists")
	}

	// traversing along the fork, until either
	// found a block that has the given register updated, or
	// reached the pruned view, which means the register was not updated
	// from pruned view to the given view
	for {
		value, ok := blockUpdates.Updates[id]
		if ok {
			return value, nil
		}

		blockUpdates, ok = f.blockUpdates[blockUpdates.Parent]
		if !ok {
			return nil, fmt.Errorf("critical error: missing block updates")
		}

		// go back to the loop
	}
}

// AddForBlock add the register updates of a given block to ForkAwareStore
func (f *ForkAwareStore) AddForBlock(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	if header.View <= f.prunedView {
		return nil // drop
	}

	// update blockUpdates
	blockID := header.ID()
	f.blockUpdates[blockID] = &blockUpdates{
		View:    header.View,
		Parent:  header.ParentID,
		Updates: update,
	}

	// update f.children
	parent, ok := f.children[header.ParentID]
	if !ok {
		parent = make(map[flow.Identifier]struct{})
	}
	// add itself to its parent's children
	parent[blockID] = struct{}{}
	f.children[header.ParentID] = parent

	// update blockViews
	f.blockViews[header.View] = blockID

	return nil
}

// GetUpdatesByBlock returns all the register updates updated at the given block.
// Useful for moving them into NonForkAwareStorage when the given block becomes finalized.
// If the given block has not been executed, it returns nil, NotFoundError
func (f *ForkAwareStore) GetUpdatesByBlock(blockID flow.Identifier) (map[flow.RegisterID]flow.RegisterValue, bool) {
	f.lock.Lock()
	defer f.lock.Unlock()
	blockUpdates, ok := f.blockUpdates[blockID]
	if !ok {
		return nil, false
	}
	return blockUpdates.Updates, true
}

// PruneByFinalized remove the register updates for all the blocks below the finalized height
// as well as the blocks that are conflicting with the finalized block
// Make sure the register updates for finalized blocks have been moved to non-forkaware
// storage before pruning them from forkaware store.
// For instance, given the following state in the forkaware store:
// A <- B <- C <- D
//
//	^- E <- F
//
// if C is finalized, then PruneByFinalized(C) will prune [A, B, E, F],
// [A,B] are pruned because they are below finalized view,
// [E,F] are pruned because they are conflicting with finalized block C.
// Pruned Finalized must be a previous exectued block
func (f *ForkAwareStore) PruneByFinalized(finalized *flow.Header) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if finalized.View <= f.prunedView {
		return
	}

	finalizedID := finalized.ID()
	// prune all forks that are conflicting with finalized chain
	f.pruneConflictingForks(finalizedID)

	// prune the finalized and executed blocks
	f.pruneFinalized(finalizedID)

	f.prunedView = finalized.View
}

func (f *ForkAwareStore) pruneConflictingForks(finalizedID flow.Identifier) {
	blockID := finalizedID
	for {
		blockUpdates, ok := f.blockUpdates[blockID]
		if !ok {
			continue
		}

		finalizedView := blockUpdates.View

		siblings, ok := f.children[blockUpdates.Parent]
		if !ok {
			panic("can't find parent")
		}
		for sibling := range siblings {
			siblingUpdates, ok := f.blockUpdates[sibling]
			if !ok {
				continue
			}

			// conflicting forks
			if siblingUpdates.View != finalizedView {
				f.pruneChildren(sibling)
			}
		}

		blockID = blockUpdates.Parent
	}
}

func (f *ForkAwareStore) pruneChildren(blockID flow.Identifier) {
	blockUpdates, ok := f.blockUpdates[blockID]
	if !ok {
		return
	}

	f.pruneBlockUpdate(blockID, blockUpdates.View)

	children, ok := f.children[blockID]
	// prune children recursively
	for child := range children {
		f.pruneChildren(child)
	}
}

func (f *ForkAwareStore) pruneFinalized(finalizedID flow.Identifier) {
	blockID := finalizedID
	for {
		blockUpdates, ok := f.blockUpdates[blockID]
		if !ok {
			break
		}

		f.pruneBlockUpdate(blockID, blockUpdates.View)

		blockID = blockUpdates.Parent
	}
}

func (f *ForkAwareStore) pruneBlockUpdate(blockID flow.Identifier, view uint64) {
	blockUpdates, ok := f.blockUpdates[blockID]
	if !ok {
		return // should not happen
	}
	delete(f.blockUpdates, blockID)
	delete(f.blockViews, view)
	delete(f.children, blockID)

	// remove itself from its parent's children
	siblings, ok := f.children[blockUpdates.Parent]
	if !ok {
		return
	}
	delete(siblings, blockID)
}
