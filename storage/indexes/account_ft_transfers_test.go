package indexes

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

// RunWithBootstrappedFTTransferIndex creates a new Pebble database and bootstraps it
// for fungible token transfer indexing at the given start height. The callback receives a shared
// lock manager that should be passed to storeFTTransfers for consistent lock usage.
func RunWithBootstrappedFTTransferIndex(tb testing.TB, startHeight uint64, transfers []access.FungibleTokenTransfer, f func(db storage.DB, lockManager storage.LockManager, idx *FungibleTokenTransfers)) {
	unittest.RunWithPebbleDB(tb, func(db *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		var idx *FungibleTokenTransfers
		storageDB := pebbleimpl.ToDB(db)
		err := unittest.WithLock(tb, lockManager, storage.LockIndexFungibleTokenTransfers, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				var bootstrapErr error
				idx, bootstrapErr = BootstrapFungibleTokenTransfers(lctx, rw, storageDB, startHeight, transfers)
				return bootstrapErr
			})
		})
		require.NoError(tb, err)
		f(storageDB, lockManager, idx)
	})
}

// storeFTTransfers stores fungible token transfers at the given height using the provided index
// and lock manager.
func storeFTTransfers(tb testing.TB, lockManager storage.LockManager, idx *FungibleTokenTransfers, height uint64, transfers []access.FungibleTokenTransfer) error {
	return unittest.WithLock(tb, lockManager, storage.LockIndexFungibleTokenTransfers, func(lctx lockctx.Context) error {
		return idx.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return idx.Store(lctx, rw, height, transfers)
		})
	})
}

// makeTestTransfer is a helper to build an access.FungibleTokenTransfer for testing.
func makeTestTransfer(
	source flow.Address,
	recipient flow.Address,
	blockHeight uint64,
	txIndex uint32,
	eventIndex uint32,
	tokenType string,
	amount *big.Int,
) access.FungibleTokenTransfer {
	return access.FungibleTokenTransfer{
		TransactionID:    unittest.IdentifierFixture(),
		BlockHeight:      blockHeight,
		TransactionIndex: txIndex,
		EventIndices:     []uint32{eventIndex},
		SourceAddress:    source,
		RecipientAddress: recipient,
		TokenType:        tokenType,
		Amount:           amount,
	}
}

