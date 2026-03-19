package extended

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/onflow/cadence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended/transfers/testutil"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

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

func (m *mockFTBootstrapper) ByAddress(_ flow.Address, _ *access.TransferCursor) (storage.FungibleTokenTransferIterator, error) {
	return nil, nil
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

// ===== TestFilterFTTransfers =====

func TestFilterFTTransfers(t *testing.T) {
	t.Parallel()

	a := &FungibleTokenTransfers{}

	t.Run("empty input returns empty output", func(t *testing.T) {
		result := a.filterFTTransfers(nil)
		assert.Empty(t, result)
	})

	t.Run("filters out zero-amount transfers", func(t *testing.T) {
		addr := flow.HexToAddress("0x1234567890abcdef")
		transfers := []access.FungibleTokenTransfer{
			{
				RecipientAddress: addr,
				Amount:           big.NewInt(0),
				TokenType:        "A.0x1.FlowToken",
			},
		}

		result := a.filterFTTransfers(transfers)
		assert.Empty(t, result)
	})

	t.Run("non-zero amount transfers are kept regardless of recipient", func(t *testing.T) {
		sc := systemcontracts.SystemContractsForChain(flow.Testnet)
		flowFeesAddress := sc.FlowFees.Address
		otherAddress := flow.HexToAddress("0x1234567890abcdef")

		transfers := []access.FungibleTokenTransfer{
			{
				RecipientAddress: flowFeesAddress,
				Amount:           big.NewInt(100),
				TokenType:        "A.0x1.FlowToken",
			},
			{
				RecipientAddress: otherAddress,
				Amount:           big.NewInt(200),
				TokenType:        "A.0x1.FlowToken",
			},
		}

		result := a.filterFTTransfers(transfers)
		require.Len(t, result, 2)
	})

	t.Run("mixed: only non-zero amount transfers are kept", func(t *testing.T) {
		addr := flow.HexToAddress("0x1234567890abcdef")
		transfers := []access.FungibleTokenTransfer{
			{
				RecipientAddress: addr,
				Amount:           big.NewInt(200),
				TokenType:        "A.0x1.FlowToken",
			},
			{
				RecipientAddress: addr,
				Amount:           big.NewInt(0),
				TokenType:        "A.0x1.FlowToken",
			},
			{
				RecipientAddress: addr,
				Amount:           big.NewInt(999),
				TokenType:        "A.0x2.USDC",
			},
		}

		result := a.filterFTTransfers(transfers)
		require.Len(t, result, 2)
		assert.Equal(t, big.NewInt(200), result[0].Amount)
		assert.Equal(t, big.NewInt(999), result[1].Amount)
	})

	t.Run("filters out self-transfers", func(t *testing.T) {
		addr := flow.HexToAddress("0x1234567890abcdef")
		transfers := []access.FungibleTokenTransfer{
			{
				SourceAddress:    addr,
				RecipientAddress: addr,
				Amount:           big.NewInt(100),
				TokenType:        "A.0x1.FlowToken",
			},
		}

		result := a.filterFTTransfers(transfers)
		assert.Empty(t, result)
	})

	t.Run("filters out zero-address self-transfer", func(t *testing.T) {
		// Both source and recipient are the zero address (e.g. a mint with no from/to).
		transfers := []access.FungibleTokenTransfer{
			{
				SourceAddress:    flow.Address{},
				RecipientAddress: flow.Address{},
				Amount:           big.NewInt(100),
				TokenType:        "A.0x1.FlowToken",
			},
		}

		result := a.filterFTTransfers(transfers)
		assert.Empty(t, result)
	})

	t.Run("keeps transfer with distinct source and recipient", func(t *testing.T) {
		src := flow.HexToAddress("0x0000000000000001")
		dst := flow.HexToAddress("0x0000000000000002")
		transfers := []access.FungibleTokenTransfer{
			{
				SourceAddress:    src,
				RecipientAddress: dst,
				Amount:           big.NewInt(100),
				TokenType:        "A.0x1.FlowToken",
			},
		}

		result := a.filterFTTransfers(transfers)
		require.Len(t, result, 1)
		assert.Equal(t, src, result[0].SourceAddress)
		assert.Equal(t, dst, result[0].RecipientAddress)
	})

	t.Run("mixed: self-transfers filtered, regular transfers kept", func(t *testing.T) {
		addrA := flow.HexToAddress("0x0000000000000001")
		addrB := flow.HexToAddress("0x0000000000000002")
		transfers := []access.FungibleTokenTransfer{
			{SourceAddress: addrA, RecipientAddress: addrA, Amount: big.NewInt(100), TokenType: "A.0x1.FlowToken"},
			{SourceAddress: addrA, RecipientAddress: addrB, Amount: big.NewInt(200), TokenType: "A.0x1.FlowToken"},
			{SourceAddress: addrB, RecipientAddress: addrB, Amount: big.NewInt(300), TokenType: "A.0x1.FlowToken"},
		}

		result := a.filterFTTransfers(transfers)
		require.Len(t, result, 1)
		assert.Equal(t, addrA, result[0].SourceAddress)
		assert.Equal(t, addrB, result[0].RecipientAddress)
		assert.Equal(t, big.NewInt(200), result[0].Amount)
	})
}

// ===== TestFungibleTokenTransfers_NextHeight =====

func TestFungibleTokenTransfers_NextHeight(t *testing.T) {
	t.Parallel()

	t.Run("initialized store returns latestHeight+1", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{
			latestHeight:    99,
			latestHeightErr: nil,
		}

		a := &FungibleTokenTransfers{ftStore: ftStore}
		height, err := a.NextHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(100), height)
	})

	t.Run("uninitialized store returns firstHeight", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{
			latestHeightErr: storage.ErrNotBootstrapped,
			firstHeight:     50,
			isInitialized:   false,
		}

		a := &FungibleTokenTransfers{ftStore: ftStore}
		height, err := a.NextHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(50), height)
	})

	t.Run("store error propagates", func(t *testing.T) {
		ftErr := fmt.Errorf("FT storage failure")
		ftStore := &mockFTBootstrapper{
			latestHeightErr: ftErr,
		}

		a := &FungibleTokenTransfers{ftStore: ftStore}
		_, err := a.NextHeight()
		require.Error(t, err)
		assert.ErrorIs(t, err, ftErr)
	})
}

