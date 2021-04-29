package protocol_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/context"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	storageMock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func doTest(t *testing.T, f func(*testing.T, *storageMock.Headers, context.Blocks, flow.Header)) func(*testing.T) {
	return func(t *testing.T) {

		headers := new(storageMock.Headers)
		blockFinder := protocol.NewBlockFinder(headers)
		header := unittest.BlockHeaderFixture()
		header.Height = 10

		f(t, headers, blockFinder, header)

		headers.AssertExpectations(t)
	}
}

func Test_BlockFinder_ReturnsHeaderIfSameHeight(t *testing.T) {

	t.Run("returns header is height is the same", doTest(t, func(t *testing.T, headers *storageMock.Headers, blockFinder Blocks, header flow.Header) {
		heightFrom, err := blockFinder.ByHeightFrom(10, &header)
		require.NoError(t, err)
		require.Equal(t, header, *heightFrom)
	}))

	t.Run("nil header defaults to ByHeight", doTest(t, func(t *testing.T, headers *storageMock.Headers, blockFinder Blocks, header flow.Header) {

		headers.On("ByHeight", uint64(10)).Return(&header, nil)

		heightFrom, err := blockFinder.ByHeightFrom(10, nil)
		require.NoError(t, err)
		require.Equal(t, header, *heightFrom)
	}))

	t.Run("follows blocks chain", doTest(t, func(t *testing.T, headers *storageMock.Headers, blockFinder Blocks, header flow.Header) {

		header0 := unittest.BlockHeaderFixture()
		header0.Height = 0
		header1 := unittest.BlockHeaderWithParentFixture(&header0)
		header2 := unittest.BlockHeaderWithParentFixture(&header1)
		header3 := unittest.BlockHeaderWithParentFixture(&header2)
		header4 := unittest.BlockHeaderWithParentFixture(&header3)
		header5 := unittest.BlockHeaderWithParentFixture(&header4)

		headers.On("ByBlockID", header4.ID()).Return(&header4, nil)
		headers.On("ByHeight", uint64(4)).Return(nil, storage.ErrNotFound)
		headers.On("ByBlockID", header3.ID()).Return(&header3, nil)
		headers.On("ByHeight", uint64(3)).Return(nil, storage.ErrNotFound)
		headers.On("ByBlockID", header2.ID()).Return(&header2, nil)
		headers.On("ByHeight", uint64(2)).Return(nil, storage.ErrNotFound)
		headers.On("ByBlockID", header1.ID()).Return(&header1, nil)
		headers.On("ByHeight", uint64(1)).Return(nil, storage.ErrNotFound)
		headers.On("ByBlockID", header0.ID()).Return(&header0, nil)
		//headers.On("ByHeight", uint64(0)).Return(nil, storage.ErrNotFound)

		heightFrom, err := blockFinder.ByHeightFrom(0, &header5)
		require.NoError(t, err)
		require.Equal(t, header0, *heightFrom)
	}))

	t.Run("skips heights once it get to finalized chain", doTest(t, func(t *testing.T, headers *storageMock.Headers, blockFinder Blocks, header flow.Header) {

		header0 := unittest.BlockHeaderFixture()
		header0.Height = 0
		header1 := unittest.BlockHeaderWithParentFixture(&header0)
		header2 := unittest.BlockHeaderWithParentFixture(&header1)
		header3 := unittest.BlockHeaderWithParentFixture(&header2)
		header4 := unittest.BlockHeaderWithParentFixture(&header3)
		header5 := unittest.BlockHeaderWithParentFixture(&header4)

		headers.On("ByBlockID", header4.ID()).Return(&header4, nil)
		headers.On("ByHeight", uint64(4)).Return(nil, storage.ErrNotFound)
		headers.On("ByBlockID", header3.ID()).Return(&header3, nil)
		headers.On("ByHeight", uint64(3)).Return(&header3, nil)
		headers.On("ByHeight", uint64(0)).Return(&header0, nil)

		heightFrom, err := blockFinder.ByHeightFrom(0, &header5)
		require.NoError(t, err)
		require.Equal(t, header0, *heightFrom)
	}))

}
