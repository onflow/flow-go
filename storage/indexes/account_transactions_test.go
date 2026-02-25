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

// collectAll drains an iterator into a slice.
func collectAll(tb testing.TB, iter storage.AccountTransactionIterator) []access.AccountTransaction {
	tb.Helper()
	var txs []access.AccountTransaction
	for item := range iter {
		tx, err := item.Value()
		require.NoError(tb, err)
		txs = append(txs, tx)
	}
	return txs
}

func TestAccountTransactions_Initialize(t *testing.T) {
	t.Parallel()

	t.Run("uninitialized database returns ErrNotBootstrapped", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			_, err := NewAccountTransactions(storageDB)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})
	})

	t.Run("corrupted DB with firstHeight but no latestHeight returns exception", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)

			// Write only the firstHeight key, simulating a corrupted state
			err := storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertByKey(rw.Writer(), keyAccountTransactionFirstHeightKey, uint64(10))
			})
			require.NoError(t, err)

			_, err = NewAccountTransactions(storageDB)
			require.Error(t, err)
			assert.False(t, errors.Is(err, storage.ErrNotBootstrapped),
				"should not return ErrNotBootstrapped for corrupted state")
		})
	})

	t.Run("bootstrap initializes the index", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, _ storage.LockManager, idx *AccountTransactions) {
			first := idx.FirstIndexedHeight()
			assert.Equal(t, uint64(1), first)

			latest := idx.LatestIndexedHeight()
			assert.Equal(t, uint64(1), latest)
		})
	})

	t.Run("bootstrap with initial data", func(t *testing.T) {
		initialData := []access.AccountTransaction{
			{
				Address:          unittest.RandomAddressFixture(),
				BlockHeight:      1,
				TransactionID:    unittest.IdentifierFixture(),
				TransactionIndex: 0,
				Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
			},
		}
		RunWithBootstrappedAccountTxIndex(t, 1, initialData, func(_ storage.DB, _ storage.LockManager, idx *AccountTransactions) {
			first := idx.FirstIndexedHeight()
			assert.Equal(t, uint64(1), first)

			latest := idx.LatestIndexedHeight()
			assert.Equal(t, uint64(1), latest)

			iter, err := idx.ByAddress(initialData[0].Address, nil)
			require.NoError(t, err)
			txs := collectAll(t, iter)
			require.Len(t, txs, 1)
			assert.Equal(t, initialData[0].BlockHeight, txs[0].BlockHeight)
			assert.Equal(t, initialData[0].TransactionID, txs[0].TransactionID)
			assert.Equal(t, initialData[0].TransactionIndex, txs[0].TransactionIndex)
			assert.Equal(t, initialData[0].Roles, txs[0].Roles)
		})
	})

	t.Run("bootstrap at height 0", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 0, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
			assert.Equal(t, uint64(0), idx.FirstIndexedHeight())
			assert.Equal(t, uint64(0), idx.LatestIndexedHeight())

			// Store at height 1 (consecutive after 0)
			account := unittest.RandomAddressFixture()
			txID := unittest.IdentifierFixture()
			err := storeAccountTransactions(t, lm, idx, 1, []access.AccountTransaction{
				{
					Address:          account,
					BlockHeight:      1,
					TransactionID:    txID,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
				},
			})
			require.NoError(t, err)

			assert.Equal(t, uint64(1), idx.LatestIndexedHeight())

			iter, err := idx.ByAddress(account, nil)
			require.NoError(t, err)
			txs := collectAll(t, iter)
			require.Len(t, txs, 1)
			assert.Equal(t, txID, txs[0].TransactionID)
		})
	})
}