func TestFTTransfers_Initialize(t *testing.T) {
	t.Parallel()

	t.Run("uninitialized database returns ErrNotBootstrapped", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			_, err := NewFungibleTokenTransfers(storageDB)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})
	})

	t.Run("corrupted DB with firstHeight but no latestHeight returns exception", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)

			// Write only the firstHeight key, simulating a corrupted state
			err := storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertByKey(rw.Writer(), keyAccountFTTransferFirstHeightKey, uint64(10))
			})
			require.NoError(t, err)

			_, err = NewFungibleTokenTransfers(storageDB)
			require.Error(t, err)
			assert.False(t, errors.Is(err, storage.ErrNotBootstrapped),
				"should not return ErrNotBootstrapped for corrupted state")
		})
	})

	t.Run("bootstrap initializes the index", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, _ storage.LockManager, idx *FungibleTokenTransfers) {
			assert.Equal(t, uint64(1), idx.FirstIndexedHeight())
			assert.Equal(t, uint64(1), idx.LatestIndexedHeight())
		})
	})

	t.Run("bootstrap with initial data stores and retrieves transfers", func(t *testing.T) {
		t.Parallel()
		source := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()
		amount := big.NewInt(42)

		initialData := []access.FungibleTokenTransfer{
			{
				TransactionID:    unittest.IdentifierFixture(),
				BlockHeight:      5,
				TransactionIndex: 0,
				EventIndices:     []uint32{0},
				SourceAddress:    source,
				RecipientAddress: recipient,
				TokenType:        "A.0x1654653399040a61.FlowToken",
				Amount:           amount,
			},
		}

		RunWithBootstrappedFTTransferIndex(t, 5, initialData, func(_ storage.DB, _ storage.LockManager, idx *FungibleTokenTransfers) {
			assert.Equal(t, uint64(5), idx.FirstIndexedHeight())
			assert.Equal(t, uint64(5), idx.LatestIndexedHeight())

			// Query by source
			page, err := idx.TransfersByAddress(source, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 1)
			assert.Equal(t, initialData[0].TransactionID, page.Transfers[0].TransactionID)
			assert.Equal(t, initialData[0].BlockHeight, page.Transfers[0].BlockHeight)
			assert.Equal(t, initialData[0].TransactionIndex, page.Transfers[0].TransactionIndex)
			assert.Equal(t, initialData[0].EventIndices[0], page.Transfers[0].EventIndices[0])
			assert.Equal(t, source, page.Transfers[0].SourceAddress)
			assert.Equal(t, recipient, page.Transfers[0].RecipientAddress)
			assert.Equal(t, initialData[0].TokenType, page.Transfers[0].TokenType)
			assert.Equal(t, 0, amount.Cmp(page.Transfers[0].Amount))

			// Query by recipient
			page, err = idx.TransfersByAddress(recipient, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 1)
			assert.Equal(t, initialData[0].TransactionID, page.Transfers[0].TransactionID)
		})
	})

	t.Run("bootstrap at height 0", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 0, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			assert.Equal(t, uint64(0), idx.FirstIndexedHeight())
			assert.Equal(t, uint64(0), idx.LatestIndexedHeight())

			// Store at height 1 (consecutive after 0)
			source := unittest.RandomAddressFixture()
			recipient := unittest.RandomAddressFixture()
			transfer := makeTestTransfer(source, recipient, 1, 0, 0, "A.FlowToken", big.NewInt(100))

			err := storeFTTransfers(t, lm, idx, 1, []access.FungibleTokenTransfer{transfer})
			require.NoError(t, err)
			assert.Equal(t, uint64(1), idx.LatestIndexedHeight())

			page, err := idx.TransfersByAddress(source, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 1)
			assert.Equal(t, transfer.TransactionID, page.Transfers[0].TransactionID)
		})
	})
}

