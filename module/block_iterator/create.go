package block_iterator

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// CreateIndexedBlockIterator creates a block iterator that iterates through blocks by index.
func CreateIndexedBlockIterator(
	getBlockIDByIndex func(uint64) (blockID flow.Identifier, indexed bool, exception error),
	progress storage.ConsumerProgress,
	getRoot func() (uint64, error),
	latest func() (uint64, error),
) (module.BlockIterator, error) {

	initializer := NewInitializer(progress, getRoot)
	progressReader, progressWriter, err := initializer.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize progress: %w", err)
	}

	rangeCreator := NewIteratorRangeCreator(latest)
	iterRange, err := rangeCreator.CreateRange(progressReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create range for block iteration: %w", err)
	}

	return NewIndexedBlockIterator(getBlockIDByIndex, progressWriter, iterRange), nil
}

// CreateHeightBasedBlockIterator creates a block iterator that iterates through blocks
// from root to the latest (either finalized or sealed) by height.
func CreateHeightBasedBlockIterator(
	getBlockIDByHeight func(height uint64) (flow.Identifier, error),
	progress storage.ConsumerProgress,
	getRoot func() (*flow.Header, error),
	latest func() (*flow.Header, error),
) (module.BlockIterator, error) {

	return CreateIndexedBlockIterator(
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
		func() (uint64, error) {
			root, err := getRoot()
			if err != nil {
				return 0, fmt.Errorf("failed to get root block: %w", err)
			}
			return root.Height, nil
		},
		func() (uint64, error) {
			latestBlock, err := latest()
			if err != nil {
				return 0, fmt.Errorf("failed to get latest block: %w", err)
			}
			return latestBlock.Height, nil
		},
	)
}

// CreateViewBasedBlockIterator creates a block iterator that iterates through blocks
// from root to the latest (either finalized or sealed) by view.
func CreateViewBasedBlockIterator(
	getBlockIDByView func(view uint64) (blockID flow.Identifier, viewIndexed bool, exception error),
	progress storage.ConsumerProgress,
	getRoot func() (*flow.Header, error),
	latest func() (*flow.Header, error),
) (module.BlockIterator, error) {
	return CreateIndexedBlockIterator(
		getBlockIDByView,
		progress,
		func() (uint64, error) {
			root, err := getRoot()
			if err != nil {
				return 0, fmt.Errorf("failed to get root block: %w", err)
			}
			return root.View, nil
		},
		func() (uint64, error) {
			latestBlock, err := latest()
			if err != nil {
				return 0, fmt.Errorf("failed to get latest block: %w", err)
			}
			return latestBlock.View, nil
		},
	)
}
