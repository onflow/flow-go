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

			results, err := idx.TransactionsByAddress(initialData[0].Address, 1, 1)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, initialData[0].BlockHeight, results[0].BlockHeight)
			assert.Equal(t, initialData[0].TransactionID, results[0].TransactionID)
			assert.Equal(t, initialData[0].TransactionIndex, results[0].TransactionIndex)
			assert.Equal(t, initialData[0].Roles, results[0].Roles)
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

			results, err := idx.TransactionsByAddress(account, 0, 1)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, txID, results[0].TransactionID)
		})
	})
}

func TestAccountTransactions_IndexAndQuery(t *testing.T) {
	t.Parallel()

	t.Run("index single block with single transaction", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
			// Create test data
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

			// Index block at height 2
			err := storeAccountTransactions(t, lm, idx, 2, txData)
			require.NoError(t, err)

			// Query should return the transaction
			results, err := idx.TransactionsByAddress(account1, 2, 2)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, txID, results[0].TransactionID)
			assert.Equal(t, uint64(2), results[0].BlockHeight)
			assert.Equal(t, uint32(0), results[0].TransactionIndex)
			assert.Equal(t, []access.TransactionRole{access.TransactionRoleAuthorizer}, results[0].Roles)
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
				// tx2 involves both account1 and account2
				{
					Address:          account1,
					BlockHeight:      3,
					TransactionID:    txID2,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleInteraction},
				},
				{
					Address:          account2,
					BlockHeight:      3,
					TransactionID:    txID2,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
				},
				// tx3 involves only account2
				{
					Address:          account2,
					BlockHeight:      3,
					TransactionID:    txID3,
					TransactionIndex: 1,
					Roles:            []access.TransactionRole{access.TransactionRoleInteraction},
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
			assert.Equal(t, []access.TransactionRole{access.TransactionRoleInteraction}, results[0].Roles)

			assert.Equal(t, txID1, results[1].TransactionID)
			assert.Equal(t, uint64(2), results[1].BlockHeight)
			assert.Equal(t, []access.TransactionRole{access.TransactionRoleAuthorizer}, results[1].Roles)

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
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()

			// Index blocks 2, 3, 4
			for height := uint64(2); height <= 4; height++ {
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

			// Query only blocks 2-3
			results, err := idx.TransactionsByAddress(account, 2, 3)
			require.NoError(t, err)
			require.Len(t, results, 2)

			// All results should be in the queried range
			for _, r := range results {
				assert.GreaterOrEqual(t, r.BlockHeight, uint64(2))
				assert.LessOrEqual(t, r.BlockHeight, uint64(3))
			}
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

		// Query all
		results, err := idx.TransactionsByAddress(account, 2, 11)
		require.NoError(t, err)
		require.Len(t, results, 10)

		// iterate over all results, and verify they were provided in descending order by height
		for i := range len(results) - 1 {
			assert.Greater(t, results[i].BlockHeight, results[i+1].BlockHeight,
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
				Roles:            []access.TransactionRole{access.TransactionRoleInteraction},
			},
		})
		require.NoError(t, err)

		results, err := idx.TransactionsByAddress(account, 2, 2)
		require.NoError(t, err)
		require.Len(t, results, 3)

		// All at same height, should be ordered by ascending txIndex
		assert.Equal(t, txID0, results[0].TransactionID)
		assert.Equal(t, uint32(0), results[0].TransactionIndex)
		assert.Equal(t, txID1, results[1].TransactionID)
		assert.Equal(t, uint32(1), results[1].TransactionIndex)
		assert.Equal(t, txID2, results[2].TransactionID)
		assert.Equal(t, uint32(2), results[2].TransactionIndex)
	})
}

