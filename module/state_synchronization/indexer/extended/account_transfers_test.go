package extended

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// ===== Mock Types =====

// mockHeightProvider is a simple mock implementing the heightProvider interface.
type mockHeightProvider struct {
	latestHeight    uint64
	latestHeightErr error
	firstHeight     uint64
	isInitialized   bool
}

func (m *mockHeightProvider) LatestIndexedHeight() (uint64, error) {
	return m.latestHeight, m.latestHeightErr
}

func (m *mockHeightProvider) UninitializedFirstHeight() (uint64, bool) {
	return m.firstHeight, m.isInitialized
}

// mockFTBootstrapper implements storage.FungibleTokenTransfersBootstrapper for testing.
type mockFTBootstrapper struct {
	latestHeight    uint64
	latestHeightErr error
	firstHeight     uint64
	isInitialized   bool
	storeErr        error
	storedHeight    uint64
	storedTransfers []access.FungibleTokenTransfer
}

func (m *mockFTBootstrapper) TransfersByAddress(flow.Address, uint32, *access.TransferCursor, storage.IndexFilter[*access.FungibleTokenTransfer]) (access.FungibleTokenTransfersPage, error) {
	return access.FungibleTokenTransfersPage{}, nil
}

func (m *mockFTBootstrapper) LatestIndexedHeight() (uint64, error) {
	return m.latestHeight, m.latestHeightErr
}

func (m *mockFTBootstrapper) UninitializedFirstHeight() (uint64, bool) {
	return m.firstHeight, m.isInitialized
}

func (m *mockFTBootstrapper) FirstIndexedHeight() (uint64, error) {
	if m.latestHeightErr != nil {
		return 0, m.latestHeightErr
	}
	return m.firstHeight, nil
}

func (m *mockFTBootstrapper) Store(_ lockctx.Proof, _ storage.ReaderBatchWriter, height uint64, transfers []access.FungibleTokenTransfer) error {
	m.storedHeight = height
	m.storedTransfers = transfers
	return m.storeErr
}

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

func (m *mockNFTBootstrapper) TransfersByAddress(flow.Address, uint32, *access.TransferCursor, storage.IndexFilter[*access.NonFungibleTokenTransfer]) (access.NonFungibleTokenTransfersPage, error) {
	return access.NonFungibleTokenTransfersPage{}, nil
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

// ===== TestNextHeight =====

func TestNextHeight(t *testing.T) {
	t.Parallel()

	t.Run("initialized store returns latestHeight+1", func(t *testing.T) {
		store := &mockHeightProvider{
			latestHeight:    99,
			latestHeightErr: nil,
		}

		height, err := nextHeight(store)
		require.NoError(t, err)
		assert.Equal(t, uint64(100), height)
	})

	t.Run("uninitialized store returns firstHeight", func(t *testing.T) {
		store := &mockHeightProvider{
			latestHeightErr: storage.ErrNotBootstrapped,
			firstHeight:     42,
			isInitialized:   false,
		}

		height, err := nextHeight(store)
		require.NoError(t, err)
		assert.Equal(t, uint64(42), height)
	})

	t.Run("unexpected error from LatestIndexedHeight propagates", func(t *testing.T) {
		unexpectedErr := fmt.Errorf("disk I/O error")
		store := &mockHeightProvider{
			latestHeightErr: unexpectedErr,
		}

		_, err := nextHeight(store)
		require.Error(t, err)
		assert.ErrorIs(t, err, unexpectedErr)
	})

	t.Run("inconsistent state: not bootstrapped but isInitialized returns error", func(t *testing.T) {
		store := &mockHeightProvider{
			latestHeightErr: storage.ErrNotBootstrapped,
			firstHeight:     42,
			isInitialized:   true,
		}

		_, err := nextHeight(store)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "but index is initialized")
	})
}

// ===== TestFilterFTTransfers =====