func TestAccountTransactions_IndexAndQuery(t *testing.T) {
	t.Parallel()

	t.Run("index single block with single transaction", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
			account1 := unittest.RandomAddressFixture()
			txID := unittest.IdentifierFixture()

			txData := []access.AccountTransaction{
				{
					Address:          account1,
					BlockHeight:      2,
					TransactionID:    txID,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
				},
			}

			err := storeAccountTransactions(t, lm, idx, 2, txData)
			require.NoError(t, err)

			iter, err := idx.ByAddress(account1, nil)
			require.NoError(t, err)
			txs := collectAll(t, iter)
			require.Len(t, txs, 1)
			assert.Equal(t, txID, txs[0].TransactionID)
			assert.Equal(t, uint64(2), txs[0].BlockHeight)
			assert.Equal(t, uint32(0), txs[0].TransactionIndex)
			assert.Equal(t, []access.TransactionRole{access.TransactionRoleAuthorizer}, txs[0].Roles)
		})
	})

	t.Run("index multiple blocks with multiple transactions", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
			account1 := unittest.RandomAddressFixture()
			account2 := unittest.RandomAddressFixture()
			require.NotEqual(t, account1, account2, "accounts should be different")

			// Block 2: one tx involving account1
			txID1 := unittest.IdentifierFixture()
			err := storeAccountTransactions(t, lm, idx, 2, []access.AccountTransaction{
				{
					Address:          account1,
					BlockHeight:      2,
					TransactionID:    txID1,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
				},
			})
			require.NoError(t, err)

			// Block 3: two txs, one involving both accounts
			txID2 := unittest.IdentifierFixture()
			txID3 := unittest.IdentifierFixture()
			err = storeAccountTransactions(t, lm, idx, 3, []access.AccountTransaction{
				{
					Address:          account1,
					BlockHeight:      3,
					TransactionID:    txID2,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleInteracted},
				},
				{
					Address:          account2,
					BlockHeight:      3,
					TransactionID:    txID2,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
				},
				{
					Address:          account2,
					BlockHeight:      3,
					TransactionID:    txID3,
					TransactionIndex: 1,
					Roles:            []access.TransactionRole{access.TransactionRoleInteracted},
				},
			})
			require.NoError(t, err)

			// Query account1 (should have 2 txs)
			iter, err := idx.ByAddress(account1, nil)
			require.NoError(t, err)
			txs := collectAll(t, iter)
			require.Len(t, txs, 2)

			// Results should be in descending order (newest first)
			assert.Equal(t, txID2, txs[0].TransactionID)
			assert.Equal(t, uint64(3), txs[0].BlockHeight)
			assert.Equal(t, []access.TransactionRole{access.TransactionRoleInteracted}, txs[0].Roles)

			assert.Equal(t, txID1, txs[1].TransactionID)
			assert.Equal(t, uint64(2), txs[1].BlockHeight)
			assert.Equal(t, []access.TransactionRole{access.TransactionRoleAuthorizer}, txs[1].Roles)

			// Query account2 (should have 2 txs, both in block 3)
			iter, err = idx.ByAddress(account2, nil)
			require.NoError(t, err)
			txs = collectAll(t, iter)
			require.Len(t, txs, 2)

			// Both in block 3, ordered by txIndex ascending
			assert.Equal(t, uint64(3), txs[0].BlockHeight)
			assert.Equal(t, uint64(3), txs[1].BlockHeight)
			assert.Less(t, txs[0].TransactionIndex, txs[1].TransactionIndex)
		})
	})
}

func TestAccountTransactions_CursorPositioning(t *testing.T) {
	t.Parallel()

	t.Run("cursor positions iterator at correct entry", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()

			// Index 5 blocks (heights 2-6), each with 1 tx
			for height := uint64(2); height <= 6; height++ {
				txID := unittest.IdentifierFixture()
				err := storeAccountTransactions(t, lm, idx, height, []access.AccountTransaction{
					{
						Address:          account,
						BlockHeight:      height,
						TransactionID:    txID,
						TransactionIndex: 0,
						Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
					},
				})
				require.NoError(t, err)
			}

			// No cursor: starts from latest (height 6)
			iter, err := idx.ByAddress(account, nil)
			require.NoError(t, err)
			txs := collectAll(t, iter)
			require.Len(t, txs, 5)
			assert.Equal(t, uint64(6), txs[0].BlockHeight)
			assert.Equal(t, uint64(2), txs[4].BlockHeight)

			// Cursor at height 4, txIndex 0: starts from that entry inclusive
			cursor := &access.AccountTransactionCursor{BlockHeight: 4, TransactionIndex: 0}
			iter, err = idx.ByAddress(account, cursor)
			require.NoError(t, err)
			txs = collectAll(t, iter)
			require.Len(t, txs, 3) // heights 4, 3, 2
			assert.Equal(t, uint64(4), txs[0].BlockHeight)
			assert.Equal(t, uint64(3), txs[1].BlockHeight)
			assert.Equal(t, uint64(2), txs[2].BlockHeight)
		})
	})

	t.Run("cursor within same block positions by transaction index", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()

			txID1 := unittest.IdentifierFixture()
			txID2 := unittest.IdentifierFixture()
			txID3 := unittest.IdentifierFixture()
			err := storeAccountTransactions(t, lm, idx, 2, []access.AccountTransaction{
				{Address: account, BlockHeight: 2, TransactionID: txID1, TransactionIndex: 0, Roles: []access.TransactionRole{access.TransactionRoleAuthorizer}},
				{Address: account, BlockHeight: 2, TransactionID: txID2, TransactionIndex: 1, Roles: []access.TransactionRole{access.TransactionRoleAuthorizer}},
				{Address: account, BlockHeight: 2, TransactionID: txID3, TransactionIndex: 2, Roles: []access.TransactionRole{access.TransactionRoleAuthorizer}},
			})
			require.NoError(t, err)

			// Cursor at txIndex 1: starts from txIndex 1 inclusive
			cursor := &access.AccountTransactionCursor{BlockHeight: 2, TransactionIndex: 1}
			iter, err := idx.ByAddress(account, cursor)
			require.NoError(t, err)
			txs := collectAll(t, iter)
			require.Len(t, txs, 2)
			assert.Equal(t, uint32(1), txs[0].TransactionIndex)
			assert.Equal(t, uint32(2), txs[1].TransactionIndex)
		})
	})
}