func TestFTTransfers_StoreAndQuery(t *testing.T) {
	t.Parallel()

	t.Run("store single block with single transfer", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			source := unittest.RandomAddressFixture()
			recipient := unittest.RandomAddressFixture()
			amount := big.NewInt(500)
			transfer := access.FungibleTokenTransfer{
				TransactionID:    unittest.IdentifierFixture(),
				BlockHeight:      2,
				TransactionIndex: 0,
				EventIndices:     []uint32{0},
				SourceAddress:    source,
				RecipientAddress: recipient,
				TokenType:        "A.FlowToken",
				Amount:           amount,
			}

			err := storeFTTransfers(t, lm, idx, 2, []access.FungibleTokenTransfer{transfer})
			require.NoError(t, err)

			page, err := idx.TransfersByAddress(source, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 1)
			assert.Equal(t, transfer.TransactionID, page.Transfers[0].TransactionID)
			assert.Equal(t, uint64(2), page.Transfers[0].BlockHeight)
			assert.Equal(t, uint32(0), page.Transfers[0].TransactionIndex)
			assert.Equal(t, uint32(0), page.Transfers[0].EventIndices[0])
			assert.Equal(t, source, page.Transfers[0].SourceAddress)
			assert.Equal(t, recipient, page.Transfers[0].RecipientAddress)
			assert.Equal(t, "A.FlowToken", page.Transfers[0].TokenType)
			assert.Equal(t, 0, amount.Cmp(page.Transfers[0].Amount))
		})
	})

	t.Run("store single block with multiple transfers for different accounts", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			alice := unittest.RandomAddressFixture()
			bob := unittest.RandomAddressFixture()
			charlie := unittest.RandomAddressFixture()

			transfers := []access.FungibleTokenTransfer{
				makeTestTransfer(alice, bob, 2, 0, 0, "A.FlowToken", big.NewInt(100)),
				makeTestTransfer(bob, charlie, 2, 1, 0, "A.FlowToken", big.NewInt(50)),
			}

			err := storeFTTransfers(t, lm, idx, 2, transfers)
			require.NoError(t, err)

			// Alice: 1 transfer (as source)
			page, err := idx.TransfersByAddress(alice, 100, nil, nil)
			require.NoError(t, err)
			assert.Len(t, page.Transfers, 1)

			// Bob: 2 transfers (as recipient of first, source of second)
			page, err = idx.TransfersByAddress(bob, 100, nil, nil)
			require.NoError(t, err)
			assert.Len(t, page.Transfers, 2)

			// Charlie: 1 transfer (as recipient)
			page, err = idx.TransfersByAddress(charlie, 100, nil, nil)
			require.NoError(t, err)
			assert.Len(t, page.Transfers, 1)
		})
	})

	t.Run("store multiple blocks with transfers, query by address", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			alice := unittest.RandomAddressFixture()
			bob := unittest.RandomAddressFixture()

			// Block 2: Alice sends to Bob
			transfer1 := makeTestTransfer(alice, bob, 2, 0, 0, "A.FlowToken", big.NewInt(100))
			err := storeFTTransfers(t, lm, idx, 2, []access.FungibleTokenTransfer{transfer1})
			require.NoError(t, err)

			// Block 3: Bob sends to Alice
			transfer2 := makeTestTransfer(bob, alice, 3, 0, 0, "A.FlowToken", big.NewInt(50))
			err = storeFTTransfers(t, lm, idx, 3, []access.FungibleTokenTransfer{transfer2})
			require.NoError(t, err)

			// Alice should see both transfers (source in block 2, recipient in block 3)
			page, err := idx.TransfersByAddress(alice, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 2)

			// Bob should see both transfers (recipient in block 2, source in block 3)
			page, err = idx.TransfersByAddress(bob, 100, nil, nil)
			require.NoError(t, err)
			assert.Len(t, page.Transfers, 2)
		})
	})

	t.Run("dual indexing: transfer indexed under both source and recipient", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			source := unittest.RandomAddressFixture()
			recipient := unittest.RandomAddressFixture()
			txID := unittest.IdentifierFixture()

			transfer := access.FungibleTokenTransfer{
				TransactionID:    txID,
				BlockHeight:      2,
				TransactionIndex: 0,
				EventIndices:     []uint32{0},
				SourceAddress:    source,
				RecipientAddress: recipient,
				TokenType:        "A.FlowToken",
				Amount:           big.NewInt(999),
			}

			err := storeFTTransfers(t, lm, idx, 2, []access.FungibleTokenTransfer{transfer})
			require.NoError(t, err)

			// Query by source address
			sourcePage, err := idx.TransfersByAddress(source, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, sourcePage.Transfers, 1)
			assert.Equal(t, txID, sourcePage.Transfers[0].TransactionID)

			// Query by recipient address
			recipientPage, err := idx.TransfersByAddress(recipient, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, recipientPage.Transfers, 1)
			assert.Equal(t, txID, recipientPage.Transfers[0].TransactionID)

			// Both should contain the same transfer data
			assert.Equal(t, sourcePage.Transfers[0].SourceAddress, recipientPage.Transfers[0].SourceAddress)
			assert.Equal(t, sourcePage.Transfers[0].RecipientAddress, recipientPage.Transfers[0].RecipientAddress)
			assert.Equal(t, sourcePage.Transfers[0].TokenType, recipientPage.Transfers[0].TokenType)
		})
	})

	t.Run("query returns results in descending order (newest first)", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			account := unittest.RandomAddressFixture()
			other := unittest.RandomAddressFixture()

			// Index 10 blocks, each with a transfer involving account
			for height := uint64(2); height <= 11; height++ {
				transfer := makeTestTransfer(account, other, height, 0, 0, "A.FlowToken", big.NewInt(int64(height)))
				err := storeFTTransfers(t, lm, idx, height, []access.FungibleTokenTransfer{transfer})
				require.NoError(t, err)
			}

			page, err := idx.TransfersByAddress(account, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 10)

			// Verify descending order by height
			for i := 0; i < len(page.Transfers)-1; i++ {
				assert.Greater(t, page.Transfers[i].BlockHeight, page.Transfers[i+1].BlockHeight,
					"results should be in descending order by height")
			}
		})
	})

	t.Run("multiple transfers at same height ordered by txIndex then eventIndex", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			account := unittest.RandomAddressFixture()
			other := unittest.RandomAddressFixture()

			transfers := []access.FungibleTokenTransfer{
				makeTestTransfer(account, other, 2, 0, 0, "A.FlowToken", big.NewInt(10)),
				makeTestTransfer(account, other, 2, 0, 1, "A.FlowToken", big.NewInt(20)),
				makeTestTransfer(account, other, 2, 1, 0, "A.FlowToken", big.NewInt(30)),
				makeTestTransfer(account, other, 2, 1, 2, "A.FlowToken", big.NewInt(40)),
				makeTestTransfer(account, other, 2, 2, 0, "A.FlowToken", big.NewInt(50)),
			}

			err := storeFTTransfers(t, lm, idx, 2, transfers)
			require.NoError(t, err)

			page, err := idx.TransfersByAddress(account, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 5)

			// Within same height, should be ordered by txIndex ascending, then eventIndex ascending
			assert.Equal(t, uint32(0), page.Transfers[0].TransactionIndex)
			assert.Equal(t, uint32(0), page.Transfers[0].EventIndices[0])

			assert.Equal(t, uint32(0), page.Transfers[1].TransactionIndex)
			assert.Equal(t, uint32(1), page.Transfers[1].EventIndices[0])

			assert.Equal(t, uint32(1), page.Transfers[2].TransactionIndex)
			assert.Equal(t, uint32(0), page.Transfers[2].EventIndices[0])

			assert.Equal(t, uint32(1), page.Transfers[3].TransactionIndex)
			assert.Equal(t, uint32(2), page.Transfers[3].EventIndices[0])

			assert.Equal(t, uint32(2), page.Transfers[4].TransactionIndex)
			assert.Equal(t, uint32(0), page.Transfers[4].EventIndices[0])
		})
	})
}

