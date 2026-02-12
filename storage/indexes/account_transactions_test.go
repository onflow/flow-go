package indexes

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAccountTransactions_Initialize(t *testing.T) {
	t.Parallel()

	t.Run("uninitialized database returns ErrNotBootstrapped", func(t *testing.T) {
		db, _ := unittest.TempPebbleDBWithOpts(t, nil)
		defer db.Close()

		storageDB := pebbleimpl.ToDB(db)
		_, err := NewAccountTransactions(storageDB)
		require.ErrorIs(t, err, storage.ErrNotBootstrapped)
	})

	t.Run("bootstrap initializes the index", func(t *testing.T) {
		db, _ := unittest.TempPebbleDBWithOpts(t, nil)
		defer db.Close()

		storageDB := pebbleimpl.ToDB(db)
		lockManager := storage.NewTestingLockManager()

		var idx *AccountTransactions
		err := unittest.WithLock(t, lockManager, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				var bootstrapErr error
				idx, bootstrapErr = BootstrapAccountTransactions(lctx, rw, storageDB, 1, nil)
				return bootstrapErr
			})
		})
		require.NoError(t, err)

		first := idx.FirstIndexedHeight()
		assert.Equal(t, uint64(1), first)

		latest := idx.LatestIndexedHeight()
		assert.Equal(t, uint64(1), latest)
	})

	t.Run("succeeds on initialized database", func(t *testing.T) {
		RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
			first := idx.FirstIndexedHeight()
			assert.Equal(t, uint64(1), first)

			latest := idx.LatestIndexedHeight()
			assert.Equal(t, uint64(1), latest)
		})
	})
}

func TestAccountTransactions_IndexAndQuery(t *testing.T) {
	t.Parallel()

	t.Run("index single block with single transaction", func(t *testing.T) {
		RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
			// Create test data
			account1 := unittest.RandomAddressFixture()
			txID := unittest.IdentifierFixture()

			txData := []access.AccountTransaction{
				{
					Address:          account1,
					BlockHeight:      2,
					TransactionID:    txID,
					TransactionIndex: 0,
					IsAuthorizer:     true,
				},
			}

			// Index block at height 2
			err := storeAccountTransactions(t, idx, 2, txData)
			require.NoError(t, err)

			// Query should return the transaction
			results, err := idx.TransactionsByAddress(account1, 2, 2)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, txID, results[0].TransactionID)
			assert.Equal(t, uint64(2), results[0].BlockHeight)
			assert.Equal(t, uint32(0), results[0].TransactionIndex)
			assert.True(t, results[0].IsAuthorizer)
		})
	})

	t.Run("index multiple blocks with multiple transactions", func(t *testing.T) {
		RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
			account1 := unittest.RandomAddressFixture()
			account2 := unittest.RandomAddressFixture()
			require.NotEqual(t, account1, account2, "accounts should be different")

			// Block 2: one tx involving account1
			txID1 := unittest.IdentifierFixture()
			err := storeAccountTransactions(t, idx, 2, []access.AccountTransaction{
				{
					Address:          account1,
					BlockHeight:      2,
					TransactionID:    txID1,
					TransactionIndex: 0,
					IsAuthorizer:     true,
				},
			})
			require.NoError(t, err)

			// Block 3: two txs, one involving both accounts
			txID2 := unittest.IdentifierFixture()
			txID3 := unittest.IdentifierFixture()
			err = storeAccountTransactions(t, idx, 3, []access.AccountTransaction{
				// tx2 involves both account1 and account2
				{
					Address:          account1,
					BlockHeight:      3,
					TransactionID:    txID2,
					TransactionIndex: 0,
					IsAuthorizer:     false,
				},
				{
					Address:          account2,
					BlockHeight:      3,
					TransactionID:    txID2,
					TransactionIndex: 0,
					IsAuthorizer:     true,
				},
				// tx3 involves only account2
				{
					Address:          account2,
					BlockHeight:      3,
					TransactionID:    txID3,
					TransactionIndex: 1,
					IsAuthorizer:     false,
				},
			})
			require.NoError(t, err)

			// Query account1 (should have 2 txs)
			results, err := idx.TransactionsByAddress(account1, 2, 3)
			require.NoError(t, err)
			require.Len(t, results, 2)

			// Results should be in descending order (newest first)
			assert.Equal(t, txID2, results[0].TransactionID)
			assert.Equal(t, uint64(3), results[0].BlockHeight)
			assert.False(t, results[0].IsAuthorizer)

			assert.Equal(t, txID1, results[1].TransactionID)
			assert.Equal(t, uint64(2), results[1].BlockHeight)
			assert.True(t, results[1].IsAuthorizer)

			// Query account2 (should have 2 txs, both in block 3)
			results, err = idx.TransactionsByAddress(account2, 2, 3)
			require.NoError(t, err)
			require.Len(t, results, 2)

			// Both in block 3, ordered by txIndex ascending
			assert.Equal(t, uint64(3), results[0].BlockHeight)
			assert.Equal(t, uint64(3), results[1].BlockHeight)
			assert.Less(t, results[0].TransactionIndex, results[1].TransactionIndex)
		})
	})

	t.Run("query with height range filter", func(t *testing.T) {
		RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()

			// Index blocks 2, 3, 4
			for height := uint64(2); height <= 4; height++ {
				txID := unittest.IdentifierFixture()
				err := storeAccountTransactions(t, idx, height, []access.AccountTransaction{
					{
						Address:          account,
						BlockHeight:      height,
						TransactionID:    txID,
						TransactionIndex: 0,
						IsAuthorizer:     true,
					},
				})
				require.NoError(t, err)
			}

			// Query only blocks 2-3
			results, err := idx.TransactionsByAddress(account, 2, 3)
			require.NoError(t, err)
			require.Len(t, results, 2)

			// All results should be in range
			for _, r := range results {
				assert.GreaterOrEqual(t, r.BlockHeight, uint64(2))
				assert.LessOrEqual(t, r.BlockHeight, uint64(3))
			}
		})
	})
}

