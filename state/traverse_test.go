package state

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestTraverse tests different scenarios for reverse block traversing
func TestTraverse(t *testing.T) {

	// create a storage.Headers mock with a backing map
	byID := make(map[flow.Identifier]*flow.Header)
	byHeight := make(map[uint64]*flow.Header)
	headers := new(mockstorage.Headers)
	headers.On("ByBlockID", mock.Anything).Return(
		func(id flow.Identifier) *flow.Header {
			return byID[id]
		},
		func(id flow.Identifier) error {
			_, ok := byID[id]
			if !ok {
				return storage.ErrNotFound
			}
			return nil
		})

	// populate the mocked header storage with genesis and 10 child blocks
	genesis := unittest.BlockHeaderFixture()
	genesis.Height = 0
	byID[genesis.ID()] = &genesis
	byHeight[genesis.Height] = &genesis

	parent := &genesis
	for i := 0; i < 10; i++ {
		child := unittest.BlockHeaderWithParentFixture(parent)
		byID[child.ID()] = &child
		t.Log(child.Height)
		byHeight[child.Height] = &child

		parent = &child
	}

	// should return error and not call callback when start block doesn't exist
	t.Run("non-existent start block", func(t *testing.T) {
		start := unittest.IdentifierFixture()
		err := Traverse(headers, start, func(_ *flow.Header) (bool, error) {
			// should not be called
			t.Fail()
			return false, nil
		})
		assert.Error(t, err)
	})

	// should return error when end block doesn't exist
	t.Run("non-existent end block", func(t *testing.T) {
		start := byHeight[8].ID()
		err := Traverse(headers, start, func(_ *flow.Header) (bool, error) {
			return true, nil
		})
		assert.Error(t, err)
	})

	// should return error if the callback returns an error
	t.Run("callback error", func(t *testing.T) {
		start := byHeight[8].ID()
		err := Traverse(headers, start, func(_ *flow.Header) (bool, error) {
			return true, fmt.Errorf("callback error")
		})
		assert.Error(t, err)
	})

	// should call the callback exactly once and not return an error when start == end
	t.Run("single-block traversal", func(t *testing.T) {
		start := byHeight[5].ID()
		end := byHeight[4].ID()

		called := 0
		err := Traverse(headers, start, func(header *flow.Header) (bool, error) {
			if header.ID() == end {
				return false, nil
			}
			// should call callback for single block in traversal path
			assert.Equal(t, start, header.ID())
			// track calls - should only be called once
			called++
			return true, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, called)
	})

	// should call the callback exactly once for each block in traversal path
	// and not return an error
	t.Run("multi-block traversal", func(t *testing.T) {
		startHeight := uint64(8)
		endHeight := uint64(4)

		start := byHeight[startHeight].ID()

		// assert that we are receiving the correct block at each height
		height := startHeight
		err := Traverse(headers, start, func(header *flow.Header) (bool, error) {
			if header.Height < endHeight {
				return false, nil
			}
			expectedID := byHeight[height].ID()
			assert.Equal(t, expectedID, header.ID())
			height--
			return true, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, endHeight, height+1)
	})
}

// TestTraverseParentFirst tests different scenarios for parent-first block traversing
func TestTraverseParentFirst(t *testing.T) {

	// create a storage.Headers mock with a backing map
	byID := make(map[flow.Identifier]*flow.Header)
	byHeight := make(map[uint64]*flow.Header)
	headers := new(mockstorage.Headers)
	headers.On("ByBlockID", mock.Anything).Return(
		func(id flow.Identifier) *flow.Header {
			return byID[id]
		},
		func(id flow.Identifier) error {
			_, ok := byID[id]
			if !ok {
				return storage.ErrNotFound
			}
			return nil
		})

	// populate the mocked header storage with genesis and 10 child blocks
	genesis := unittest.BlockHeaderFixture()
	genesis.Height = 0
	byID[genesis.ID()] = &genesis
	byHeight[genesis.Height] = &genesis

	parent := &genesis
	for i := 0; i < 10; i++ {
		child := unittest.BlockHeaderWithParentFixture(parent)
		byID[child.ID()] = &child
		t.Log(child.Height)
		byHeight[child.Height] = &child

		parent = &child
	}

	// should return error and not call callback when start block doesn't exist
	t.Run("non-existent start block", func(t *testing.T) {
		start := unittest.IdentifierFixture()
		end := unittest.IdentifierFixture()
		err := TraverseParentFirst(headers, start, end, func(_ *flow.Header) error {
			// should not be called
			t.Fail()
			return nil
		})
		assert.Error(t, err)
	})

	// should return error when end block doesn't exist
	t.Run("non-existent end block", func(t *testing.T) {
		start := byHeight[8].ID()
		end := unittest.IdentifierFixture()
		err := TraverseParentFirst(headers, start, end, func(_ *flow.Header) error {
			return nil
		})
		assert.Error(t, err)
	})

	// should return error if the callback returns an error
	t.Run("callback error", func(t *testing.T) {
		start := byHeight[8].ID()
		end := byHeight[5].ID()
		err := TraverseParentFirst(headers, start, end, func(_ *flow.Header) error {
			return fmt.Errorf("callback error")
		})
		assert.Error(t, err)
	})

	// should call the callback exactly once and not return an error when start == end
	t.Run("single-block traversal", func(t *testing.T) {
		start := byHeight[5].ID()
		end := byHeight[4].ID()

		called := 0
		err := TraverseParentFirst(headers, start, end, func(header *flow.Header) error {
			// should call callback for single block in traversal path
			assert.Equal(t, start, header.ID())
			// track calls - should only be called once
			called++
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, called)
	})

	// should call the callback exactly once for each block in traversal path
	// and not return an error
	t.Run("multi-block traversal", func(t *testing.T) {
		startHeight := uint64(8)
		endHeight := uint64(4)

		start := byHeight[startHeight].ID()
		end := byHeight[endHeight].ID()

		// assert that we are receiving the correct block at each height
		height := endHeight + 1
		err := TraverseParentFirst(headers, start, end, func(header *flow.Header) error {
			expectedID := byHeight[height].ID()
			assert.Equal(t, height, header.Height)
			assert.Equal(t, expectedID, header.ID())
			height++
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, height, startHeight+1)
	})
}
