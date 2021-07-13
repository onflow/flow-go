package bfinder_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	bfinder "github.com/onflow/flow-go/engine/execution/computation/blocks"
	"github.com/onflow/flow-go/storage"
	storageMock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func Test_BlockFinder_ReturnsHeaderIfSameHeight(t *testing.T) {

	t.Run("returns header is height is the same", func(t *testing.T) {
		headers := new(storageMock.Headers)
		header := unittest.BlockHeaderFixture()
		header.Height = 5
		blockFinder := bfinder.NewBlockFinder(&header, headers, 0, 5)
		heightFrom, found, err := blockFinder.ByHeight(5)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, bfinder.RuntimeBlockFromFlowHeader(&header), heightFrom)

		headers.AssertExpectations(t)
	})

	t.Run("nil header defaults to ByHeight", func(t *testing.T) {
		headers := new(storageMock.Headers)
		header := unittest.BlockHeaderFixture()
		header.Height = 5
		blockFinder := bfinder.NewBlockFinder(nil, headers, 0, 5)

		headers.On("ByHeight", uint64(5)).Return(&header, nil)
		heightFrom, found, err := blockFinder.ByHeight(5)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, bfinder.RuntimeBlockFromFlowHeader(&header), heightFrom)
		headers.AssertExpectations(t)
	})

	t.Run("error for heights in the future", func(t *testing.T) {
		headers := new(storageMock.Headers)
		header := unittest.BlockHeaderFixture()
		header.Height = 5
		blockFinder := bfinder.NewBlockFinder(nil, headers, 0, 5)

		_, found, err := blockFinder.ByHeight(6)
		require.Error(t, err)
		require.False(t, found)
		headers.AssertExpectations(t)
	})

	t.Run("follows blocks chain", func(t *testing.T) {

		header0 := unittest.BlockHeaderFixture()
		header0.Height = 0
		header1 := unittest.BlockHeaderWithParentFixture(&header0)
		header2 := unittest.BlockHeaderWithParentFixture(&header1)
		header3 := unittest.BlockHeaderWithParentFixture(&header2)
		header4 := unittest.BlockHeaderWithParentFixture(&header3)
		header5 := unittest.BlockHeaderWithParentFixture(&header4)

		headers := new(storageMock.Headers)
		headers.On("ByBlockID", header4.ID()).Return(&header4, nil)
		headers.On("ByHeight", uint64(4)).Return(nil, storage.ErrNotFound)
		headers.On("ByBlockID", header3.ID()).Return(&header3, nil)
		headers.On("ByHeight", uint64(3)).Return(nil, storage.ErrNotFound)
		headers.On("ByBlockID", header2.ID()).Return(&header2, nil)
		headers.On("ByHeight", uint64(2)).Return(nil, storage.ErrNotFound)
		headers.On("ByBlockID", header1.ID()).Return(&header1, nil)
		headers.On("ByHeight", uint64(1)).Return(nil, storage.ErrNotFound)
		headers.On("ByBlockID", header0.ID()).Return(&header0, nil)

		blockFinder := bfinder.NewBlockFinder(&header5, headers, 0, 5)

		heightFrom, found, err := blockFinder.ByHeight(0)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, bfinder.RuntimeBlockFromFlowHeader(&header0), heightFrom)
		headers.AssertExpectations(t)
	})

	t.Run("skips heights once it get to finalized chain", func(t *testing.T) {

		header0 := unittest.BlockHeaderFixture()
		header0.Height = 0
		header1 := unittest.BlockHeaderWithParentFixture(&header0)
		header2 := unittest.BlockHeaderWithParentFixture(&header1)
		header3 := unittest.BlockHeaderWithParentFixture(&header2)
		header4 := unittest.BlockHeaderWithParentFixture(&header3)
		header5 := unittest.BlockHeaderWithParentFixture(&header4)

		headers := new(storageMock.Headers)
		headers.On("ByBlockID", header4.ID()).Return(&header4, nil)
		headers.On("ByHeight", uint64(4)).Return(nil, storage.ErrNotFound)
		headers.On("ByBlockID", header3.ID()).Return(&header3, nil)
		headers.On("ByHeight", uint64(3)).Return(&header3, nil)
		headers.On("ByHeight", uint64(0)).Return(&header0, nil)

		blockFinder := bfinder.NewBlockFinder(&header5, headers, 0, 5)

		heightFrom, found, err := blockFinder.ByHeight(0)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, bfinder.RuntimeBlockFromFlowHeader(&header0), heightFrom)
		headers.AssertExpectations(t)
	})

}