func TestAccountTransactions_DescendingOrder(t *testing.T) {
	t.Parallel()

	RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
		account := unittest.RandomAddressFixture()

		// Index 10 blocks
		for height := uint64(2); height <= 11; height++ {
			txID := unittest.IdentifierFixture()
			err := storeAccountTransactions(t, idx, height, []access.AccountTransaction{
				{
					Address:          account,
					BlockHeight:      height,
					TransactionID:    txID,
					TransactionIndex: 0,
					IsAuthorizer:     true,
				},
			})
			require.NoError(t, err)
		}

		// Query all
		results, err := idx.TransactionsByAddress(account, 2, 11)
		require.NoError(t, err)
		require.Len(t, results, 10)

		// Verify descending order
		for i := 0; i < len(results)-1; i++ {
			assert.Greater(t, results[i].BlockHeight, results[i+1].BlockHeight,
				"results should be in descending order by height")
		}
	})
}

func TestAccountTransactions_ErrorCases(t *testing.T) {
	t.Parallel()

	t.Run("query beyond indexed range", func(t *testing.T) {
		RunWithAccountTxIndex(t, 5, func(idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()

			// Query before first height returns error
			_, err := idx.TransactionsByAddress(account, 3, 5)
			require.ErrorIs(t, err, storage.ErrHeightNotIndexed)

			// Query after latest height clamps to latest (returns empty since no data)
			results, err := idx.TransactionsByAddress(account, 5, 10)
			require.NoError(t, err)
			assert.Empty(t, results)
		})
	})

	t.Run("index non-consecutive height fails", func(t *testing.T) {
		RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
			// Try to index height 5 when latest is 1
			err := storeAccountTransactions(t, idx, 5, nil)
			require.Error(t, err)
		})
	})

	t.Run("store below latest returns ErrAlreadyExists", func(t *testing.T) {
		RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
			// Index height 2 and 3
			err := storeAccountTransactions(t, idx, 2, nil)
			require.NoError(t, err)
			err = storeAccountTransactions(t, idx, 3, nil)
			require.NoError(t, err)

			// Try to store height 1 (below latest=3)
			err = storeAccountTransactions(t, idx, 1, nil)
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})

	t.Run("duplicate key in committed DB returns ErrAlreadyExists", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			db := NewBootstrappedAccountTxIndexForTest(t, dir, 1)
			defer db.Close()

			account := unittest.RandomAddressFixture()
			txID := unittest.IdentifierFixture()
			txData := []access.AccountTransaction{
				{
					Address:          account,
					BlockHeight:      2,
					TransactionID:    txID,
					TransactionIndex: 0,
					IsAuthorizer:     true,
				},
			}

			idx, err := NewAccountTransactions(db)
			require.NoError(t, err)

			// Store txData at height 2
			err = storeAccountTransactions(t, idx, 2, txData)
			require.NoError(t, err)

			// Simulate a partial write scenario: roll back the latest height marker
			// in the DB from 2 to 1, as if the tx keys were committed but the
			// height marker update was lost (e.g. crash between writes).
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertByKey(rw.Writer(), keyAccountTransactionLatestHeightKey, uint64(1))
			})
			require.NoError(t, err)

			// Now indexAccountTransactions at height 2 passes the consecutive check
			// (2 == 1+1) but finds the already-committed keys.
			lockManager := storage.NewTestingLockManager()
			err = unittest.WithLock(t, lockManager, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return indexAccountTransactions(lctx, rw, 2, txData)
				})
			})
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})

	t.Run("block height mismatch in entry fails", func(t *testing.T) {
		RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()
			txID := unittest.IdentifierFixture()

			// Entry claims height 5 but we're indexing height 2
			err := storeAccountTransactions(t, idx, 2, []access.AccountTransaction{
				{
					Address:          account,
					BlockHeight:      5, // mismatch with height 2
					TransactionID:    txID,
					TransactionIndex: 0,
					IsAuthorizer:     true,
				},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "block height mismatch")
		})
	})

	t.Run("index same height is idempotent", func(t *testing.T) {
		RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
			// Index height 2
			err := storeAccountTransactions(t, idx, 2, nil)
			require.NoError(t, err)

			// Index height 2 again (should succeed)
			err = storeAccountTransactions(t, idx, 2, nil)
			require.NoError(t, err)
		})
	})

	t.Run("start height greater than end height", func(t *testing.T) {
		RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()

			_, err := idx.TransactionsByAddress(account, 5, 2)
			require.Error(t, err)
		})
	})
}