func TestFilterFTTransfers(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	flowFeesAddress := sc.FlowFees.Address

	a := &AccountTransfers{
		flowFeesAddress: flowFeesAddress,
	}

	t.Run("empty input returns empty output", func(t *testing.T) {
		result := a.filterFTTransfers(nil)
		assert.Empty(t, result)
	})

	// NOTE: The filter logic is currently inverted. Despite the comment in the source saying
	// "skip flow fee deposits", the code actually KEEPS only transfers TO the flow fees address
	// (it continues/skips when RecipientAddress != flowFeesAddress). These tests verify the
	// current behavior, not the intended behavior described in the comments.

	t.Run("keeps transfers TO flow fees address with non-zero amount (current inverted behavior)", func(t *testing.T) {
		transfers := []access.FungibleTokenTransfer{
			{
				RecipientAddress: flowFeesAddress,
				Amount:           big.NewInt(100),
				TokenType:        "A.0x1.FlowToken",
			},
		}

		result := a.filterFTTransfers(transfers)
		require.Len(t, result, 1)
		assert.Equal(t, flowFeesAddress, result[0].RecipientAddress)
	})

	t.Run("filters out transfers NOT to flow fees address (current inverted behavior)", func(t *testing.T) {
		otherAddress := flow.HexToAddress("0x1234567890abcdef")
		transfers := []access.FungibleTokenTransfer{
			{
				RecipientAddress: otherAddress,
				Amount:           big.NewInt(100),
				TokenType:        "A.0x1.FlowToken",
			},
		}

		result := a.filterFTTransfers(transfers)
		assert.Empty(t, result)
	})

	t.Run("filters out zero-amount transfers to flow fees address", func(t *testing.T) {
		transfers := []access.FungibleTokenTransfer{
			{
				RecipientAddress: flowFeesAddress,
				Amount:           big.NewInt(0),
				TokenType:        "A.0x1.FlowToken",
			},
		}

		result := a.filterFTTransfers(transfers)
		assert.Empty(t, result)
	})

	t.Run("mixed transfers: only non-zero amount transfers to flow fees address are kept", func(t *testing.T) {
		otherAddress := flow.HexToAddress("0x1234567890abcdef")
		transfers := []access.FungibleTokenTransfer{
			{
				// kept: to flow fees, non-zero amount
				RecipientAddress: flowFeesAddress,
				Amount:           big.NewInt(500),
				TokenType:        "A.0x1.FlowToken",
			},
			{
				// filtered: not to flow fees
				RecipientAddress: otherAddress,
				Amount:           big.NewInt(200),
				TokenType:        "A.0x1.FlowToken",
			},
			{
				// filtered: to flow fees but zero amount
				RecipientAddress: flowFeesAddress,
				Amount:           big.NewInt(0),
				TokenType:        "A.0x1.FlowToken",
			},
			{
				// kept: to flow fees, non-zero amount
				RecipientAddress: flowFeesAddress,
				Amount:           big.NewInt(1),
				TokenType:        "A.0x1.FlowToken",
			},
			{
				// filtered: not to flow fees
				RecipientAddress: otherAddress,
				Amount:           big.NewInt(999),
				TokenType:        "A.0x2.USDC",
			},
		}

		result := a.filterFTTransfers(transfers)
		require.Len(t, result, 2)
		assert.Equal(t, big.NewInt(500), result[0].Amount)
		assert.Equal(t, big.NewInt(1), result[1].Amount)
	})
}

// ===== TestFlattenEvents =====

