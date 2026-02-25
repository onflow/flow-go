package indexes

import (
	"errors"
	"math"
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

// queryAllNFTTransfers is a test helper that queries all transfers for the given account
// using a large limit and no cursor or filter.
func queryAllNFTTransfers(t *testing.T, idx *NonFungibleTokenTransfers, account flow.Address) []access.NonFungibleTokenTransfer {
	t.Helper()
	page, err := idx.ByAddress(account, 100, nil, nil)
	require.NoError(t, err)
	return page.Transfers
}

func TestNFTTransfers_Initialize(t *testing.T) {
	t.Parallel()

	t.Run("uninitialized database returns ErrNotBootstrapped", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			_, err := NewNonFungibleTokenTransfers(storageDB)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})
	})

	t.Run("corrupted DB with firstHeight but no latestHeight returns exception", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)

			// Write only the firstHeight key, simulating a corrupted state
			err := storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertByKey(rw.Writer(), keyAccountNFTTransferFirstHeightKey, uint64(10))
			})
			require.NoError(t, err)

			_, err = NewNonFungibleTokenTransfers(storageDB)
			require.Error(t, err)
			assert.False(t, errors.Is(err, storage.ErrNotBootstrapped),
				"should not return ErrNotBootstrapped for corrupted state")
		})
	})

	t.Run("bootstrap initializes the index", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(_ storage.DB, _ storage.LockManager, idx *NonFungibleTokenTransfers) {
			first := idx.FirstIndexedHeight()
			assert.Equal(t, uint64(1), first)

			latest := idx.LatestIndexedHeight()
			assert.Equal(t, uint64(1), latest)
		})
	})

	t.Run("bootstrap with initial data", func(t *testing.T) {
		source := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()

		initialData := []access.NonFungibleTokenTransfer{
			{
				TransactionID:    txID,
				BlockHeight:      1,
				TransactionIndex: 0,
				EventIndices:     []uint32{0},
				SourceAddress:    source,
				RecipientAddress: recipient,
				ID:               42,
			},
		}
		RunWithBootstrappedNFTTransferIndex(t, 1, initialData, func(_ storage.DB, _ storage.LockManager, idx *NonFungibleTokenTransfers) {
			first := idx.FirstIndexedHeight()
			assert.Equal(t, uint64(1), first)

			latest := idx.LatestIndexedHeight()
			assert.Equal(t, uint64(1), latest)

			// Query by source
			page, err := idx.ByAddress(source, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 1)
			assert.Equal(t, txID, page.Transfers[0].TransactionID)
			assert.Equal(t, uint64(1), page.Transfers[0].BlockHeight)
			assert.Equal(t, uint32(0), page.Transfers[0].TransactionIndex)
			assert.Equal(t, uint32(0), page.Transfers[0].EventIndices[0])
			assert.Equal(t, source, page.Transfers[0].SourceAddress)
			assert.Equal(t, recipient, page.Transfers[0].RecipientAddress)
			assert.Equal(t, uint64(42), page.Transfers[0].ID)

			// Query by recipient
			page, err = idx.ByAddress(recipient, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 1)
			assert.Equal(t, txID, page.Transfers[0].TransactionID)
		})
	})

	t.Run("bootstrap at height 0", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 0, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			assert.Equal(t, uint64(0), idx.FirstIndexedHeight())
			assert.Equal(t, uint64(0), idx.LatestIndexedHeight())

			// Store at height 1 (consecutive after 0)
			source := unittest.RandomAddressFixture()
			recipient := unittest.RandomAddressFixture()
			txID := unittest.IdentifierFixture()
			err := storeNFTTransfers(t, lm, idx, 1, []access.NonFungibleTokenTransfer{
				{
					TransactionID:    txID,
					BlockHeight:      1,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    source,
					RecipientAddress: recipient,
					ID:               1,
				},
			})
			require.NoError(t, err)

			assert.Equal(t, uint64(1), idx.LatestIndexedHeight())

			page, err := idx.ByAddress(source, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 1)
			assert.Equal(t, txID, page.Transfers[0].TransactionID)
		})
	})
}

