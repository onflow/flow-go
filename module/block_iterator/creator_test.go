package block_iterator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// TestCanIterate: iterate through all heights from root to latest exactly once
// TestCanIterateNewLatest: iterate through all heights from root to latest,
// 			and after latest updated, it can iterate from latest + 1 to new latest
// TestCanResume: stop at a height, and take checkpoint, create a new iterator,
// 			verify it will resume from the next height to the latest
// TestIterateByView: iterate through all views
// TestCanSkipViewsIfNotIndexed: iterate through all views, and skip views that are not indexed

func TestCanIterate(t *testing.T) {
	root := &flow.Header{Height: 0}
	// Create mock blocks
	blocks := []*flow.Header{
		{Height: 1},
		{Height: 2},
		{Height: 3},
		{Height: 4},
		{Height: 5},
	}

	// Mock getBlockIDByHeight function
	getBlockIDByHeight := func(height uint64) (flow.Identifier, error) {
		for _, block := range blocks {
			if block.Height == height {
				return block.ID(), nil
			}
		}
		return flow.Identifier{}, fmt.Errorf("block not found at height %d", height)
	}

	// Mock progress tracker
	progress := &mockProgress{}

	// Mock getRoot and latest functions
	getRoot := func() (*flow.Header, error) {
		return root, nil
	}
	latest := func() (*flow.Header, error) {
		return blocks[len(blocks)-1], nil
	}

	// Create iterator
	iterator, err := CreateHeightBasedBlockIterator(
		getBlockIDByHeight,
		progress,
		getRoot,
		latest,
	)
	require.NoError(t, err)

	// Iterate through blocks
	visitedBlocks := make(map[flow.Identifier]int)
	for {
		blockID, ok, err := iterator.Next()
		require.NoError(t, err)
		if !ok {
			break
		}
		visitedBlocks[blockID]++
	}

	// Verify all blocks were visited exactly once
	for _, block := range blocks {
		count, ok := visitedBlocks[block.ID()]
		require.True(t, ok, "Block %v was not visited", block.Height)
		require.Equal(t, 1, count, "Block %v was visited %d times, expected once", block.ID, count)
	}

	// Verify no extra blocks were visited
	require.Equal(t, len(blocks), len(visitedBlocks), "Unexpected number of blocks visited")

	// Verify the final checkpoint
	require.NoError(t, iterator.Checkpoint())
	savedNextHeight, err := progress.ProcessedIndex()
	require.NoError(t, err)
	require.Equal(t, uint64(6), savedNextHeight, "Expected next height to be 6 (last height + 1)")
}

type mockProgress struct {
	height      uint64
	initialized bool
}

func (m *mockProgress) ProcessedIndex() (uint64, error) {
	if !m.initialized {
		return 0, fmt.Errorf("processed index not initialized: %w", storage.ErrNotFound)
	}
	return m.height, nil
}

func (m *mockProgress) InitProcessedIndex(defaultIndex uint64) error {
	if m.initialized {
		return fmt.Errorf("processed index already initialized")
	}
	m.height = defaultIndex
	m.initialized = true
	return nil
}

func (m *mockProgress) SetProcessedIndex(processed uint64) error {
	if !m.initialized {
		return fmt.Errorf("processed index not initialized")
	}
	m.height = processed
	return nil
}
