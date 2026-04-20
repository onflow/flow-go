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
	"github.com/onflow/flow-go/storage/indexes/iterator"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

// allFTTransfers is a test helper that queries all transfers for the given account using
// ByAddress with no cursor, and collects all results.
func allFTTransfers(tb testing.TB, idx storage.FungibleTokenTransfersReader, account flow.Address) []access.FungibleTokenTransfer {
	tb.Helper()
	iter, err := idx.ByAddress(account, nil)
	require.NoError(tb, err)
	var transfers []access.FungibleTokenTransfer
	for item := range iter {
		t, err := item.Value()
		require.NoError(tb, err)
		transfers = append(transfers, t)
	}
	return transfers
}

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
			transfers := allFTTransfers(t, idx, source)
			require.Len(t, transfers, 1)
			assert.Equal(t, initialData[0].TransactionID, transfers[0].TransactionID)
			assert.Equal(t, initialData[0].BlockHeight, transfers[0].BlockHeight)
			assert.Equal(t, initialData[0].TransactionIndex, transfers[0].TransactionIndex)
			assert.Equal(t, initialData[0].EventIndices[0], transfers[0].EventIndices[0])
			assert.Equal(t, source, transfers[0].SourceAddress)
			assert.Equal(t, recipient, transfers[0].RecipientAddress)
			assert.Equal(t, initialData[0].TokenType, transfers[0].TokenType)
			assert.Equal(t, 0, amount.Cmp(transfers[0].Amount))

			// Query by recipient
			recipientTransfers := allFTTransfers(t, idx, recipient)
			require.Len(t, recipientTransfers, 1)
			assert.Equal(t, initialData[0].TransactionID, recipientTransfers[0].TransactionID)
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

			transfers := allFTTransfers(t, idx, source)
			require.Len(t, transfers, 1)
			assert.Equal(t, transfer.TransactionID, transfers[0].TransactionID)
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

			transfers := allFTTransfers(t, idx, source)
			require.Len(t, transfers, 1)
			assert.Equal(t, transfer.TransactionID, transfers[0].TransactionID)
			assert.Equal(t, uint64(2), transfers[0].BlockHeight)
			assert.Equal(t, uint32(0), transfers[0].TransactionIndex)
			assert.Equal(t, uint32(0), transfers[0].EventIndices[0])
			assert.Equal(t, source, transfers[0].SourceAddress)
			assert.Equal(t, recipient, transfers[0].RecipientAddress)
			assert.Equal(t, "A.FlowToken", transfers[0].TokenType)
			assert.Equal(t, 0, amount.Cmp(transfers[0].Amount))
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
			aliceTransfers := allFTTransfers(t, idx, alice)
			assert.Len(t, aliceTransfers, 1)

			// Bob: 2 transfers (as recipient of first, source of second)
			bobTransfers := allFTTransfers(t, idx, bob)
			assert.Len(t, bobTransfers, 2)

			// Charlie: 1 transfer (as recipient)
			charlieTransfers := allFTTransfers(t, idx, charlie)
			assert.Len(t, charlieTransfers, 1)
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
			aliceTransfers := allFTTransfers(t, idx, alice)
			require.Len(t, aliceTransfers, 2)

			// Bob should see both transfers (recipient in block 2, source in block 3)
			bobTransfers := allFTTransfers(t, idx, bob)
			assert.Len(t, bobTransfers, 2)
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
			sourceTransfers := allFTTransfers(t, idx, source)
			require.Len(t, sourceTransfers, 1)
			assert.Equal(t, txID, sourceTransfers[0].TransactionID)

			// Query by recipient address
			recipientTransfers := allFTTransfers(t, idx, recipient)
			require.Len(t, recipientTransfers, 1)
			assert.Equal(t, txID, recipientTransfers[0].TransactionID)

			// Both should contain the same transfer data
			assert.Equal(t, sourceTransfers[0].SourceAddress, recipientTransfers[0].SourceAddress)
			assert.Equal(t, sourceTransfers[0].RecipientAddress, recipientTransfers[0].RecipientAddress)
			assert.Equal(t, sourceTransfers[0].TokenType, recipientTransfers[0].TokenType)
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

			transfers := allFTTransfers(t, idx, account)
			require.Len(t, transfers, 10)

			// Verify descending order by height
			for i := 0; i < len(transfers)-1; i++ {
				assert.Greater(t, transfers[i].BlockHeight, transfers[i+1].BlockHeight,
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

			results := allFTTransfers(t, idx, account)
			require.Len(t, results, 5)

			// Within same height, should be ordered by txIndex ascending, then eventIndex ascending
			assert.Equal(t, uint32(0), results[0].TransactionIndex)
			assert.Equal(t, uint32(0), results[0].EventIndices[0])

			assert.Equal(t, uint32(0), results[1].TransactionIndex)
			assert.Equal(t, uint32(1), results[1].EventIndices[0])

			assert.Equal(t, uint32(1), results[2].TransactionIndex)
			assert.Equal(t, uint32(0), results[2].EventIndices[0])

			assert.Equal(t, uint32(1), results[3].TransactionIndex)
			assert.Equal(t, uint32(2), results[3].EventIndices[0])

			assert.Equal(t, uint32(2), results[4].TransactionIndex)
			assert.Equal(t, uint32(0), results[4].EventIndices[0])
		})
	})
}