func TestAccountTransactions_KeyEncoding(t *testing.T) {
	t.Parallel()

	t.Run("key encoding and decoding roundtrip", func(t *testing.T) {
		address := unittest.RandomAddressFixture()
		height := uint64(12345)
		txID := unittest.IdentifierFixture()
		txIndex := uint32(42)

		key := makeAccountTxKey(address, height, txIndex)
		value := encodeAccountTxValue(txID, true)

		// Decode the key
		decodedAddress, decodedHeight, decodedTxIndex, err := decodeAccountTxKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, decodedAddress)
		assert.Equal(t, height, decodedHeight)
		assert.Equal(t, txIndex, decodedTxIndex)

		// Decode the value (msgpack-encoded storedAccountTransaction)
		var stored storedAccountTransaction
		err = msgpack.Unmarshal(value, &stored)
		require.NoError(t, err)
		assert.Equal(t, txID, stored.TransactionID)
		assert.True(t, stored.IsAuthorizer)
	})

	t.Run("value encoding", func(t *testing.T) {
		txID := unittest.IdentifierFixture()

		// Test encoding and decoding true
		encoded := encodeAccountTxValue(txID, true)
		var stored storedAccountTransaction
		err := msgpack.Unmarshal(encoded, &stored)
		require.NoError(t, err)
		assert.Equal(t, txID, stored.TransactionID)
		assert.True(t, stored.IsAuthorizer)

		// Test encoding and decoding false
		encoded = encodeAccountTxValue(txID, false)
		err = msgpack.Unmarshal(encoded, &stored)
		require.NoError(t, err)
		assert.Equal(t, txID, stored.TransactionID)
		assert.False(t, stored.IsAuthorizer)

		// Test error cases
		err = msgpack.Unmarshal(nil, &stored)
		require.Error(t, err)
		err = msgpack.Unmarshal([]byte{}, &stored)
		require.Error(t, err)
	})
}

func TestAccountTransactions_EmptyResults(t *testing.T) {
	t.Parallel()

	RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
		// Query for account that has no transactions
		account := unittest.RandomAddressFixture()

		// First index some data so we have indexed heights
		err := storeAccountTransactions(t, idx, 2, nil)
		require.NoError(t, err)

		results, err := idx.TransactionsByAddress(account, 1, 2)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func storeAccountTransactions(tb testing.TB, idx *AccountTransactions, height uint64, txData []access.AccountTransaction) error {
	lockManager := storage.NewTestingLockManager()
	return unittest.WithLock(tb, lockManager, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
		return idx.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return idx.Store(lctx, rw, height, txData)
		})
	})
}

// RunWithAccountTxIndex creates a temporary Pebble database, bootstraps the
// AccountTransactions at the given start height, and runs the provided function.
func RunWithAccountTxIndex(tb testing.TB, startHeight uint64, f func(idx *AccountTransactions)) {
	unittest.RunWithTempDir(tb, func(dir string) {
		db := NewBootstrappedAccountTxIndexForTest(tb, dir, startHeight)
		defer db.Close()

		idx, err := NewAccountTransactions(db)
		require.NoError(tb, err)

		f(idx)
	})
}

// NewBootstrappedAccountTxIndexForTest creates a new Pebble database and bootstraps it
// for account transaction indexing at the given start height.
func NewBootstrappedAccountTxIndexForTest(tb testing.TB, dir string, startHeight uint64) storage.DB {
	db, err := pebble.Open(dir, &pebble.Options{})
	require.NoError(tb, err)

	storageDB := pebbleimpl.ToDB(db)

	lockManager := storage.NewTestingLockManager()
	err = unittest.WithLock(tb, lockManager, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
		return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			_, bootstrapErr := BootstrapAccountTransactions(lctx, rw, storageDB, startHeight, nil)
			return bootstrapErr
		})
	})
	require.NoError(tb, err)

	return storageDB
}

// encodeAccountTxValue encodes a transaction ID and authorizer flag using msgpack.
// This matches the encoding used in the implementation.
func encodeAccountTxValue(txID flow.Identifier, isAuthorizer bool) []byte {
	type storedAccountTransaction struct {
		TransactionID flow.Identifier
		IsAuthorizer  bool
	}
	stored := storedAccountTransaction{
		TransactionID: txID,
		IsAuthorizer:  isAuthorizer,
	}
	data, _ := msgpack.Marshal(&stored)
	return data
}