func TestAccountTransactions_ErrorCases(t *testing.T) {
	t.Parallel()

	t.Run("query beyond indexed range", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 5, nil, func(_ storage.DB, _ storage.LockManager, idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()

			// Query before first height returns error
			_, err := idx.TransactionsByAddress(account, 3, 5)
			require.ErrorIs(t, err, storage.ErrHeightNotIndexed)

			// Query with startHeight above latestHeight returns ErrHeightNotIndexed
			_, err = idx.TransactionsByAddress(account, 10, 15)
			require.ErrorIs(t, err, storage.ErrHeightNotIndexed)

			// Query after latest height clamps endHeight to latest (returns empty since no data)
			results, err := idx.TransactionsByAddress(account, 5, 10)
			require.NoError(t, err)
			assert.Empty(t, results)
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
			// Index height 2 and 3
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

			// Now indexAccountTransactions at height 2 passes the consecutive check
			// (2 == 1+1) but finds the already-committed keys.
			err = unittest.WithLock(t, lm, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return indexAccountTransactions(lctx, rw, 2, txData)
				})
			})
			// an error should be returned, but it should not be a generic error and not storage.ErrAlreadyExists
			require.Error(t, err)
			assert.False(t, errors.Is(err, storage.ErrAlreadyExists))
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
					BlockHeight:      5, // mismatch with height 2
					TransactionID:    txID,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
				},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "block height mismatch")
		})
	})

	t.Run("repeated store at latest height is a no-op", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
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

			// Index height 2 with actual data
			err := storeAccountTransactions(t, lm, idx, 2, txData)
			require.NoError(t, err)

			// Re-indexing height 2 with different data should be a no-op (original data retained)
			differentTxID := unittest.IdentifierFixture()
			differentTxData := []access.AccountTransaction{
				{
					Address:          account,
					BlockHeight:      2,
					TransactionID:    differentTxID,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRolePayer},
				},
			}
			err = storeAccountTransactions(t, lm, idx, 2, differentTxData)
			require.NoError(t, err)

			// Verify original data is retained, not replaced
			results, err := idx.TransactionsByAddress(account, 2, 2)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, txID, results[0].TransactionID)
			assert.Equal(t, []access.TransactionRole{access.TransactionRoleAuthorizer}, results[0].Roles)
		})
	})

	t.Run("start height greater than end height", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *AccountTransactions) {
			account := unittest.RandomAddressFixture()

			// Store blocks up to height 6 so both 5 and 3 are within indexed range
			for h := uint64(2); h <= 6; h++ {
				err := storeAccountTransactions(t, lm, idx, h, nil)
				require.NoError(t, err)
			}

			// Query with startHeight(5) > endHeight(3), both within indexed range
			_, err := idx.TransactionsByAddress(account, 5, 3)
			require.ErrorIs(t, err, storage.ErrInvalidQuery)
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

		decodedAddress, decodedHeight, decodedTxIndex, err := decodeAccountTxKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, decodedAddress)
		assert.Equal(t, height, decodedHeight)
		assert.Equal(t, txIndex, decodedTxIndex)
	})

	t.Run("makeAccountTxValue sorts roles", func(t *testing.T) {
		txID := unittest.IdentifierFixture()
		entry := access.AccountTransaction{
			TransactionID: txID,
			// Roles deliberately out of order
			Roles: []access.TransactionRole{
				access.TransactionRoleInteraction, // 3
				access.TransactionRoleAuthorizer,  // 0
				access.TransactionRolePayer,       // 1
			},
		}

		stored := makeAccountTxValue(entry)
		assert.Equal(t, txID, stored.TransactionID)
		assert.Equal(t, []access.TransactionRole{
			access.TransactionRoleAuthorizer,  // 0
			access.TransactionRolePayer,       // 1
			access.TransactionRoleInteraction, // 3
		}, stored.Roles)
	})

	t.Run("boundary values: height 0, txIndex 0", func(t *testing.T) {
		address := unittest.RandomAddressFixture()
		height := uint64(0)
		txIndex := uint32(0)

		key := makeAccountTxKey(address, height, txIndex)
		decodedAddress, decodedHeight, decodedTxIndex, err := decodeAccountTxKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, decodedAddress)
		assert.Equal(t, height, decodedHeight)
		assert.Equal(t, txIndex, decodedTxIndex)
	})

	t.Run("boundary values: max height, max txIndex", func(t *testing.T) {
		address := unittest.RandomAddressFixture()
		height := uint64(math.MaxUint64)
		txIndex := uint32(math.MaxUint32)

		key := makeAccountTxKey(address, height, txIndex)
		decodedAddress, decodedHeight, decodedTxIndex, err := decodeAccountTxKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, decodedAddress)
		assert.Equal(t, height, decodedHeight)
		assert.Equal(t, txIndex, decodedTxIndex)
	})

	t.Run("boundary values: zero address", func(t *testing.T) {
		address := flow.Address{}
		height := uint64(12345)
		txIndex := uint32(42)

		key := makeAccountTxKey(address, height, txIndex)
		decodedAddress, decodedHeight, decodedTxIndex, err := decodeAccountTxKey(key)
		require.NoError(t, err)
		assert.Equal(t, address, decodedAddress)
		assert.Equal(t, height, decodedHeight)
		assert.Equal(t, txIndex, decodedTxIndex)
	})
}