func TestNFTTransfers_StoreAndQuery(t *testing.T) {
	t.Parallel()

	t.Run("single block with single transfer", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			source := unittest.RandomAddressFixture()
			recipient := unittest.RandomAddressFixture()
			txID := unittest.IdentifierFixture()

			transfers := []access.NonFungibleTokenTransfer{
				{
					TransactionID:    txID,
					BlockHeight:      2,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    source,
					RecipientAddress: recipient,
					ID:               100,
				},
			}

			err := storeNFTTransfers(t, lm, idx, 2, transfers)
			require.NoError(t, err)

			page, err := idx.ByAddress(source, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 1)
			assert.Equal(t, txID, page.Transfers[0].TransactionID)
			assert.Equal(t, uint64(2), page.Transfers[0].BlockHeight)
			assert.Equal(t, uint32(0), page.Transfers[0].TransactionIndex)
			assert.Equal(t, uint32(0), page.Transfers[0].EventIndices[0])
			assert.Equal(t, source, page.Transfers[0].SourceAddress)
			assert.Equal(t, recipient, page.Transfers[0].RecipientAddress)
			assert.Equal(t, uint64(100), page.Transfers[0].ID)
		})
	})

	t.Run("multiple accounts", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			account1 := unittest.RandomAddressFixture()
			account2 := unittest.RandomAddressFixture()
			account3 := unittest.RandomAddressFixture()
			require.NotEqual(t, account1, account2)
			require.NotEqual(t, account2, account3)

			txID1 := unittest.IdentifierFixture()
			txID2 := unittest.IdentifierFixture()

			// Block 2: account1 -> account2
			err := storeNFTTransfers(t, lm, idx, 2, []access.NonFungibleTokenTransfer{
				{
					TransactionID:    txID1,
					BlockHeight:      2,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    account1,
					RecipientAddress: account2,
					ID:               1,
				},
			})
			require.NoError(t, err)

			// Block 3: account2 -> account3
			err = storeNFTTransfers(t, lm, idx, 3, []access.NonFungibleTokenTransfer{
				{
					TransactionID:    txID2,
					BlockHeight:      3,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    account2,
					RecipientAddress: account3,
					ID:               2,
				},
			})
			require.NoError(t, err)

			// account1: 1 transfer (source in block 2)
			results := queryAllNFTTransfers(t, idx, account1)
			require.Len(t, results, 1)
			assert.Equal(t, txID1, results[0].TransactionID)

			// account2: 2 transfers (recipient in block 2, source in block 3)
			results = queryAllNFTTransfers(t, idx, account2)
			require.Len(t, results, 2)

			// account3: 1 transfer (recipient in block 3)
			results = queryAllNFTTransfers(t, idx, account3)
			require.Len(t, results, 1)
			assert.Equal(t, txID2, results[0].TransactionID)
		})
	})

	t.Run("dual indexing source and recipient", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			source := unittest.RandomAddressFixture()
			recipient := unittest.RandomAddressFixture()
			require.NotEqual(t, source, recipient)
			txID := unittest.IdentifierFixture()

			err := storeNFTTransfers(t, lm, idx, 2, []access.NonFungibleTokenTransfer{
				{
					TransactionID:    txID,
					BlockHeight:      2,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    source,
					RecipientAddress: recipient,
					ID:               7,
				},
			})
			require.NoError(t, err)

			// Both source and recipient should see the transfer
			sourceResults := queryAllNFTTransfers(t, idx, source)
			require.Len(t, sourceResults, 1)
			assert.Equal(t, txID, sourceResults[0].TransactionID)

			recipientResults := queryAllNFTTransfers(t, idx, recipient)
			require.Len(t, recipientResults, 1)
			assert.Equal(t, txID, recipientResults[0].TransactionID)
		})
	})

	t.Run("descending order", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			account := unittest.RandomAddressFixture()

			// Index 10 blocks
			for height := uint64(2); height <= 11; height++ {
				err := storeNFTTransfers(t, lm, idx, height, []access.NonFungibleTokenTransfer{
					{
						TransactionID:    unittest.IdentifierFixture(),
						BlockHeight:      height,
						TransactionIndex: 0,
						EventIndices:     []uint32{0},
						SourceAddress:    account,
						RecipientAddress: unittest.RandomAddressFixture(),
						ID:               height,
					},
				})
				require.NoError(t, err)
			}

			results := queryAllNFTTransfers(t, idx, account)
			require.Len(t, results, 10)

			// Verify descending order
			for i := 0; i < len(results)-1; i++ {
				assert.Greater(t, results[i].BlockHeight, results[i+1].BlockHeight,
					"results should be in descending order by height")
			}
		})
	})

	t.Run("multiple transfers at same height", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			account := unittest.RandomAddressFixture()
			txID0 := unittest.IdentifierFixture()
			txID1 := unittest.IdentifierFixture()

			err := storeNFTTransfers(t, lm, idx, 2, []access.NonFungibleTokenTransfer{
				{
					TransactionID:    txID0,
					BlockHeight:      2,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    account,
					RecipientAddress: unittest.RandomAddressFixture(),
					ID:               1,
				},
				{
					TransactionID:    txID1,
					BlockHeight:      2,
					TransactionIndex: 1,
					EventIndices:     []uint32{0},
					SourceAddress:    account,
					RecipientAddress: unittest.RandomAddressFixture(),
					ID:               2,
				},
			})
			require.NoError(t, err)

			results := queryAllNFTTransfers(t, idx, account)
			require.Len(t, results, 2)

			// Same height, ascending txIndex
			assert.Equal(t, uint64(2), results[0].BlockHeight)
			assert.Equal(t, uint64(2), results[1].BlockHeight)
			assert.Less(t, results[0].TransactionIndex, results[1].TransactionIndex)
		})
	})
}

