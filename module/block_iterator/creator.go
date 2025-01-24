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
	progress          module.IterateProgress
	latest            func() (uint64, error)
}

var _ module.IteratorCreator = (*Creator)(nil)

// NewCreator creates a block iterator that iterates through blocks by index.
func NewCreator(
	getBlockIDByIndex func(uint64) (blockID flow.Identifier, indexed bool, exception error),
	progressStorage storage.ConsumerProgress,
	root uint64,
	latest func() (uint64, error),
) (*Creator, error) {
	// initialize the progress in storage, saving the root block index in storage
	progress, err := InitializeProgress(progressStorage, root)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize progress: %w", err)
	}

	return &Creator{
		getBlockIDByIndex: getBlockIDByIndex,
		progress:          progress,
		latest:            latest,
	}, nil
}

func (c *Creator) Create() (module.BlockIterator, error) {
	// create a iteration range from the root block to the latest block
	iterRange, err := CreateRange(c.progress, c.latest)
	if err != nil {
		return nil, fmt.Errorf("failed to create range for block iteration: %w", err)
	}

	// create a block iterator with
	// the function to get block ID by index,
	// the progress writer to update the progress in storage,
	// and the iteration range
	return NewIndexedBlockIterator(c.getBlockIDByIndex, c.progress, iterRange), nil
}

// NewHeightBasedCreator creates a block iterator that iterates through blocks
// from root to the latest (either finalized or sealed) by height.
func NewHeightBasedCreator(
	getBlockIDByHeight func(height uint64) (flow.Identifier, error),
	progress storage.ConsumerProgress,
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
	progress storage.ConsumerProgress,
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