func TestAccountTransactions_KeyDecoding_Errors(t *testing.T) {
	t.Parallel()

	t.Run("key too short", func(t *testing.T) {
		_, _, _, err := decodeAccountTxKey(make([]byte, 10))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid key length")
	})

	t.Run("key too long", func(t *testing.T) {
		_, _, _, err := decodeAccountTxKey(make([]byte, 25))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid key length")
	})

	t.Run("invalid prefix", func(t *testing.T) {
		key := make([]byte, accountTxKeyLen)
		key[0] = 0xFF // wrong prefix
		_, _, _, err := decodeAccountTxKey(key)
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

		results, err := idx.TransactionsByAddress(account, 1, 2)
		require.NoError(t, err)
		assert.Empty(t, results)
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
			roles:         []access.TransactionRole{access.TransactionRoleInteraction},
			expectedRoles: []access.TransactionRole{access.TransactionRoleInteraction},
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
				access.TransactionRoleInteraction,
			},
			expectedRoles: []access.TransactionRole{
				access.TransactionRoleAuthorizer,
				access.TransactionRolePayer,
				access.TransactionRoleProposer,
				access.TransactionRoleInteraction,
			},
		},
		{
			name: "unsorted roles are stored sorted",
			roles: []access.TransactionRole{
				access.TransactionRoleInteraction,
				access.TransactionRoleAuthorizer,
				access.TransactionRolePayer,
			},
			expectedRoles: []access.TransactionRole{
				access.TransactionRoleAuthorizer,
				access.TransactionRolePayer,
				access.TransactionRoleInteraction,
			},
		},
		{
			name: "duplicate roles are deduplicated",
			roles: []access.TransactionRole{
				access.TransactionRoleInteraction,
				access.TransactionRoleInteraction,
				access.TransactionRoleInteraction,
			},
			expectedRoles: []access.TransactionRole{
				access.TransactionRoleInteraction,
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

				results, err := idx.TransactionsByAddress(account, 2, 2)
				require.NoError(t, err)
				require.Len(t, results, 1)
				assert.Equal(t, txID, results[0].TransactionID)
				assert.Equal(t, tt.expectedRoles, results[0].Roles)
			})
		})
	}
}

func TestAccountTransactions_LockRequirement(t *testing.T) {
	t.Parallel()

	t.Run("indexAccountTransactions without lock returns error", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 1, nil, func(db storage.DB, lm storage.LockManager, _ *AccountTransactions) {
			lctx := lm.NewContext()
			defer lctx.Release()

			// Call without acquiring the required lock
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return indexAccountTransactions(lctx, rw, 2, nil)
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