func TestFTTransfers_HeightValidation(t *testing.T) {
	t.Parallel()

	t.Run("store at latestHeight returns ErrAlreadyExists", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			// Index height 2
			err := storeFTTransfers(t, lm, idx, 2, nil)
			require.NoError(t, err)

			// Re-store at height 2 should return ErrAlreadyExists
			err = storeFTTransfers(t, lm, idx, 2, nil)
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
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

	t.Run("store with nil EventIndices fails", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			transfer := access.FungibleTokenTransfer{
				TransactionID:    unittest.IdentifierFixture(),
				BlockHeight:      2,
				TransactionIndex: 0,
				EventIndices:     nil, // invalid: must have at least one event index
				SourceAddress:    unittest.RandomAddressFixture(),
				RecipientAddress: unittest.RandomAddressFixture(),
				TokenType:        "A.FlowToken",
				Amount:           big.NewInt(100),
			}

			err := storeFTTransfers(t, lm, idx, 2, []access.FungibleTokenTransfer{transfer})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "at least one event index")
		})
	})

	t.Run("store with empty EventIndices fails", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			transfer := access.FungibleTokenTransfer{
				TransactionID:    unittest.IdentifierFixture(),
				BlockHeight:      2,
				TransactionIndex: 0,
				EventIndices:     []uint32{}, // invalid: must have at least one event index
				SourceAddress:    unittest.RandomAddressFixture(),
				RecipientAddress: unittest.RandomAddressFixture(),
				TokenType:        "A.FlowToken",
				Amount:           big.NewInt(100),
			}

			err := storeFTTransfers(t, lm, idx, 2, []access.FungibleTokenTransfer{transfer})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "at least one event index")
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
			_, err := idx.ByAddress(account, cursor)
			require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
		})
	})

	t.Run("cursor height less than firstIndexedHeight returns ErrHeightNotIndexed", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 5, nil, func(_ storage.DB, _ storage.LockManager, idx *FungibleTokenTransfers) {
			account := unittest.RandomAddressFixture()
			cursor := &access.TransferCursor{BlockHeight: 1}
			_, err := idx.ByAddress(account, cursor)
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
			transfers := allFTTransfers(t, idx, source)
			assert.Len(t, transfers, 2)
		})
	})

	t.Run("empty results for address with no transfers", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			// Index a block so we have indexed heights
			err := storeFTTransfers(t, lm, idx, 2, nil)
			require.NoError(t, err)

			noTransfersAccount := unittest.RandomAddressFixture()
			transfers := allFTTransfers(t, idx, noTransfersAccount)
			assert.Empty(t, transfers)
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

		cursor, err := decodeFTTransferKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, cursor.Address)
		assert.Equal(t, height, cursor.BlockHeight)
		assert.Equal(t, txIndex, cursor.TransactionIndex)
		assert.Equal(t, eventIndex, cursor.EventIndex)
	})

	t.Run("boundary values: height 0, txIndex 0, eventIndex 0", func(t *testing.T) {
		t.Parallel()
		address := unittest.RandomAddressFixture()

		key := makeFTTransferKey(address, 0, 0, 0)
		cursor, err := decodeFTTransferKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, cursor.Address)
		assert.Equal(t, uint64(0), cursor.BlockHeight)
		assert.Equal(t, uint32(0), cursor.TransactionIndex)
		assert.Equal(t, uint32(0), cursor.EventIndex)
	})

	t.Run("boundary values: max height, max txIndex, max eventIndex", func(t *testing.T) {
		t.Parallel()
		address := unittest.RandomAddressFixture()

		key := makeFTTransferKey(address, math.MaxUint64, math.MaxUint32, math.MaxUint32)
		cursor, err := decodeFTTransferKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, cursor.Address)
		assert.Equal(t, uint64(math.MaxUint64), cursor.BlockHeight)
		assert.Equal(t, uint32(math.MaxUint32), cursor.TransactionIndex)
		assert.Equal(t, uint32(math.MaxUint32), cursor.EventIndex)
	})

	t.Run("boundary values: zero address", func(t *testing.T) {
		t.Parallel()
		address := flow.Address{}
		height := uint64(12345)
		txIndex := uint32(42)
		eventIndex := uint32(7)

		key := makeFTTransferKey(address, height, txIndex, eventIndex)
		cursor, err := decodeFTTransferKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, cursor.Address)
		assert.Equal(t, height, cursor.BlockHeight)
		assert.Equal(t, txIndex, cursor.TransactionIndex)
		assert.Equal(t, eventIndex, cursor.EventIndex)
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
		_, err := decodeFTTransferKey(make([]byte, 10))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid key length")
	})

	t.Run("key too long", func(t *testing.T) {
		t.Parallel()
		_, err := decodeFTTransferKey(make([]byte, 30))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid key length")
	})

	t.Run("invalid prefix", func(t *testing.T) {
		t.Parallel()
		key := make([]byte, ftTransferKeyLen)
		key[0] = 0xFF // wrong prefix
		_, err := decodeFTTransferKey(key)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid prefix")
	})
}

