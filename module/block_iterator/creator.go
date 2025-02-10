package block_iterator

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// Creator creates block iterators.
// a block iterator iterates through a saved index to the latest block.
// after iterating through all the blocks in the range, the iterator can be discarded.
// a new block iterator can be created to iterate through the next range.
type Creator struct {
	getBlockIDByIndex func(uint64) (flow.Identifier, bool, error)
	progress          *PersistentIteratorState
}

var _ module.IteratorCreator = (*Creator)(nil)

// NewCreator creates a block iterator that iterates through blocks by index.
// the root is the block index to start iterating from. (it could either root height or root view)
// the latest is a function that returns the latest block index.
// since latest is a function, the caller can reuse the creator to create block iterator one
// after another to iterate from the root to the latest, and from last iterated to the new latest.
func NewCreator(
	getBlockIDByIndex func(uint64) (blockID flow.Identifier, indexed bool, exception error),
	progressStorage storage.ConsumerProgressInitializer,
	root uint64,
	latest func() (uint64, error),
) (*Creator, error) {
	// initialize the progress in storage, saving the root block index in storage
	progress, err := NewPersistentIteratorState(progressStorage, root, latest)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize progress: %w", err)
	}

	return &Creator{
		getBlockIDByIndex: getBlockIDByIndex,
		progress:          progress,
	}, nil
}

func (c *Creator) Create() (iter module.BlockIterator, hasNext bool, exception error) {
	// create a iteration range from the first un-iterated to the latest block
	iterRange, hasNext, err := c.progress.NextRange()
	if err != nil {
		return nil, false, fmt.Errorf("failed to create range for block iteration: %w", err)
	}

	if !hasNext {
		// no block to iterate
		return nil, false, nil
	}

	// create a block iterator with
	// the function to get block ID by index,
	// the progress writer to update the progress in storage,
	// and the iteration range
	return NewIndexedBlockIterator(c.getBlockIDByIndex, c.progress, iterRange), true, nil
}

// NewHeightBasedCreator creates a block iterator that iterates through blocks
// from root to the latest (either finalized or sealed) by height.
func NewHeightBasedCreator(
	getBlockIDByHeight func(height uint64) (flow.Identifier, error),
	progress storage.ConsumerProgressInitializer,
	root *flow.Header,
	latest func() (*flow.Header, error),
) (*Creator, error) {

	return NewCreator(
		func(height uint64) (flow.Identifier, bool, error) {
			blockID, err := getBlockIDByHeight(height)
			if err != nil {
				return flow.Identifier{}, false, fmt.Errorf("failed to get block ID by height: %w", err)
			}
			// each height between root and latest (either finalized or sealed) must be indexed.
			// so it's always true
			alwaysIndexed := true
			return blockID, alwaysIndexed, nil
		},
		progress,
		root.Height,
		func() (uint64, error) {
			latestBlock, err := latest()
			if err != nil {
				return 0, fmt.Errorf("failed to get latest block: %w", err)
			}
			return latestBlock.Height, nil
		},
	)
}

// NewViewBasedCreator creates a block iterator that iterates through blocks
// from root to the latest (either finalized or sealed) by view.
// since view has gaps, the iterator will skip views that have no blocks.
func NewViewBasedCreator(
	getBlockIDByView func(view uint64) (blockID flow.Identifier, viewIndexed bool, exception error),
	progress storage.ConsumerProgressInitializer,
	root *flow.Header,
	latest func() (*flow.Header, error),
) (*Creator, error) {
	return NewCreator(
		getBlockIDByView,
		progress,
		root.View,
		func() (uint64, error) {
			latestBlock, err := latest()
			if err != nil {
				return 0, fmt.Errorf("failed to get latest block: %w", err)
			}
			return latestBlock.View, nil
		},
	)
}
