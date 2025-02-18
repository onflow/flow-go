package block_iterator

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// IndexedBlockIterator is a block iterator that iterates over blocks by height or view
// when index is height, it iterates from lower height to higher height
// when index is view, it iterates from lower view to higher view
// caller must ensure that the range is finalized, otherwise the iteration might miss some blocks
// it's not concurrent safe, so don't use it in multiple goroutines
type IndexedBlockIterator struct {
	// dependencies
	getBlockIDByIndex func(uint64) (blockID flow.Identifier, indexed bool, exception error)
	progress          module.IteratorStateWriter // for saving the next index to be iterated for resuming the iteration
	endIndex          uint64                     // the end index to iterate, this never change
	nextIndex         uint64                     // the start index to iterate, this will be updated after each iteration
}

var _ module.BlockIterator = (*IndexedBlockIterator)(nil)

// caller must ensure that both iterRange.Start and iterRange.End are finalized
func NewIndexedBlockIterator(
	getBlockIDByIndex func(uint64) (blockID flow.Identifier, indexed bool, exception error),
	progress module.IteratorStateWriter,
	iterRange module.IteratorRange,
) module.BlockIterator {
	return &IndexedBlockIterator{
		getBlockIDByIndex: getBlockIDByIndex,
		progress:          progress,
		endIndex:          iterRange.End,
		nextIndex:         iterRange.Start,
	}
}

// Next returns the next block ID in the iteration
// it iterates from lower index to higher index.
// Note: this method is not concurrent-safe
func (b *IndexedBlockIterator) Next() (flow.Identifier, bool, error) {
	if b.nextIndex > b.endIndex {
		return flow.ZeroID, false, nil
	}

	next, indexed, err := b.getBlockIDByIndex(b.nextIndex)
	if err != nil {
		return flow.ZeroID, false, fmt.Errorf("failed to fetch block at index (height or view) %v: %w", b.nextIndex, err)
	}

	// if the block is not indexed, skip it. This is only possible when we are iterating by view.
	// when iterating by height, all blocks should be indexed, so `indexed` should always be true.
	if !indexed {
		// if the view is not indexed, then iterate next view
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