func TestAccountTransactions_DescendingOrder(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
		account := unittest.RandomAddressFixture()

		// Index 10 blocks
		for height := uint64(2); height <= 11; height++ {
			err := storeAccountTransactions(t, lm, idx, height, []access.AccountTransaction{
				{
					Address:          account,
					BlockHeight:      height,
					TransactionID:    unittest.IdentifierFixture(),
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
				},
			})
			require.NoError(t, err)
		}

		iter, err := idx.ByAddress(account, nil)
		require.NoError(t, err)
		txs := collectAll(t, iter)
		require.Len(t, txs, 10)

		// Verify descending order
		for i := 0; i < len(txs)-1; i++ {
			assert.Greater(t, txs[i].BlockHeight, txs[i+1].BlockHeight,
				"results should be in descending order by height")
		}
	})
}

func TestAccountTransactions_MultiTxSameHeightOrdering(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
		account := unittest.RandomAddressFixture()

		txID0 := unittest.IdentifierFixture()
		txID1 := unittest.IdentifierFixture()
		txID2 := unittest.IdentifierFixture()

		// Store 3 txs for the same account at the same height with different txIndex values
		err := storeAccountTransactions(t, lm, idx, 2, []access.AccountTransaction{
			{
				Address:          account,
				BlockHeight:      2,
				TransactionID:    txID0,
				TransactionIndex: 0,
				Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
			},
			{
				Address:          account,
				BlockHeight:      2,
				TransactionID:    txID1,
				TransactionIndex: 1,
				Roles:            []access.TransactionRole{access.TransactionRolePayer},
			},
			{
				Address:          account,
				BlockHeight:      2,
				TransactionID:    txID2,
				TransactionIndex: 2,
				Roles:            []access.TransactionRole{access.TransactionRoleInteracted},
			},
		})
		require.NoError(t, err)

		iter, err := idx.ByAddress(account, nil)
		require.NoError(t, err)
		txs := collectAll(t, iter)
		require.Len(t, txs, 3)

		// All at same height, should be ordered by ascending txIndex
		assert.Equal(t, txID0, txs[0].TransactionID)
		assert.Equal(t, uint32(0), txs[0].TransactionIndex)
		assert.Equal(t, txID1, txs[1].TransactionID)
		assert.Equal(t, uint32(1), txs[1].TransactionIndex)
		assert.Equal(t, txID2, txs[2].TransactionID)
		assert.Equal(t, uint32(2), txs[2].TransactionIndex)
	})
}