func TestNFTTransfers_HeightValidation(t *testing.T) {
	t.Parallel()

	t.Run("repeated store at latest height returns ErrAlreadyExists", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			// Index height 2
			err := storeNFTTransfers(t, lm, idx, 2, nil)
			require.NoError(t, err)

			// Re-indexing height 2 should return ErrAlreadyExists
			err = storeNFTTransfers(t, lm, idx, 2, nil)
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})

	t.Run("store below latest returns ErrAlreadyExists", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			err := storeNFTTransfers(t, lm, idx, 2, nil)
			require.NoError(t, err)
			err = storeNFTTransfers(t, lm, idx, 3, nil)
			require.NoError(t, err)

			// Try to store height 1 (below latest=3)
			err = storeNFTTransfers(t, lm, idx, 1, nil)
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})

	t.Run("store non-consecutive height fails", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			// Try to index height 5 when latest is 1
			err := storeNFTTransfers(t, lm, idx, 5, nil)
			require.Error(t, err)
			assert.False(t, errors.Is(err, storage.ErrAlreadyExists),
				"non-consecutive height should not return ErrAlreadyExists")
		})
	})

	t.Run("block height mismatch in entry fails", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			// Entry claims height 5 but we're indexing height 2
			err := storeNFTTransfers(t, lm, idx, 2, []access.NonFungibleTokenTransfer{
				{
					TransactionID:    unittest.IdentifierFixture(),
					BlockHeight:      5, // mismatch
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    unittest.RandomAddressFixture(),
					RecipientAddress: unittest.RandomAddressFixture(),
					ID:               1,
				},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "block height mismatch")
		})
	})

	t.Run("store with nil EventIndices fails", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			err := storeNFTTransfers(t, lm, idx, 2, []access.NonFungibleTokenTransfer{
				{
					TransactionID:    unittest.IdentifierFixture(),
					BlockHeight:      2,
					TransactionIndex: 0,
					EventIndices:     nil, // invalid: must have at least one event index
					SourceAddress:    unittest.RandomAddressFixture(),
					RecipientAddress: unittest.RandomAddressFixture(),
					ID:               1,
				},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "at least one event index")
		})
	})

	t.Run("store with empty EventIndices fails", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			err := storeNFTTransfers(t, lm, idx, 2, []access.NonFungibleTokenTransfer{
				{
					TransactionID:    unittest.IdentifierFixture(),
					BlockHeight:      2,
					TransactionIndex: 0,
					EventIndices:     []uint32{}, // invalid: must have at least one event index
					SourceAddress:    unittest.RandomAddressFixture(),
					RecipientAddress: unittest.RandomAddressFixture(),
					ID:               1,
				},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "at least one event index")
		})
	})
}

