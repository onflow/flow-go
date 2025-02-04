package block_iterator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// TestCanIterateNewLatest: iterate through all heights from root to latest,
// 			and after finish iteration, iterate again with an updated latest height,
// 			verify it can iterate from latest + 1 to new latest
// TestCanResume: stop at a height, and take checkpoint, create a new iterator,
// 			verify it will resume from the next height to the latest
// TestCanSkipViewsIfNotIndexed: iterate through all views, and skip views that are not indexed
// TestCanSkipIfThereIsNoBlockToIterate: skip iterationg if there is no block to iterate

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

	// Mock latest functions
	latest := func() (*flow.Header, error) {
		return blocks[len(blocks)-1], nil
	}

	// Create iterator
	creator, err := NewHeightBasedCreator(
		getBlockIDByHeight,
		progress,
		root,
		latest,
	)
	require.NoError(t, err)

	iterator, hasNext, err := creator.Create()
	require.NoError(t, err)
	require.True(t, hasNext)

	// Iterate through blocks
	visitedBlocks := make([]flow.Identifier, 0, len(blocks))
	for {
		blockID, ok, err := iterator.Next()
		require.NoError(t, err)
		if !ok {
			break
		}

		visitedBlocks = append(visitedBlocks, blockID)
	}

	// Verify all blocks were visited exactly once
	for i, block := range blocks {
		require.Equal(t, block.ID(), visitedBlocks[i], "Block %v was not visited", block.Height)
	}

	// Verify no extra blocks were visited
	require.Equal(t, len(blocks), len(visitedBlocks), "Unexpected number of blocks visited")

	// Verify the final checkpoint
	next, err := iterator.Checkpoint()
	require.NoError(t, err)
	require.Equal(t, uint64(6), next, "Expected next height to be 6 (last height + 1)")
	savedNextHeight, err := progress.ProcessedIndex()
	require.NoError(t, err)
	require.Equal(t, uint64(6), savedNextHeight, "Expected next height to be 6 (last height + 1)")

	// Additional blocks to be added later
	additionalBlocks := []*flow.Header{
		{Height: 6},
		{Height: 7},
		{Height: 8},
	}
	visitedBlocks = make([]flow.Identifier, 0, len(additionalBlocks))

	// Update blocks so that the latest block is updated, and getBlockIDByHeight
	// will return the new blocks
	blocks = append(blocks, additionalBlocks...)

	// Create another iterator
	iterator, hasNext, err = creator.Create()
	require.NoError(t, err)
	require.True(t, hasNext)

	// Iterate through initial blocks
	for i := 0; i < len(additionalBlocks); i++ {
		blockID, ok, err := iterator.Next()
		require.NoError(t, err)
		require.True(t, ok)
		visitedBlocks = append(visitedBlocks, blockID)
	}

	// No more blocks to iterate
	_, ok, err := iterator.Next()
	require.NoError(t, err)
	require.False(t, ok)

	// Verify all additional blocks were visited exactly once
	for i, block := range additionalBlocks {
		require.Equal(t, block.ID(), visitedBlocks[i], "Block %v was not visited", block.Height)
	}

	// Verify no extra blocks were visited
	require.Equal(t, len(additionalBlocks), len(visitedBlocks), "Unexpected number of blocks visited")

	// Verify the final checkpoint
	next, err = iterator.Checkpoint()
	require.NoError(t, err)
	require.Equal(t, uint64(9), next, "Expected next height to be 9 (last height + 1)")
	savedHeight, err := progress.ProcessedIndex()
	require.NoError(t, err)
	require.Equal(t, uint64(9), savedHeight, "Expected next height to be 9 (last height + 1)")
}

func TestCanResume(t *testing.T) {

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

	// Mock latest functions
	latest := func() (*flow.Header, error) {
		return blocks[len(blocks)-1], nil
	}

	// Create iterator
	creator, err := NewHeightBasedCreator(
		getBlockIDByHeight,
		progress,
		root,
		latest,
	)
	require.NoError(t, err)

	iterator, hasNext, err := creator.Create()
	require.NoError(t, err)
	require.True(t, hasNext)

	// Iterate through blocks
	visitedBlocks := make([]flow.Identifier, 0, len(blocks))
	for i := 0; i < 3; i++ { // iterate up to Height 3
		blockID, ok, err := iterator.Next()
		require.NoError(t, err)
		if !ok {
			break
		}
		visitedBlocks = append(visitedBlocks, blockID)
	}

	// save the progress
	_, err = iterator.Checkpoint()
	require.NoError(t, err)

	// Additional blocks to be added later
	additionalBlocks := []*flow.Header{
		{Height: 6},
		{Height: 7},
		{Height: 8},
	}

	// Update blocks so that the latest block is updated, and getBlockIDByHeight
	// will return the new blocks
	blocks = append(blocks, additionalBlocks...)

	// simulate a restart by creating a new creator with a different latest
	newCreator, err := NewHeightBasedCreator(
		getBlockIDByHeight,
		progress,
		root,
		latest,
	)
	require.NoError(t, err)

	newIterator, hasNext, err := newCreator.Create()
	require.NoError(t, err)
	require.True(t, hasNext)

	// iterate until the end
	for {
		blockID, ok, err := newIterator.Next()
		require.NoError(t, err)
		if !ok {
			break
		}

		visitedBlocks = append(visitedBlocks, blockID)
	}

	// verify all blocks are visited
	for i, block := range blocks {
		require.Equal(t, block.ID(), visitedBlocks[i], "Block %v was not visited", block.Height)
	}

	// Verify no extra blocks were visited
	require.Equal(t, len(blocks), len(visitedBlocks), "Unexpected number of blocks visited")
}