func TestAccountTransactions_ErrorCases(t *testing.T) {
	t.Parallel()

	t.Run("cursor before first indexed height returns error", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 5, nil, func(_ storage.DB, _ storage.LockManager, idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()

			cursor := &access.AccountTransactionCursor{BlockHeight: 3, TransactionIndex: 0}
			_, err := idx.ByAddress(account, cursor)
			require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
		})
	})

	t.Run("cursor after latest indexed height returns error", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 5, nil, func(_ storage.DB, _ storage.LockManager, idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()

			cursor := &access.AccountTransactionCursor{BlockHeight: 100, TransactionIndex: 0}
			_, err := idx.ByAddress(account, cursor)
			require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
		})
	})

	t.Run("nil cursor returns empty iterator for account with no transactions", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 5, nil, func(_ storage.DB, _ storage.LockManager, idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()

			iter, err := idx.ByAddress(account, nil)
			require.NoError(t, err)
			txs := collectAll(t, iter)
			assert.Empty(t, txs)
		})
	})

	t.Run("store non-consecutive height fails", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
			// Try to index height 5 when latest is 1
			err := storeAccountTransactions(t, lm, idx, 5, nil)
			require.Error(t, err)
			assert.False(t, errors.Is(err, storage.ErrAlreadyExists),
				"non-consecutive height should not return ErrAlreadyExists")
		})
	})

	t.Run("store below latest returns ErrAlreadyExists", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
			err := storeAccountTransactions(t, lm, idx, 2, nil)
			require.NoError(t, err)
			err = storeAccountTransactions(t, lm, idx, 3, nil)
			require.NoError(t, err)

			// Try to store height 1 (below latest=3)
			err = storeAccountTransactions(t, lm, idx, 1, nil)
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})

	t.Run("duplicate key in committed DB returns corruption error", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(db storage.DB, lm storage.LockManager, idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()
			txID := unittest.IdentifierFixture()
			txData := []access.AccountTransaction{
				{
					Address:          account,
					BlockHeight:      2,
					TransactionID:    txID,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
				},
			}

			// Store txData at height 2
			err := storeAccountTransactions(t, lm, idx, 2, txData)
			require.NoError(t, err)

			// Simulate a partial write scenario: roll back the latest height marker
			// in the DB from 2 to 1, as if the tx keys were committed but the
			// height marker update was lost (e.g. crash between writes).
			// Note: this should not be possible given the storage logic.
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertByKey(rw.Writer(), keyAccountTransactionLatestHeightKey, uint64(1))
			})
			require.NoError(t, err)

			// Now Store at height 2 detects (via in-memory height) that height 2 is already indexed.
			err = storeAccountTransactions(t, lm, idx, 2, txData)
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})

	t.Run("block height mismatch in entry fails", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()
			txID := unittest.IdentifierFixture()

			// Entry claims height 5 but we're indexing height 2
			err := storeAccountTransactions(t, lm, idx, 2, []access.AccountTransaction{
				{
					Address:          account,
					BlockHeight:      5, // mismatch
					TransactionID:    txID,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
				},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "block height mismatch")
		})
	})

	t.Run("repeated store at latest height returns ErrAlreadyExists", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
			// Index height 2
			err := storeAccountTransactions(t, lm, idx, 2, nil)
			require.NoError(t, err)

			// Re-indexing height 2 should return ErrAlreadyExists
			err = storeAccountTransactions(t, lm, idx, 2, nil)
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})
}

func TestAccountTransactions_KeyEncoding(t *testing.T) {
	t.Parallel()

	t.Run("key encoding and decoding roundtrip", func(t *testing.T) {
		address := unittest.RandomAddressFixture()
		height := uint64(12345)
		txIndex := uint32(42)

		key := makeAccountTxKey(address, height, txIndex)

		cursor, err := decodeAccountTxKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, cursor.Address)
		assert.Equal(t, height, cursor.BlockHeight)
		assert.Equal(t, txIndex, cursor.TransactionIndex)
	})

	t.Run("makeAccountTxValue sorts roles", func(t *testing.T) {
		txID := unittest.IdentifierFixture()
		entry := access.AccountTransaction{
			TransactionID: txID,
			// Roles deliberately out of order
			Roles: []access.TransactionRole{
				access.TransactionRoleInteracted, // 3
				access.TransactionRoleAuthorizer, // 0
				access.TransactionRolePayer,      // 1
			},
		}

		stored := makeAccountTxValue(entry)
		assert.Equal(t, txID, stored.TransactionID)
		assert.Equal(t, []access.TransactionRole{
			access.TransactionRoleAuthorizer, // 0
			access.TransactionRolePayer,      // 1
			access.TransactionRoleInteracted, // 3
		}, stored.Roles)
	})

	t.Run("boundary values: height 0, txIndex 0", func(t *testing.T) {
		address := unittest.RandomAddressFixture()
		height := uint64(0)
		txIndex := uint32(0)

		key := makeAccountTxKey(address, height, txIndex)
		cursor, err := decodeAccountTxKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, cursor.Address)
		assert.Equal(t, height, cursor.BlockHeight)
		assert.Equal(t, txIndex, cursor.TransactionIndex)
	})

	t.Run("boundary values: max height, max txIndex", func(t *testing.T) {
		address := unittest.RandomAddressFixture()
		height := uint64(math.MaxUint64)
		txIndex := uint32(math.MaxUint32)

		key := makeAccountTxKey(address, height, txIndex)
		cursor, err := decodeAccountTxKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, cursor.Address)
		assert.Equal(t, uint64(math.MaxUint64), cursor.BlockHeight)
		assert.Equal(t, uint32(math.MaxUint32), cursor.TransactionIndex)
	})

	t.Run("boundary values: zero address", func(t *testing.T) {
		address := flow.Address{}
		height := uint64(12345)
		txIndex := uint32(42)

		key := makeAccountTxKey(address, height, txIndex)
		cursor, err := decodeAccountTxKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, cursor.Address)
		assert.Equal(t, height, cursor.BlockHeight)
		assert.Equal(t, txIndex, cursor.TransactionIndex)
	})
}