func TestNFTTransfers_RangeQueries(t *testing.T) {
	t.Parallel()

	t.Run("cursor height greater than latest returns ErrHeightNotIndexed", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 5, nil, func(_ storage.DB, _ storage.LockManager, idx *NonFungibleTokenTransfers) {
			account := unittest.RandomAddressFixture()
			cursor := &access.TransferCursor{BlockHeight: 100}
			_, err := idx.ByAddress(account, 10, cursor, nil)
			require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
		})
	})

	t.Run("cursor height before first returns ErrHeightNotIndexed", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 5, nil, func(_ storage.DB, _ storage.LockManager, idx *NonFungibleTokenTransfers) {
			account := unittest.RandomAddressFixture()
			cursor := &access.TransferCursor{BlockHeight: 1}
			_, err := idx.ByAddress(account, 10, cursor, nil)
			require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
		})
	})

	t.Run("nil cursor queries from latest", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			account := unittest.RandomAddressFixture()
			txID := unittest.IdentifierFixture()

			err := storeNFTTransfers(t, lm, idx, 2, []access.NonFungibleTokenTransfer{
				{
					TransactionID:    txID,
					BlockHeight:      2,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    account,
					RecipientAddress: unittest.RandomAddressFixture(),
					ID:               1,
				},
			})
			require.NoError(t, err)

			// nil cursor should query from latest
			page, err := idx.ByAddress(account, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 1)
			assert.Equal(t, txID, page.Transfers[0].TransactionID)
		})
	})

	t.Run("limit zero returns ErrInvalidQuery", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 5, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			account := unittest.RandomAddressFixture()
			_, err := idx.ByAddress(account, 0, nil, nil)
			require.ErrorIs(t, err, storage.ErrInvalidQuery)
		})
	})

	t.Run("empty results for account with no transfers", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
			account := unittest.RandomAddressFixture()

			// Index some data so we have indexed heights
			err := storeNFTTransfers(t, lm, idx, 2, nil)
			require.NoError(t, err)

			results := queryAllNFTTransfers(t, idx, account)
			assert.Empty(t, results)
		})
	})
}

func TestNFTTransfers_KeyEncoding(t *testing.T) {
	t.Parallel()

	t.Run("key encoding and decoding roundtrip", func(t *testing.T) {
		address := unittest.RandomAddressFixture()
		height := uint64(12345)
		txIndex := uint32(42)
		eventIndex := uint32(7)

		key := makeNFTTransferKey(address, height, txIndex, eventIndex)

		decodedAddress, decodedHeight, decodedTxIndex, decodedEventIndex, err := decodeNFTTransferKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, decodedAddress)
		assert.Equal(t, height, decodedHeight)
		assert.Equal(t, txIndex, decodedTxIndex)
		assert.Equal(t, eventIndex, decodedEventIndex)
	})

	t.Run("boundary values: height 0, txIndex 0, eventIndex 0", func(t *testing.T) {
		address := unittest.RandomAddressFixture()
		key := makeNFTTransferKey(address, 0, 0, 0)
		decodedAddress, decodedHeight, decodedTxIndex, decodedEventIndex, err := decodeNFTTransferKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, decodedAddress)
		assert.Equal(t, uint64(0), decodedHeight)
		assert.Equal(t, uint32(0), decodedTxIndex)
		assert.Equal(t, uint32(0), decodedEventIndex)
	})

	t.Run("boundary values: max height, max txIndex, max eventIndex", func(t *testing.T) {
		address := unittest.RandomAddressFixture()
		key := makeNFTTransferKey(address, math.MaxUint64, math.MaxUint32, math.MaxUint32)
		decodedAddress, decodedHeight, decodedTxIndex, decodedEventIndex, err := decodeNFTTransferKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, decodedAddress)
		assert.Equal(t, uint64(math.MaxUint64), decodedHeight)
		assert.Equal(t, uint32(math.MaxUint32), decodedTxIndex)
		assert.Equal(t, uint32(math.MaxUint32), decodedEventIndex)
	})

	t.Run("boundary values: zero address", func(t *testing.T) {
		address := flow.Address{}
		key := makeNFTTransferKey(address, 12345, 42, 7)
		decodedAddress, decodedHeight, decodedTxIndex, decodedEventIndex, err := decodeNFTTransferKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, decodedAddress)
		assert.Equal(t, uint64(12345), decodedHeight)
		assert.Equal(t, uint32(42), decodedTxIndex)
		assert.Equal(t, uint32(7), decodedEventIndex)
	})
}

func TestNFTTransfers_KeyDecoding_Errors(t *testing.T) {
	t.Parallel()

	t.Run("key too short", func(t *testing.T) {
		_, _, _, _, err := decodeNFTTransferKey(make([]byte, 10))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid key length")
	})

	t.Run("key too long", func(t *testing.T) {
		_, _, _, _, err := decodeNFTTransferKey(make([]byte, 30))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid key length")
	})

	t.Run("invalid prefix", func(t *testing.T) {
		key := make([]byte, nftTransferKeyLen)
		key[0] = 0xFF // wrong prefix
		_, _, _, _, err := decodeNFTTransferKey(key)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid prefix")
	})
}

func TestNFTTransfers_LockRequirement(t *testing.T) {
	t.Parallel()

	t.Run("Store without lock returns error", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(db storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
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

	t.Run("initializeNFTTransfers without lock returns error", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			lctx := lm.NewContext()
			defer lctx.Release()

			// Call without acquiring the required lock
			err := storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapNonFungibleTokenTransfers(lctx, rw, storageDB, 1, nil)
				return bootstrapErr
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing required lock")
		})
	})
}