func TestFTTransfers_LockRequirement(t *testing.T) {
	t.Parallel()

	t.Run("Store without lock returns error", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedFTTransferIndex(t, 1, nil, func(db storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
			lctx := lm.NewContext()
			defer lctx.Release()

			// Call without acquiring the required lock
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return idx.Store(lctx, rw, 2, nil)
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

func TestFTTransfers_BootstrapEmptyEventIndices(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		lm := storage.NewTestingLockManager()

		err := unittest.WithLock(t, lm, storage.LockIndexFungibleTokenTransfers, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapFungibleTokenTransfers(lctx, rw, storageDB, 5, []access.FungibleTokenTransfer{
					{
						TransactionID:    unittest.IdentifierFixture(),
						BlockHeight:      5,
						TransactionIndex: 0,
						EventIndices:     nil, // invalid: must have at least one event index
						SourceAddress:    unittest.RandomAddressFixture(),
						RecipientAddress: unittest.RandomAddressFixture(),
						TokenType:        "A.FlowToken",
						Amount:           big.NewInt(100),
					},
				})
				return bootstrapErr
			})
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one event index")
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

		transfers := allFTTransfers(t, idx, account)
		assert.Len(t, transfers, 1, "self-transfer should produce exactly one entry per address")
		assert.Equal(t, transfer.TransactionID, transfers[0].TransactionID)
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

		transfers := allFTTransfers(t, idx, source)
		require.Len(t, transfers, 1)
		assert.Equal(t, 0, largeAmount.Cmp(transfers[0].Amount),
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
			Amount:           nil,
		}

		err := storeFTTransfers(t, lm, idx, 2, []access.FungibleTokenTransfer{transfer})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "transfer amount is nil")
	})
}

// TestFTTransfers_PaginationCoversAllEntries verifies that paginating through all transfers
// for an account using CollectResults visits every entry exactly once. This specifically
// exercises the PrefixInclusiveEnd logic: when a cursor lands at firstHeight, the iterator
// range must still include all remaining entries at that height.
func TestFTTransfers_PaginationCoversAllEntries(t *testing.T) {
	t.Parallel()

	const firstHeight = uint64(5)
	const pageSize = uint32(3)

	account := unittest.RandomAddressFixture()
	other := unittest.RandomAddressFixture()

	// Bootstrap with 3 transfers at firstHeight so that all 3 are stored at the
	// first indexed height. When the page boundary later falls exactly at firstHeight,
	// PrefixInclusiveEnd must pad the end key so the iterator covers all entries there.
	initialTransfers := []access.FungibleTokenTransfer{
		makeTestTransfer(account, other, firstHeight, 0, 0, "A.FlowToken", big.NewInt(1)),
		makeTestTransfer(account, other, firstHeight, 1, 0, "A.FlowToken", big.NewInt(2)),
		makeTestTransfer(account, other, firstHeight, 2, 0, "A.FlowToken", big.NewInt(3)),
	}

	RunWithBootstrappedFTTransferIndex(t, firstHeight, initialTransfers, func(_ storage.DB, lm storage.LockManager, idx *FungibleTokenTransfers) {
		// 3 more transfers at height 6 (one above firstHeight)
		err := storeFTTransfers(t, lm, idx, 6, []access.FungibleTokenTransfer{
			makeTestTransfer(account, other, 6, 0, 0, "A.FlowToken", big.NewInt(4)),
			makeTestTransfer(account, other, 6, 1, 0, "A.FlowToken", big.NewInt(5)),
			makeTestTransfer(account, other, 6, 2, 0, "A.FlowToken", big.NewInt(6)),
		})
		require.NoError(t, err)

		// Paginate using CollectResults until cursor is nil.
		// Page 1 (cursor=nil) collects height-6 entries and returns a cursor pointing
		// to firstHeight. Page 2 must still return all 3 entries at firstHeight.
		var allCollected []access.FungibleTokenTransfer
		var cursor *access.TransferCursor
		for {
			ftIter, err := idx.ByAddress(account, cursor)
			require.NoError(t, err)

			page, nextCursor, err := iterator.CollectResults(ftIter, pageSize, nil)
			require.NoError(t, err)

			allCollected = append(allCollected, page...)
			cursor = nextCursor
			if cursor == nil {
				break
			}
		}

		// All 6 transfers must be visited exactly once.
		require.Len(t, allCollected, 6)

		// First 3 results are from height 6 (newest first), next 3 from firstHeight.
		for i := 0; i < 3; i++ {
			assert.Equal(t, uint64(6), allCollected[i].BlockHeight)
			assert.Equal(t, uint32(i), allCollected[i].TransactionIndex)
		}
		for i := 0; i < 3; i++ {
			assert.Equal(t, firstHeight, allCollected[3+i].BlockHeight)
			assert.Equal(t, uint32(i), allCollected[3+i].TransactionIndex)
		}
	})
}
