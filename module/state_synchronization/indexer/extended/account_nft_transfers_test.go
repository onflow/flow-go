package extended

import (
	"fmt"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// mockNFTBootstrapper implements storage.NonFungibleTokenTransfersBootstrapper for testing.
type mockNFTBootstrapper struct {
	latestHeight    uint64
	latestHeightErr error
	firstHeight     uint64
	isInitialized   bool
	storeErr        error
	storedHeight    uint64
	storedTransfers []access.NonFungibleTokenTransfer
}

func (m *mockNFTBootstrapper) ByAddress(_ flow.Address, _ *access.TransferCursor) (storage.NonFungibleTokenTransferIterator, error) {
	return nil, nil
}

func (m *mockNFTBootstrapper) LatestIndexedHeight() (uint64, error) {
	return m.latestHeight, m.latestHeightErr
}

func (m *mockNFTBootstrapper) UninitializedFirstHeight() (uint64, bool) {
	return m.firstHeight, m.isInitialized
}

func (m *mockNFTBootstrapper) FirstIndexedHeight() (uint64, error) {
	if m.latestHeightErr != nil {
		return 0, m.latestHeightErr
	}
	return m.firstHeight, nil
}

func (m *mockNFTBootstrapper) Store(_ lockctx.Proof, _ storage.ReaderBatchWriter, height uint64, transfers []access.NonFungibleTokenTransfer) error {
	m.storedHeight = height
	m.storedTransfers = transfers
	return m.storeErr
}

// ===== TestNonFungibleTokenTransfers_NextHeight =====

func TestNonFungibleTokenTransfers_NextHeight(t *testing.T) {
	t.Parallel()

	t.Run("initialized store returns latestHeight+1", func(t *testing.T) {
		nftStore := &mockNFTBootstrapper{
			latestHeight:    99,
			latestHeightErr: nil,
		}

		a := &NonFungibleTokenTransfers{nftStore: nftStore}
		height, err := a.NextHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(100), height)
	})

	t.Run("uninitialized store returns firstHeight", func(t *testing.T) {
		nftStore := &mockNFTBootstrapper{
			latestHeightErr: storage.ErrNotBootstrapped,
			firstHeight:     50,
			isInitialized:   false,
		}

		a := &NonFungibleTokenTransfers{nftStore: nftStore}
		height, err := a.NextHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(50), height)
	})

	t.Run("store error propagates", func(t *testing.T) {
		nftErr := fmt.Errorf("NFT storage failure")
		nftStore := &mockNFTBootstrapper{
			latestHeightErr: nftErr,
		}

		a := &NonFungibleTokenTransfers{nftStore: nftStore}
		_, err := a.NextHeight()
		require.Error(t, err)
		assert.ErrorIs(t, err, nftErr)
	})
}

// ===== TestNonFungibleTokenTransfers_Name =====

func TestNonFungibleTokenTransfers_Name(t *testing.T) {
	t.Parallel()

	a := &NonFungibleTokenTransfers{}
	assert.Equal(t, "account_nft_transfers", a.Name())
}

// ===== TestNonFungibleTokenTransfers_IndexBlockData =====

func TestNonFungibleTokenTransfers_IndexBlockData(t *testing.T) {
	t.Parallel()

	t.Run("empty block stores empty transfer slice", func(t *testing.T) {
		nftStore := &mockNFTBootstrapper{latestHeight: 99}
		a := NewNonFungibleTokenTransfers(unittest.Logger(), flow.Testnet, nftStore, metrics.NewNoopCollector())

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: []flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.NoError(t, err)
		assert.Equal(t, uint64(100), nftStore.storedHeight)
		assert.Empty(t, nftStore.storedTransfers)
	})

	t.Run("future height returns ErrFutureHeight", func(t *testing.T) {
		nftStore := &mockNFTBootstrapper{latestHeight: 99}
		a := NewNonFungibleTokenTransfers(unittest.Logger(), flow.Testnet, nftStore, metrics.NewNoopCollector())

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(101)), // next expected is 100
			Events: []flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.ErrorIs(t, err, ErrFutureHeight)
	})

	t.Run("already indexed returns ErrAlreadyIndexed", func(t *testing.T) {
		nftStore := &mockNFTBootstrapper{latestHeight: 99}
		a := NewNonFungibleTokenTransfers(unittest.Logger(), flow.Testnet, nftStore, metrics.NewNoopCollector())

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(99)), // next expected is 100
			Events: []flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.ErrorIs(t, err, ErrAlreadyIndexed)
	})

	t.Run("NextHeight error propagates", func(t *testing.T) {
		nextHeightErr := fmt.Errorf("next height failure")
		nftStore := &mockNFTBootstrapper{latestHeightErr: nextHeightErr}
		a := NewNonFungibleTokenTransfers(unittest.Logger(), flow.Testnet, nftStore, metrics.NewNoopCollector())

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: []flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, nextHeightErr)
	})

	t.Run("store error propagates", func(t *testing.T) {
		storeErr := fmt.Errorf("NFT storage failure")
		nftStore := &mockNFTBootstrapper{latestHeight: 99, storeErr: storeErr}
		a := NewNonFungibleTokenTransfers(unittest.Logger(), flow.Testnet, nftStore, metrics.NewNoopCollector())

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: []flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
	})
}
