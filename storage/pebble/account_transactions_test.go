package pebble

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAccountTransactions_Initialize(t *testing.T) {
	t.Parallel()

	t.Run("fails on uninitialized database", func(t *testing.T) {
		db, _ := unittest.TempPebbleDBWithOpts(t, nil)
		defer db.Close()

		_, err := NewAccountTransactions(db)
		require.Error(t, err)
	})

	t.Run("succeeds on initialized database", func(t *testing.T) {
		RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
			first, err := idx.FirstIndexedHeight()
			require.NoError(t, err)
			assert.Equal(t, uint64(1), first)

			latest, err := idx.LatestIndexedHeight()
			require.NoError(t, err)
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
			err := idx.Store(2, txData)
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
			err := idx.Store(2, []access.AccountTransaction{
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
			err = idx.Store(3, []access.AccountTransaction{
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
			t.Logf("Results for account1: %+v", results)
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

			// Both in block 3, ordered by txIndex descending
			assert.Equal(t, uint64(3), results[0].BlockHeight)
			assert.Equal(t, uint64(3), results[1].BlockHeight)
		})
	})

	t.Run("query with height range filter", func(t *testing.T) {
		RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()

			// Index blocks 2, 3, 4
			for height := uint64(2); height <= 4; height++ {
				txID := unittest.IdentifierFixture()
				err := idx.Store(height, []access.AccountTransaction{
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
			err := idx.Store(height, []access.AccountTransaction{
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
			err := idx.Store(5, nil)
			require.Error(t, err)
		})
	})

	t.Run("index same height is idempotent", func(t *testing.T) {
		RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
			// Index height 2
			err := idx.Store(2, nil)
			require.NoError(t, err)

			// Index height 2 again (should succeed)
			err = idx.Store(2, nil)
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

		key := makeAccountTxKey(address, height, txID, txIndex)
		value := encodeAccountTxValue(true)

		decoded, err := decodeAccountTxKey(key, value)
		require.NoError(t, err)

		assert.Equal(t, height, decoded.BlockHeight)
		assert.Equal(t, txID, decoded.TransactionID)
		assert.Equal(t, txIndex, decoded.TransactionIndex)
		assert.True(t, decoded.IsAuthorizer)
	})

	t.Run("value encoding", func(t *testing.T) {
		assert.True(t, decodeAccountTxValue(encodeAccountTxValue(true)))
		assert.False(t, decodeAccountTxValue(encodeAccountTxValue(false)))
		assert.False(t, decodeAccountTxValue(nil))
		assert.False(t, decodeAccountTxValue([]byte{}))
	})
}

func TestAccountTransactions_EmptyResults(t *testing.T) {
	t.Parallel()

	RunWithAccountTxIndex(t, 1, func(idx *AccountTransactions) {
		// Query for account that has no transactions
		account := unittest.RandomAddressFixture()

		// First index some data so we have indexed heights
		err := idx.Store(2, nil)
		require.NoError(t, err)

		results, err := idx.TransactionsByAddress(account, 1, 2)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

// RunWithAccountTxIndex creates a temporary Pebble database, initializes the
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

// NewBootstrappedAccountTxIndexForTest creates a new Pebble database and initializes it
// for account transaction indexing at the given start height.
func NewBootstrappedAccountTxIndexForTest(tb testing.TB, dir string, startHeight uint64) *pebble.DB {
	db, err := pebble.Open(dir, &pebble.Options{})
	require.NoError(tb, err)

	err = InitAccountTransactions(db, startHeight)
	require.NoError(tb, err)

	return db
}