func TestFTTransfers_HeightValidation(t *testing.T) {
	t.Parallel()

	t.Run("store at latestHeight is a no-op", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			source := unittest.RandomAddressFixture()
			recipient := unittest.RandomAddressFixture()

			// Index height 2
			transfer := makeTestTransfer(source, recipient, 2, 0, 0, "A.FlowToken", big.NewInt(100))
			err := storeFTTransfers(t, lm, idx, 2, []access.FungibleTokenTransfer{transfer})
			require.NoError(t, err)

			// Re-store at height 2 with different data (should be a no-op)
			differentTransfer := makeTestTransfer(source, recipient, 2, 0, 0, "A.DifferentToken", big.NewInt(999))
			err = storeFTTransfers(t, lm, idx, 2, []access.FungibleTokenTransfer{differentTransfer})
			require.NoError(t, err)

			// Verify original data is retained
			page, err := idx.TransfersByAddress(source, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 1)
			assert.Equal(t, transfer.TransactionID, page.Transfers[0].TransactionID)
			assert.Equal(t, "A.FlowToken", page.Transfers[0].TokenType)
		})
	})

	t.Run("store below latestHeight returns ErrAlreadyExists", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			err := storeFTTransfers(t, lm, idx, 2, nil)
			require.NoError(t, err)
			err = storeFTTransfers(t, lm, idx, 3, nil)
			require.NoError(t, err)

			// Try to store height 1 (below latest=3)
			err = storeFTTransfers(t, lm, idx, 1, nil)
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})

	t.Run("store non-consecutive height fails", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			// Try to index height 5 when latest is 1
			err := storeFTTransfers(t, lm, idx, 5, nil)
			require.Error(t, err)
			assert.False(t, errors.Is(err, storage.ErrAlreadyExists),
				"non-consecutive height should not return ErrAlreadyExists")
		})
	})

	t.Run("block height mismatch in entry fails", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			source := unittest.RandomAddressFixture()
			recipient := unittest.RandomAddressFixture()

			// Entry claims height 5 but we're indexing height 2
			badTransfer := access.FungibleTokenTransfer{
				TransactionID:    unittest.IdentifierFixture(),
				BlockHeight:      5, // mismatch
				TransactionIndex: 0,
				EventIndices:     []uint32{0},
				SourceAddress:    source,
				RecipientAddress: recipient,
				TokenType:        "A.FlowToken",
				Amount:           big.NewInt(100),
			}

			err := storeFTTransfers(t, lm, idx, 2, []access.FungibleTokenTransfer{badTransfer})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "block height mismatch")
		})
	})
}

