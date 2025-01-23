package block_iterator

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// IndexedBlockIterator is a block iterator that iterates over blocks by height
// it's not concurrent safe, so don't use it in multiple goroutines
type IndexedBlockIterator struct {
	// dependencies
	getBlockIDByIndex func(uint64) (blockID flow.Identifier, indexed bool, excpetion error)
	progress          module.IterateProgressWriter // for saving the next height to be iterated for resuming the iteration

	// config
	endIndex uint64

	// state
	nextIndex uint64
}

var _ module.BlockIterator = (*IndexedBlockIterator)(nil)

// caller must ensure that both iterRange.Start and iterRange.End are finalized height
func NewIndexedBlockIterator(
	getBlockIDByIndex func(uint64) (blockID flow.Identifier, indexed bool, excpetion error),
	progress module.IterateProgressWriter,
	iterRange module.IterateRange,
) module.BlockIterator {
	return &IndexedBlockIterator{
		getBlockIDByIndex: getBlockIDByIndex,
		progress:          progress,
		endIndex:          iterRange.End,
		nextIndex:         iterRange.Start,
	}
}

// Next returns the next block ID in the iteration
// it iterates from lower height to higher height.
// when iterating a height, it iterates over all sibling blocks at that height
// Note: this method is not concurrent-safe
func (b *IndexedBlockIterator) Next() (flow.Identifier, bool, error) {
	if b.nextIndex > b.endIndex {
		return flow.ZeroID, false, nil
	}

	next, indexed, err := b.getBlockIDByIndex(b.nextIndex)
	if err != nil {
		return flow.ZeroID, false, fmt.Errorf("failed to fetch block at index (height or view) %v: %w", b.nextIndex, err)
	}

	// if the block is not indexed, skip it.
	// when we are iterating by view, it's possible that there is no block for certain views, in
	// that case, we skip and iterate the next view
	// when we are iterating by height, it's not possible that a height is not indexed, so indexed should
	// always be true
	if !indexed {
		// iterate next height
		b.nextIndex++
		return b.Next()
	}

	b.nextIndex++

	return next, true, nil
}

// Checkpoint saves the iteration progress to storage
// make sure to call this after all the blocks for processing the block IDs returned by
// Next() are completed.
func (b *IndexedBlockIterator) Checkpoint() (uint64, error) {
	err := b.progress.SaveState(b.nextIndex)
	if err != nil {
		return 0, fmt.Errorf("failed to save progress at view %v: %w", b.nextIndex, err)
	}
	return b.nextIndex, nil
}