func TestCanSkipViewsIfNotIndexed(t *testing.T) {
	// Create mock blocks with some indexed and some not
	blocks := []*flow.Header{
		{View: 1},
		{View: 2},
		{View: 3},
		{View: 5},
		{View: 7},
	}

	// Mock getBlockIDByHeight function
	getBlockIDByView := func(view uint64) (blockID flow.Identifier, viewIndexed bool, exception error) {
		for _, block := range blocks {
			if block.View == view {
				return block.ID(), true, nil
			}
		}

		return flow.Identifier{}, false, nil
	}

	// Mock progress tracker
	progress := &mockProgress{}

	// Mock getRoot and latest functions
	root := &flow.Header{View: 0}
	latest := func() (*flow.Header, error) {
		return blocks[len(blocks)-1], nil
	}

	// Create iterator
	creator, err := NewViewBasedCreator(
		getBlockIDByView,
		progress,
		root,
		latest,
	)
	require.NoError(t, err)

	iterator, hasNext, err := creator.Create()
	require.NoError(t, err)
	require.True(t, hasNext)

	// Iterate through blocks
	visitedBlocks := make(map[flow.Identifier]struct{})
	for {
		blockID, ok, err := iterator.Next()
		require.NoError(t, err)
		if !ok {
			break
		}
		visitedBlocks[blockID] = struct{}{}
	}

	// Verify all blocks were visited exactly once
	for _, block := range blocks {
		_, ok := visitedBlocks[block.ID()]
		require.True(t, ok, "Block %v was not visited", block.View)
		delete(visitedBlocks, block.ID())
	}

	// Verify no extra blocks were visited
	require.Empty(t, visitedBlocks, "Unexpected number of blocks visited")

	// Verify the final checkpoint
	next, err := iterator.Checkpoint()
	require.NoError(t, err)
	require.Equal(t, uint64(8), next, "Expected next height to be 8 (last height + 1)")
	savedView, err := progress.ProcessedIndex()
	require.NoError(t, err)
	require.Equal(t, uint64(8), savedView, "Expected next view to be 8 (last View + 1)")
}

func TestCanSkipIfThereIsNoBlockToIterate(t *testing.T) {
	// Set up root block
	root := &flow.Header{Height: 10}

	// Mock getBlockIDByHeight function
	getBlockIDByHeight := func(height uint64) (flow.Identifier, error) {
		return flow.Identifier{}, fmt.Errorf("block not found at height %d", height)
	}

	// Mock progress tracker
	progress := &mockProgress{}

	// Mock latest function that returns the same height as root
	latest := func() (*flow.Header, error) {
		return root, nil
	}

	// Create iterator
	creator, err := NewHeightBasedCreator(
		getBlockIDByHeight,
		progress,
		root,
		latest,
	)
	require.NoError(t, err)

	// Create the iterator
	_, hasNext, err := creator.Create()
	require.NoError(t, err)
	require.False(t, hasNext, "Expected no blocks to iterate")

	savedHeight, err := progress.ProcessedIndex()
	require.NoError(t, err)
	require.Equal(t, root.Height+1, savedHeight, "Expected saved height to be root height + 1")
}

type mockProgress struct {
	index       uint64
	initialized bool
}

func (m *mockProgress) ProcessedIndex() (uint64, error) {
	if !m.initialized {
		return 0, fmt.Errorf("processed index not initialized: %w", storage.ErrNotFound)
	}
	return m.index, nil
}

func (m *mockProgress) InitProcessedIndex(defaultIndex uint64) error {
	if m.initialized {
		return fmt.Errorf("processed index already initialized")
	}
	m.index = defaultIndex
	m.initialized = true
	return nil
}

func (m *mockProgress) SetProcessedIndex(processed uint64) error {
	if !m.initialized {
		return fmt.Errorf("processed index not initialized")
	}
	m.index = processed
	return nil
}