func TestFTTransfers_RangeQueries(t *testing.T) {
	t.Parallel()

	t.Run("cursor height greater than latestIndexedHeight returns ErrHeightNotIndexed", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 5, nil, func(_ storage.DB, _ storage.LockManager, idx *FungibleTokenTransfers) {
			account := unittest.RandomAddressFixture()
			cursor := &access.TransferCursor{BlockHeight: 100}
			_, err := idx.TransfersByAddress(account, 10, cursor, nil)
			require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
		})
	})

	t.Run("cursor height less than firstIndexedHeight returns ErrHeightNotIndexed", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 5, nil, func(_ storage.DB, _ storage.LockManager, idx *FungibleTokenTransfers) {
			account := unittest.RandomAddressFixture()
			cursor := &access.TransferCursor{BlockHeight: 1}
			_, err := idx.TransfersByAddress(account, 10, cursor, nil)
			require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
		})
	})

	t.Run("nil cursor queries all data from latest height", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			source := unittest.RandomAddressFixture()
			recipient := unittest.RandomAddressFixture()

			// Index heights 2 and 3
			transfer2 := makeTestTransfer(source, recipient, 2, 0, 0, "A.FlowToken", big.NewInt(100))
			err := storeFTTransfers(t, lm, idx, 2, []access.FungibleTokenTransfer{transfer2})
			require.NoError(t, err)
			transfer3 := makeTestTransfer(source, recipient, 3, 0, 0, "A.FlowToken", big.NewInt(200))
			err = storeFTTransfers(t, lm, idx, 3, []access.FungibleTokenTransfer{transfer3})
			require.NoError(t, err)

			// nil cursor returns all data
			page, err := idx.TransfersByAddress(source, 100, nil, nil)
			require.NoError(t, err)
			assert.Len(t, page.Transfers, 2)
		})
	})

	t.Run("limit must be greater than 0", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 5, nil, func(_ storage.DB, _ storage.LockManager, idx *FungibleTokenTransfers) {
			account := unittest.RandomAddressFixture()
			_, err := idx.TransfersByAddress(account, 0, nil, nil)
			require.Error(t, err)
		})
	})

	t.Run("empty results for address with no transfers", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			// Index a block so we have indexed heights
			err := storeFTTransfers(t, lm, idx, 2, nil)
			require.NoError(t, err)

			noTransfersAccount := unittest.RandomAddressFixture()
			page, err := idx.TransfersByAddress(noTransfersAccount, 100, nil, nil)
			require.NoError(t, err)
			assert.Empty(t, page.Transfers)
		})
	})
}