func TestNFTTransfers_UncommittedBatch(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedNFTTransferIndex(t, 1, nil, func(db storage.DB, lm storage.LockManager, idx *NonFungibleTokenTransfers) {
		require.Equal(t, uint64(1), idx.LatestIndexedHeight())

		transfers := []access.NonFungibleTokenTransfer{
			{
				TransactionID:    unittest.IdentifierFixture(),
				BlockHeight:      2,
				TransactionIndex: 0,
				EventIndices:     []uint32{0},
				SourceAddress:    unittest.RandomAddressFixture(),
				RecipientAddress: unittest.RandomAddressFixture(),
				ID:               1,
			},
		}

		// Create a batch manually and store data without committing.
		batch := db.NewBatch()
		err := unittest.WithLock(t, lm, storage.LockIndexNonFungibleTokenTransfers, func(lctx lockctx.Context) error {
			return idx.Store(lctx, batch, 2, transfers)
		})
		require.NoError(t, err)

		// Close the batch without committing - discards pending writes
		require.NoError(t, batch.Close())

		// latestHeight must still be 1 since the batch was never committed
		assert.Equal(t, uint64(1), idx.LatestIndexedHeight(),
			"latestHeight should not update when the batch is not committed")
	})
}

func TestNFTTransfers_BootstrapHeightMismatch(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		lm := storage.NewTestingLockManager()

		// Entry claims height 99 but we're bootstrapping at height 5
		err := unittest.WithLock(t, lm, storage.LockIndexNonFungibleTokenTransfers, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapNonFungibleTokenTransfers(lctx, rw, storageDB, 5, []access.NonFungibleTokenTransfer{
					{
						TransactionID:    unittest.IdentifierFixture(),
						BlockHeight:      99, // mismatch with bootstrap height 5
						TransactionIndex: 0,
						EventIndices:     []uint32{0},
						SourceAddress:    unittest.RandomAddressFixture(),
						RecipientAddress: unittest.RandomAddressFixture(),
						ID:               1,
					},
				})
				return bootstrapErr
			})
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "block height mismatch")
	})
}

func TestNFTTransfers_BootstrapEmptyEventIndices(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		lm := storage.NewTestingLockManager()

		err := unittest.WithLock(t, lm, storage.LockIndexNonFungibleTokenTransfers, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapNonFungibleTokenTransfers(lctx, rw, storageDB, 5, []access.NonFungibleTokenTransfer{
					{
						TransactionID:    unittest.IdentifierFixture(),
						BlockHeight:      5,
						TransactionIndex: 0,
						EventIndices:     nil, // invalid: must have at least one event index
						SourceAddress:    unittest.RandomAddressFixture(),
						RecipientAddress: unittest.RandomAddressFixture(),
						ID:               1,
					},
				})
				return bootstrapErr
			})
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one event index")
	})
}

// RunWithBootstrappedNFTTransferIndex creates a new Pebble database and bootstraps it
// for NFT transfer indexing at the given start height. The callback receives a shared
// lock manager that should be passed to storeNFTTransfers for consistent lock usage.
func RunWithBootstrappedNFTTransferIndex(tb testing.TB, startHeight uint64, transfers []access.NonFungibleTokenTransfer, f func(db storage.DB, lockManager storage.LockManager, idx *NonFungibleTokenTransfers)) {
	unittest.RunWithPebbleDB(tb, func(db *pebble.DB) {
		lockManager := storage.NewTestingLockManager()

		var idx *NonFungibleTokenTransfers
		storageDB := pebbleimpl.ToDB(db)
		err := unittest.WithLock(tb, lockManager, storage.LockIndexNonFungibleTokenTransfers, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				var bootstrapErr error
				idx, bootstrapErr = BootstrapNonFungibleTokenTransfers(lctx, rw, storageDB, startHeight, transfers)
				return bootstrapErr
			})
		})
		require.NoError(tb, err)

		f(storageDB, lockManager, idx)
	})
}

func storeNFTTransfers(tb testing.TB, lockManager storage.LockManager, idx *NonFungibleTokenTransfers, height uint64, transfers []access.NonFungibleTokenTransfer) error {
	return unittest.WithLock(tb, lockManager, storage.LockIndexNonFungibleTokenTransfers, func(lctx lockctx.Context) error {
		return idx.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return idx.Store(lctx, rw, height, transfers)
		})
	})
}