// ===== TestFungibleTokenTransfers_Name =====

func TestFungibleTokenTransfers_Name(t *testing.T) {
	t.Parallel()

	a := &FungibleTokenTransfers{}
	assert.Equal(t, "account_ft_transfers", a.Name())
}

// ===== TestFungibleTokenTransfers_ProcessBlockData =====

func TestFungibleTokenTransfers_ProcessBlockData(t *testing.T) {
	t.Parallel()

	t.Run("empty block returns empty entries and zero filtered count", func(t *testing.T) {
		a := NewFungibleTokenTransfers(unittest.Logger(), flow.Testnet, &mockFTBootstrapper{latestHeight: 99}, metrics.NewNoopCollector())

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: []flow.Event{},
		}

		entries, meta, err := a.ProcessBlockData(data)
		require.NoError(t, err)
		assert.Empty(t, entries)
		assert.Equal(t, 0, meta.FilteredCount)
	})

	t.Run("self-transfer is filtered and counted in metadata", func(t *testing.T) {
		a := NewFungibleTokenTransfers(unittest.Logger(), flow.Testnet, &mockFTBootstrapper{latestHeight: 99}, metrics.NewNoopCollector())

		addr := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()
		amount := cadence.UFix64(5_00000000)

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: []flow.Event{
				testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &addr, txID, 0, 0, 1, 50, amount),
				testutil.MakeFTDepositedEvent(t, flow.Testnet, &addr, txID, 0, 1, 1, 50, amount),
			},
		}

		entries, meta, err := a.ProcessBlockData(data)
		require.NoError(t, err)
		assert.Empty(t, entries)
		assert.Equal(t, 1, meta.FilteredCount)
	})

	t.Run("valid transfer returned with zero filtered count", func(t *testing.T) {
		a := NewFungibleTokenTransfers(unittest.Logger(), flow.Testnet, &mockFTBootstrapper{latestHeight: 99}, metrics.NewNoopCollector())

		src := unittest.RandomAddressFixture()
		dst := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()
		amount := cadence.UFix64(10_00000000)

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: []flow.Event{
				testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &src, txID, 0, 0, 1, 50, amount),
				testutil.MakeFTDepositedEvent(t, flow.Testnet, &dst, txID, 0, 1, 1, 50, amount),
			},
		}

		entries, meta, err := a.ProcessBlockData(data)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, src, entries[0].SourceAddress)
		assert.Equal(t, dst, entries[0].RecipientAddress)
		assert.Equal(t, 0, meta.FilteredCount)
	})

	t.Run("does not depend on indexer height state", func(t *testing.T) {
		// ProcessBlockData should work regardless of the indexer's height state
		a := NewFungibleTokenTransfers(unittest.Logger(), flow.Testnet, &mockFTBootstrapper{latestHeight: 50}, metrics.NewNoopCollector())

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(200)),
			Events: []flow.Event{},
		}

		entries, meta, err := a.ProcessBlockData(data)
		require.NoError(t, err)
		assert.Empty(t, entries)
		assert.Equal(t, 0, meta.FilteredCount)
	})
}

// ===== TestFungibleTokenTransfers_IndexBlockData =====