func TestFlattenEvents(t *testing.T) {
	t.Parallel()

	t.Run("nil map returns nil", func(t *testing.T) {
		result := flattenEvents(nil)
		assert.Nil(t, result)
	})

	t.Run("empty map returns nil", func(t *testing.T) {
		result := flattenEvents(map[uint32][]flow.Event{})
		assert.Nil(t, result)
	})

	t.Run("single tx single event", func(t *testing.T) {
		event := flow.Event{TransactionIndex: 0, EventIndex: 0, Type: "A.Test.Foo"}
		result := flattenEvents(map[uint32][]flow.Event{
			0: {event},
		})
		require.Len(t, result, 1)
		assert.Equal(t, event, result[0])
	})

	t.Run("single tx multiple events preserves order", func(t *testing.T) {
		event0 := flow.Event{TransactionIndex: 0, EventIndex: 0, Type: "A.Test.First"}
		event1 := flow.Event{TransactionIndex: 0, EventIndex: 1, Type: "A.Test.Second"}
		event2 := flow.Event{TransactionIndex: 0, EventIndex: 2, Type: "A.Test.Third"}
		result := flattenEvents(map[uint32][]flow.Event{
			0: {event0, event1, event2},
		})
		require.Len(t, result, 3)
		assert.Equal(t, event0, result[0])
		assert.Equal(t, event1, result[1])
		assert.Equal(t, event2, result[2])
	})

	t.Run("multiple txs multiple events all included", func(t *testing.T) {
		eventsA := []flow.Event{
			{TransactionIndex: 0, EventIndex: 0, Type: "A.Test.A0"},
			{TransactionIndex: 0, EventIndex: 1, Type: "A.Test.A1"},
		}
		eventsB := []flow.Event{
			{TransactionIndex: 1, EventIndex: 0, Type: "A.Test.B0"},
		}
		eventsC := []flow.Event{
			{TransactionIndex: 2, EventIndex: 0, Type: "A.Test.C0"},
			{TransactionIndex: 2, EventIndex: 1, Type: "A.Test.C1"},
			{TransactionIndex: 2, EventIndex: 2, Type: "A.Test.C2"},
		}
		result := flattenEvents(map[uint32][]flow.Event{
			0: eventsA,
			1: eventsB,
			2: eventsC,
		})
		require.Len(t, result, 6)

		// Map iteration order is non-deterministic, so verify all events are present
		// by checking set membership rather than exact ordering across tx groups.
		eventTypes := make(map[flow.EventType]bool)
		for _, e := range result {
			eventTypes[e.Type] = true
		}
		assert.True(t, eventTypes["A.Test.A0"])
		assert.True(t, eventTypes["A.Test.A1"])
		assert.True(t, eventTypes["A.Test.B0"])
		assert.True(t, eventTypes["A.Test.C0"])
		assert.True(t, eventTypes["A.Test.C1"])
		assert.True(t, eventTypes["A.Test.C2"])
	})

	t.Run("events order is preserved within each tx group", func(t *testing.T) {
		events := []flow.Event{
			{TransactionIndex: 5, EventIndex: 0, Type: "A.Test.First"},
			{TransactionIndex: 5, EventIndex: 1, Type: "A.Test.Second"},
			{TransactionIndex: 5, EventIndex: 2, Type: "A.Test.Third"},
		}
		result := flattenEvents(map[uint32][]flow.Event{
			5: events,
		})
		require.Len(t, result, 3)
		for i, e := range result {
			assert.Equal(t, events[i], e)
		}
	})
}

// ===== TestAccountTransfers_NextHeight =====

func TestAccountTransfers_NextHeight(t *testing.T) {
	t.Parallel()

	t.Run("FT and NFT stores in sync returns correct height", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{
			latestHeight:    99,
			latestHeightErr: nil,
		}
		nftStore := &mockNFTBootstrapper{
			latestHeight:    99,
			latestHeightErr: nil,
		}

		a := &AccountTransfers{
			ftStore:  ftStore,
			nftStore: nftStore,
		}

		height, err := a.NextHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(100), height)
	})

	t.Run("FT and NFT stores both uninitialized and in sync", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{
			latestHeightErr: storage.ErrNotBootstrapped,
			firstHeight:     50,
			isInitialized:   false,
		}
		nftStore := &mockNFTBootstrapper{
			latestHeightErr: storage.ErrNotBootstrapped,
			firstHeight:     50,
			isInitialized:   false,
		}

		a := &AccountTransfers{
			ftStore:  ftStore,
			nftStore: nftStore,
		}

		height, err := a.NextHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(50), height)
	})

	t.Run("FT and NFT stores out of sync returns error", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{
			latestHeight:    99,
			latestHeightErr: nil,
		}
		nftStore := &mockNFTBootstrapper{
			latestHeight:    100,
			latestHeightErr: nil,
		}

		a := &AccountTransfers{
			ftStore:  ftStore,
			nftStore: nftStore,
		}

		_, err := a.NextHeight()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "out of sync")
	})

	t.Run("FT store error propagates", func(t *testing.T) {
		ftErr := fmt.Errorf("FT storage failure")
		ftStore := &mockFTBootstrapper{
			latestHeightErr: ftErr,
		}
		nftStore := &mockNFTBootstrapper{
			latestHeight:    99,
			latestHeightErr: nil,
		}

		a := &AccountTransfers{
			ftStore:  ftStore,
			nftStore: nftStore,
		}

		_, err := a.NextHeight()
		require.Error(t, err)
		assert.ErrorIs(t, err, ftErr)
	})

	t.Run("NFT store error propagates", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{
			latestHeight:    99,
			latestHeightErr: nil,
		}
		nftErr := fmt.Errorf("NFT storage failure")
		nftStore := &mockNFTBootstrapper{
			latestHeightErr: nftErr,
		}

		a := &AccountTransfers{
			ftStore:  ftStore,
			nftStore: nftStore,
		}

		_, err := a.NextHeight()
		require.Error(t, err)
		assert.ErrorIs(t, err, nftErr)
	})
}