func TestFTTransfers_KeyEncoding(t *testing.T) {
	t.Parallel()

	t.Run("roundtrip: encode then decode returns original values", func(t *testing.T) {
		t.Parallel()
		address := unittest.RandomAddressFixture()
		height := uint64(12345)
		txIndex := uint32(42)
		eventIndex := uint32(7)

		key := makeFTTransferKey(address, height, txIndex, eventIndex)

		decodedAddress, decodedHeight, decodedTxIndex, decodedEventIndex, err := decodeFTTransferKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, decodedAddress)
		assert.Equal(t, height, decodedHeight)
		assert.Equal(t, txIndex, decodedTxIndex)
		assert.Equal(t, eventIndex, decodedEventIndex)
	})

	t.Run("boundary values: height 0, txIndex 0, eventIndex 0", func(t *testing.T) {
		t.Parallel()
		address := unittest.RandomAddressFixture()

		key := makeFTTransferKey(address, 0, 0, 0)
		decodedAddress, decodedHeight, decodedTxIndex, decodedEventIndex, err := decodeFTTransferKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, decodedAddress)
		assert.Equal(t, uint64(0), decodedHeight)
		assert.Equal(t, uint32(0), decodedTxIndex)
		assert.Equal(t, uint32(0), decodedEventIndex)
	})

	t.Run("boundary values: max height, max txIndex, max eventIndex", func(t *testing.T) {
		t.Parallel()
		address := unittest.RandomAddressFixture()

		key := makeFTTransferKey(address, math.MaxUint64, math.MaxUint32, math.MaxUint32)
		decodedAddress, decodedHeight, decodedTxIndex, decodedEventIndex, err := decodeFTTransferKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, decodedAddress)
		assert.Equal(t, uint64(math.MaxUint64), decodedHeight)
		assert.Equal(t, uint32(math.MaxUint32), decodedTxIndex)
		assert.Equal(t, uint32(math.MaxUint32), decodedEventIndex)
	})

	t.Run("boundary values: zero address", func(t *testing.T) {
		t.Parallel()
		address := flow.Address{}
		height := uint64(12345)
		txIndex := uint32(42)
		eventIndex := uint32(7)

		key := makeFTTransferKey(address, height, txIndex, eventIndex)
		decodedAddress, decodedHeight, decodedTxIndex, decodedEventIndex, err := decodeFTTransferKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, decodedAddress)
		assert.Equal(t, height, decodedHeight)
		assert.Equal(t, txIndex, decodedTxIndex)
		assert.Equal(t, eventIndex, decodedEventIndex)
	})

	t.Run("ones complement ensures descending order", func(t *testing.T) {
		t.Parallel()
		address := unittest.RandomAddressFixture()

		// Higher heights should produce lexicographically smaller keys (descending order)
		keyLow := makeFTTransferKey(address, 100, 0, 0)
		keyHigh := makeFTTransferKey(address, 200, 0, 0)

		// In byte comparison, the key for height 200 should sort before height 100
		// because ^200 < ^100 (ones complement inverts the order)
		assert.True(t, string(keyHigh) < string(keyLow),
			"key for higher height should sort lexicographically before key for lower height")
	})
}

func TestFTTransfers_KeyDecoding_Errors(t *testing.T) {
	t.Parallel()

	t.Run("key too short", func(t *testing.T) {
		t.Parallel()
		_, _, _, _, err := decodeFTTransferKey(make([]byte, 10))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid key length")
	})

	t.Run("key too long", func(t *testing.T) {
		t.Parallel()
		_, _, _, _, err := decodeFTTransferKey(make([]byte, 30))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid key length")
	})

	t.Run("invalid prefix", func(t *testing.T) {
		t.Parallel()
		key := make([]byte, ftTransferKeyLen)
		key[0] = 0xFF // wrong prefix
		_, _, _, _, err := decodeFTTransferKey(key)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid prefix")
	})
}

func TestFTTransfers_LockRequirement(t *testing.T) {
	t.Parallel()

	t.Run("indexFTTransfers without lock returns error", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(db storage.DB, lm storage.LockManager, _ *FungibleTokenTransfers) {
			lctx := lm.NewContext()
			defer lctx.Release()

			// Call without acquiring the required lock
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return indexFTTransfers(lctx, rw, 2, nil)
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing required lock")
		})
	})

	t.Run("initializeFTTransfers without lock returns error", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			lctx := lm.NewContext()
			defer lctx.Release()

			// Call without acquiring the required lock
			err := storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapFungibleTokenTransfers(lctx, rw, storageDB, 1, nil)
				return bootstrapErr
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing required lock")
		})
	})
}

func TestFTTransfers_UncommittedBatch(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedFTTransferIndex(t, 1, nil, func(db storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
		require.Equal(t, uint64(1), idx.LatestIndexedHeight())

		source := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()
		transfer := access.FungibleTokenTransfer{
			TransactionID:    unittest.IdentifierFixture(),
			BlockHeight:      2,
			TransactionIndex: 0,
			EventIndices:     []uint32{0},
			SourceAddress:    source,
			RecipientAddress: recipient,
			TokenType:        "A.FlowToken",
			Amount:           big.NewInt(100),
		}

		// Create a batch manually and store data without committing.
		// Store registers an OnCommitSucceed callback to update latestHeight,
		// which should only fire when the batch is committed.
		batch := db.NewBatch()
		err := unittest.WithLock(t, lm, storage.LockIndexFungibleTokenTransfers, func(lctx lockctx.Context) error {
			return idx.Store(lctx, batch, 2, []access.FungibleTokenTransfer{transfer})
		})
		require.NoError(t, err)

		// Close the batch without committing - discards pending writes
		require.NoError(t, batch.Close())

		// latestHeight must still be 1 since the batch was never committed
		assert.Equal(t, uint64(1), idx.LatestIndexedHeight(),
			"latestHeight should not update when the batch is not committed")
	})
}