func TestAccountTransactions_KeyDecoding_Errors(t *testing.T) {
	t.Parallel()

	t.Run("key too short", func(t *testing.T) {
		_, err := decodeAccountTxKey(make([]byte, 10))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid key length")
	})

	t.Run("key too long", func(t *testing.T) {
		_, err := decodeAccountTxKey(make([]byte, 25))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid key length")
	})

	t.Run("invalid prefix", func(t *testing.T) {
		key := make([]byte, accountTxKeyLen)
		key[0] = 0xFF // wrong prefix
		_, err := decodeAccountTxKey(key)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid prefix")
	})
}

func TestAccountTransactions_EmptyResults(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
		// Query for account that has no transactions
		account := unittest.RandomAddressFixture()

		// First index some data so we have indexed heights
		err := storeAccountTransactions(t, lm, idx, 2, nil)
		require.NoError(t, err)

		iter, err := idx.ByAddress(account, nil)
		require.NoError(t, err)
		txs := collectAll(t, iter)
		assert.Empty(t, txs)
	})
}

func TestAccountTransactions_RolesRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		roles         []access.TransactionRole
		expectedRoles []access.TransactionRole // expected after sorting and deduplication
	}{
		{
			name:          "single role - authorizer",
			roles:         []access.TransactionRole{access.TransactionRoleAuthorizer},
			expectedRoles: []access.TransactionRole{access.TransactionRoleAuthorizer},
		},
		{
			name:          "single role - payer",
			roles:         []access.TransactionRole{access.TransactionRolePayer},
			expectedRoles: []access.TransactionRole{access.TransactionRolePayer},
		},
		{
			name:          "single role - proposer",
			roles:         []access.TransactionRole{access.TransactionRoleProposer},
			expectedRoles: []access.TransactionRole{access.TransactionRoleProposer},
		},
		{
			name:          "single role - interaction",
			roles:         []access.TransactionRole{access.TransactionRoleInteracted},
			expectedRoles: []access.TransactionRole{access.TransactionRoleInteracted},
		},
		{
			name: "multiple roles - payer and authorizer",
			roles: []access.TransactionRole{
				access.TransactionRoleAuthorizer,
				access.TransactionRolePayer,
			},
			expectedRoles: []access.TransactionRole{
				access.TransactionRoleAuthorizer,
				access.TransactionRolePayer,
			},
		},
		{
			name: "all roles",
			roles: []access.TransactionRole{
				access.TransactionRoleAuthorizer,
				access.TransactionRolePayer,
				access.TransactionRoleProposer,
				access.TransactionRoleInteracted,
			},
			expectedRoles: []access.TransactionRole{
				access.TransactionRoleAuthorizer,
				access.TransactionRolePayer,
				access.TransactionRoleProposer,
				access.TransactionRoleInteracted,
			},
		},
		{
			name: "unsorted roles are stored sorted",
			roles: []access.TransactionRole{
				access.TransactionRoleInteracted,
				access.TransactionRoleAuthorizer,
				access.TransactionRolePayer,
			},
			expectedRoles: []access.TransactionRole{
				access.TransactionRoleAuthorizer,
				access.TransactionRolePayer,
				access.TransactionRoleInteracted,
			},
		},
		{
			name: "duplicate roles are deduplicated",
			roles: []access.TransactionRole{
				access.TransactionRoleInteracted,
				access.TransactionRoleInteracted,
				access.TransactionRoleInteracted,
			},
			expectedRoles: []access.TransactionRole{
				access.TransactionRoleInteracted,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
				account := unittest.RandomAddressFixture()
				txID := unittest.IdentifierFixture()

				// Copy the input roles to detect mutation
				inputRoles := make([]access.TransactionRole, len(tt.roles))
				copy(inputRoles, tt.roles)

				txData := []access.AccountTransaction{
					{
						Address:          account,
						BlockHeight:      2,
						TransactionID:    txID,
						TransactionIndex: 0,
						Roles:            inputRoles,
					},
				}

				err := storeAccountTransactions(t, lm, idx, 2, txData)
				require.NoError(t, err)

				iter, err := idx.ByAddress(account, nil)
				require.NoError(t, err)
				txs := collectAll(t, iter)
				require.Len(t, txs, 1)
				assert.Equal(t, txID, txs[0].TransactionID)
				assert.Equal(t, tt.expectedRoles, txs[0].Roles)
			})
		})
	}
}