func TestFungibleTokenTransfers_IndexBlockData(t *testing.T) {
	t.Parallel()

	t.Run("empty block stores empty transfer slice", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{latestHeight: 99}
		a := NewFungibleTokenTransfers(unittest.Logger(), flow.Testnet, ftStore, metrics.NewNoopCollector())

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: []flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.NoError(t, err)
		assert.Equal(t, uint64(100), ftStore.storedHeight)
		assert.Empty(t, ftStore.storedTransfers)
	})

	t.Run("future height returns ErrFutureHeight", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{latestHeight: 99}
		a := NewFungibleTokenTransfers(unittest.Logger(), flow.Testnet, ftStore, metrics.NewNoopCollector())

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(101)), // next expected is 100
			Events: []flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.ErrorIs(t, err, ErrFutureHeight)
	})

	t.Run("already indexed returns ErrAlreadyIndexed", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{latestHeight: 99}
		a := NewFungibleTokenTransfers(unittest.Logger(), flow.Testnet, ftStore, metrics.NewNoopCollector())

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(99)), // next expected is 100
			Events: []flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.ErrorIs(t, err, ErrAlreadyIndexed)
	})

	t.Run("NextHeight error propagates", func(t *testing.T) {
		nextHeightErr := fmt.Errorf("next height failure")
		ftStore := &mockFTBootstrapper{latestHeightErr: nextHeightErr}
		a := NewFungibleTokenTransfers(unittest.Logger(), flow.Testnet, ftStore, metrics.NewNoopCollector())

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: []flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, nextHeightErr)
	})

	t.Run("store error propagates", func(t *testing.T) {
		storeErr := fmt.Errorf("FT storage failure")
		ftStore := &mockFTBootstrapper{latestHeight: 99, storeErr: storeErr}
		a := NewFungibleTokenTransfers(unittest.Logger(), flow.Testnet, ftStore, metrics.NewNoopCollector())

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: []flow.Event{},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
	})

	// Tests that the flow fees transfer is excluded by the parser when a FeesDeducted event
	// is present, since the indexer is created with omitFlowFees=true.
	t.Run("flow fees transfer omitted when FeesDeducted event is present", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{latestHeight: 99}
		a := NewFungibleTokenTransfers(unittest.Logger(), flow.Testnet, ftStore, metrics.NewNoopCollector())

		payer := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()
		flowFeesAddress := testutil.FlowFeesAddress(flow.Testnet)
		feeAmount := cadence.UFix64(1_00000000)

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: []flow.Event{
				testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &payer, txID, 0, 0, 1, 50, feeAmount),
				testutil.MakeFTDepositedEvent(t, flow.Testnet, &flowFeesAddress, txID, 0, 1, 1, 50, feeAmount),
				testutil.MakeFlowFeesEvent(t, flow.Testnet, txID, 0, 2, feeAmount),
			},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.NoError(t, err)
		assert.Equal(t, uint64(100), ftStore.storedHeight)
		assert.Empty(t, ftStore.storedTransfers)
	})

	// Tests that a self-transfer (same source and recipient address) is not stored.
	t.Run("self-transfer is not stored", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{latestHeight: 99}
		a := NewFungibleTokenTransfers(unittest.Logger(), flow.Testnet, ftStore, metrics.NewNoopCollector())

		addr := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()
		amount := cadence.UFix64(5_00000000)

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: []flow.Event{
				testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &addr, txID, 0, 0, 1, 50, amount),
				testutil.MakeFTDepositedEvent(t, flow.Testnet, &addr, txID, 0, 1, 1, 50, amount),
			},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.NoError(t, err)
		assert.Equal(t, uint64(100), ftStore.storedHeight)
		assert.Empty(t, ftStore.storedTransfers)
	})

	// Tests that a regular transfer to the flow fees address (no FeesDeducted event) is indexed.
	// The parser only omits transfers that are paired with a FeesDeducted event.
	t.Run("transfer to flow fees address without FeesDeducted event is indexed", func(t *testing.T) {
		ftStore := &mockFTBootstrapper{latestHeight: 99}
		a := NewFungibleTokenTransfers(unittest.Logger(), flow.Testnet, ftStore, metrics.NewNoopCollector())

		payer := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()
		flowFeesAddress := testutil.FlowFeesAddress(flow.Testnet)
		amount := cadence.UFix64(5_00000000)

		data := BlockData{
			Header: unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100)),
			Events: []flow.Event{
				testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &payer, txID, 0, 0, 1, 50, amount),
				testutil.MakeFTDepositedEvent(t, flow.Testnet, &flowFeesAddress, txID, 0, 1, 1, 50, amount),
				// No FeesDeducted event — treated as a regular transfer.
			},
		}

		err := a.IndexBlockData(nil, data, nil)
		require.NoError(t, err)
		assert.Equal(t, uint64(100), ftStore.storedHeight)
		require.Len(t, ftStore.storedTransfers, 1)
	})
}