func TestFTTransfers_BootstrapHeightMismatch(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		lm := storage.NewTestingLockManager()

		source := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()

		// Entry claims height 99 but we're bootstrapping at height 5
		err := unittest.WithLock(t, lm, storage.LockIndexFungibleTokenTransfers, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapFungibleTokenTransfers(lctx, rw, storageDB, 5, []access.FungibleTokenTransfer{
					{
						TransactionID:    unittest.IdentifierFixture(),
						BlockHeight:      99, // mismatch with bootstrap height 5
						TransactionIndex: 0,
						EventIndices:     []uint32{0},
						SourceAddress:    source,
						RecipientAddress: recipient,
						TokenType:        "A.FlowToken",
						Amount:           big.NewInt(100),
					},
				})
				return bootstrapErr
			})
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "block height mismatch")
	})
}

func TestFTTransfers_SelfTransfer(t *testing.T) {
	t.Parallel()

	// When source == recipient, the two UpsertByKey calls write the same key.
	// The second is an idempotent overwrite. The address should still see exactly one entry.
	RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
		account := unittest.RandomAddressFixture()
		transfer := access.FungibleTokenTransfer{
			TransactionID:    unittest.IdentifierFixture(),
			BlockHeight:      2,
			TransactionIndex: 0,
			EventIndices:     []uint32{0},
			SourceAddress:    account,
			RecipientAddress: account, // same as source
			TokenType:        "A.FlowToken",
			Amount:           big.NewInt(42),
		}

		err := storeFTTransfers(t, lm, idx, 2, []access.FungibleTokenTransfer{transfer})
		require.NoError(t, err)

		page, err := idx.TransfersByAddress(account, 100, nil, nil)
		require.NoError(t, err)
		assert.Len(t, page.Transfers, 1, "self-transfer should produce exactly one entry per address")
		assert.Equal(t, transfer.TransactionID, page.Transfers[0].TransactionID)
	})
}

func TestFTTransfers_LargeAmount(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
		source := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()

		// Use a very large amount (bigger than uint64)
		largeAmount := new(big.Int)
		largeAmount.SetString("999999999999999999999999999999999999999999", 10)

		transfer := access.FungibleTokenTransfer{
			TransactionID:    unittest.IdentifierFixture(),
			BlockHeight:      2,
			TransactionIndex: 0,
			EventIndices:     []uint32{0},
			SourceAddress:    source,
			RecipientAddress: recipient,
			TokenType:        "A.FlowToken",
			Amount:           largeAmount,
		}

		err := storeFTTransfers(t, lm, idx, 2, []access.FungibleTokenTransfer{transfer})
		require.NoError(t, err)

		page, err := idx.TransfersByAddress(source, 100, nil, nil)
		require.NoError(t, err)
		require.Len(t, page.Transfers, 1)
		assert.Equal(t, 0, largeAmount.Cmp(page.Transfers[0].Amount),
			"large amount should roundtrip correctly")
	})
}

func TestFTTransfers_NilAmount(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
		source := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()

		transfer := access.FungibleTokenTransfer{
			TransactionID:    unittest.IdentifierFixture(),
			BlockHeight:      2,
			TransactionIndex: 0,
			EventIndices:     []uint32{0},
			SourceAddress:    source,
			RecipientAddress: recipient,
			TokenType:        "A.FlowToken",
			Amount:           nil, // nil amount
		}

		err := storeFTTransfers(t, lm, idx, 2, []access.FungibleTokenTransfer{transfer})
		require.NoError(t, err)

		page, err := idx.TransfersByAddress(source, 100, nil, nil)
		require.NoError(t, err)
		require.Len(t, page.Transfers, 1)
		// nil amount stored as empty bytes, then SetBytes on empty produces 0
		assert.Equal(t, 0, page.Transfers[0].Amount.Cmp(big.NewInt(0)),
			"nil amount should roundtrip as zero")
	})
}