func TestAccountTransactions_LockRequirement(t *testing.T) {
	t.Parallel()

	t.Run("Store without lock returns error", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(db storage.DB, lm storage.LockManager, idx *AccountTransactions) {
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

	t.Run("initialize without lock returns error", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			lctx := lm.NewContext()
			defer lctx.Release()

			// Call without acquiring the required lock
			err := storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapAccountTransactions(lctx, rw, storageDB, 1, nil)
				return bootstrapErr
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing required lock")
		})
	})
}

func TestAccountTransactions_BootstrapHeightMismatch(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		lm := storage.NewTestingLockManager()

		account := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()

		// Entry claims height 99 but we're bootstrapping at height 5
		err := unittest.WithLock(t, lm, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapAccountTransactions(lctx, rw, storageDB, 5, []access.AccountTransaction{
					{
						Address:          account,
						BlockHeight:      99, // mismatch with bootstrap height 5
						TransactionID:    txID,
						TransactionIndex: 0,
						Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
					},
				})
				return bootstrapErr
			})
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "block height mismatch")
	})
}

func storeAccountTransactions(tb testing.TB, lockManager storage.LockManager, idx *AccountTransactions, height uint64, txData []access.AccountTransaction) error {
	return unittest.WithLock(tb, lockManager, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
		return idx.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return idx.Store(lctx, rw, height, txData)
		})
	})
}

// RunWithBootstrappedAccountTxIndex creates a new Pebble database and bootstraps it
// for account transaction indexing at the given start height. The callback receives a shared
// lock manager that should be passed to storeAccountTransactions for consistent lock usage.
func RunWithBootstrappedAccountTxIndex(tb testing.TB, startHeight uint64, txData []access.AccountTransaction, f func(db storage.DB, lockManager storage.LockManager, idx *AccountTransactions)) {
	unittest.RunWithPebbleDB(tb, func(db *pebble.DB) {
		lockManager := storage.NewTestingLockManager()

		var accountTx *AccountTransactions
		storageDB := pebbleimpl.ToDB(db)
		err := unittest.WithLock(tb, lockManager, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				var bootstrapErr error
				accountTx, bootstrapErr = BootstrapAccountTransactions(lctx, rw, storageDB, startHeight, txData)
				return bootstrapErr
			})
		})
		require.NoError(tb, err)

		f(storageDB, lockManager, accountTx)
	})
}

func TestAccountTransactions_UncommittedBatch(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedAccountTxIndex(t, 1, nil, func(db storage.DB, lm storage.LockManager, idx *AccountTransactions) {
		require.Equal(t, uint64(1), idx.LatestIndexedHeight())

		account := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()
		txData := []access.AccountTransaction{
			{
				Address:          account,
				BlockHeight:      2,
				TransactionID:    txID,
				TransactionIndex: 0,
				Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
			},
		}

		// Create a batch manually and store data without committing.
		// Store registers an OnCommitSucceed callback to update latestHeight,
		// which should only fire when the batch is committed.
		batch := db.NewBatch()
		err := unittest.WithLock(t, lm, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
			return idx.Store(lctx, batch, 2, txData)
		})
		require.NoError(t, err)

		// Close the batch without committing - discards pending writes
		require.NoError(t, batch.Close())

		// latestHeight must still be 1 since the batch was never committed
		assert.Equal(t, uint64(1), idx.LatestIndexedHeight(),
			"latestHeight should not update when the batch is not committed")
	})
}