// ===== TestAccountTransfers_Name =====

func TestAccountTransfers_Name(t *testing.T) {
	t.Parallel()

	a := &AccountTransfers{}
	assert.Equal(t, "account_transfers", a.Name())
}

// ===== TestAccountTransfers_IndexBlockData =====

func TestAccountTransfers_IndexBlockData(t *testing.T) {
	t.Parallel()

	t.Run("empty block stores empty transfer slices", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{latestHeight: 99}
		nftStore := &mockNFTBootstrapper{latestHeight: 99}
		a := NewAccountTransfers(unittest.Logger(), flow.Testnet, ftStore, nftStore)

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: map[uint32][]flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.NoError(t, err)
		assert.Equal(t, uint64(100), ftStore.storedHeight)
		assert.Equal(t, uint64(100), nftStore.storedHeight)
		assert.Empty(t, ftStore.storedTransfers)
		assert.Empty(t, nftStore.storedTransfers)
	})

	t.Run("future height returns ErrFutureHeight", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{latestHeight: 99}
		nftStore := &mockNFTBootstrapper{latestHeight: 99}
		a := NewAccountTransfers(unittest.Logger(), flow.Testnet, ftStore, nftStore)

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(101)), // next expected is 100
			Events: map[uint32][]flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.ErrorIs(t, err, ErrFutureHeight)
	})

	t.Run("already indexed returns ErrAlreadyIndexed", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{latestHeight: 99}
		nftStore := &mockNFTBootstrapper{latestHeight: 99}
		a := NewAccountTransfers(unittest.Logger(), flow.Testnet, ftStore, nftStore)

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(99)), // next expected is 100
			Events: map[uint32][]flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.ErrorIs(t, err, ErrAlreadyIndexed)
	})

	t.Run("NextHeight error propagates", func(t *testing.T) {
		nextHeightErr := fmt.Errorf("next height failure")
		ftStore := &mockFTBootstrapper{latestHeightErr: nextHeightErr}
		nftStore := &mockNFTBootstrapper{latestHeight: 99}
		a := NewAccountTransfers(unittest.Logger(), flow.Testnet, ftStore, nftStore)

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: map[uint32][]flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, nextHeightErr)
	})

	t.Run("FT store error propagates", func(t *testing.T) {
		storeErr := fmt.Errorf("FT storage failure")
		ftStore := &mockFTBootstrapper{latestHeight: 99, storeErr: storeErr}
		nftStore := &mockNFTBootstrapper{latestHeight: 99}
		a := NewAccountTransfers(unittest.Logger(), flow.Testnet, ftStore, nftStore)

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: map[uint32][]flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
	})

	t.Run("NFT store error propagates", func(t *testing.T) {
		storeErr := fmt.Errorf("NFT storage failure")
		ftStore := &mockFTBootstrapper{latestHeight: 99}
		nftStore := &mockNFTBootstrapper{latestHeight: 99, storeErr: storeErr}
		a := NewAccountTransfers(unittest.Logger(), flow.Testnet, ftStore, nftStore)

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: map[uint32][]flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
	})
}
