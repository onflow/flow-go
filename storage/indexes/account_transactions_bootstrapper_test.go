package indexes

import (
	"errors"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBootstrapper_Constructor(t *testing.T) {
	t.Parallel()

	t.Run("uninitialized DB returns ErrNotBootstrapped", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			store, err := NewAccountTransactionsBootstrapper(storageDB, 10)
			require.NoError(t, err)

			// Inner store should be nil, so height methods return ErrNotBootstrapped
			_, err = store.FirstIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})
	})

	t.Run("already-bootstrapped DB is immediately usable", func(t *testing.T) {
		RunWithBootstrappedAccountTxIndex(t, 5, nil, func(db storage.DB, _ storage.LockManager, _ *AccountTransactions) {
			store, err := NewAccountTransactionsBootstrapper(db, 5)
			require.NoError(t, err)

			// Inner store should be loaded, so height methods work
			first, err := store.FirstIndexedHeight()
			require.NoError(t, err)
			assert.Equal(t, uint64(5), first)
		})
	})
}

func TestBootstrapper_PreBootstrapState(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := NewAccountTransactionsBootstrapper(storageDB, 42)
		require.NoError(t, err)

		t.Run("FirstIndexedHeight returns zero with ErrNotBootstrapped", func(t *testing.T) {
			height, err := store.FirstIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
			assert.Equal(t, uint64(0), height)
		})

		t.Run("LatestIndexedHeight returns zero with ErrNotBootstrapped", func(t *testing.T) {
			height, err := store.LatestIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
			assert.Equal(t, uint64(0), height)
		})

		t.Run("UninitializedFirstHeight returns initialStartHeight and false", func(t *testing.T) {
			height, initialized := store.UninitializedFirstHeight()
			assert.Equal(t, uint64(42), height)
			assert.False(t, initialized)
		})

		t.Run("TransactionsByAddress returns ErrNotBootstrapped", func(t *testing.T) {
			_, err := store.TransactionsByAddress(unittest.RandomAddressFixture(), 42, 42)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})
	})
}

func TestBootstrapper_StoreTriggersBootstrap(t *testing.T) {
	t.Parallel()

	t.Run("Store at initialStartHeight bootstraps the index", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			store, err := NewAccountTransactionsBootstrapper(storageDB, 10)
			require.NoError(t, err)

			err = storeBootstrapperTx(t, store, storageDB, 10, nil)
			require.NoError(t, err)

			// After bootstrap, height methods should work
			first, err := store.FirstIndexedHeight()
			require.NoError(t, err)
			assert.Equal(t, uint64(10), first)

			latest, err := store.LatestIndexedHeight()
			require.NoError(t, err)
			assert.Equal(t, uint64(10), latest)

			height, initialized := store.UninitializedFirstHeight()
			assert.Equal(t, uint64(10), height)
			assert.True(t, initialized)
		})
	})

	t.Run("Store at wrong height returns ErrNotBootstrapped", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			store, err := NewAccountTransactionsBootstrapper(storageDB, 10)
			require.NoError(t, err)

			// Store at height 11 (not the initialStartHeight of 10)
			err = storeBootstrapperTx(t, store, storageDB, 11, nil)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)

			// Store at height 9 (below initialStartHeight)
			err = storeBootstrapperTx(t, store, storageDB, 9, nil)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})
	})
}

func TestBootstrapper_BootstrapWithData(t *testing.T) {
	t.Parallel()

	t.Run("bootstrap with transaction data persists and is queryable", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)

			firstHeight := uint64(5)
			store, err := NewAccountTransactionsBootstrapper(storageDB, firstHeight)
			require.NoError(t, err)

			account := unittest.RandomAddressFixture()
			txID := unittest.IdentifierFixture()

			txData := []access.AccountTransaction{
				{
					Address:          account,
					BlockHeight:      firstHeight,
					TransactionID:    txID,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
				},
			}

			// Bootstrap with data
			err = storeBootstrapperTx(t, store, storageDB, firstHeight, txData)
			require.NoError(t, err)

			// Data should be queryable
			results, err := store.TransactionsByAddress(account, firstHeight, firstHeight)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, txID, results[0].TransactionID)
			assert.Equal(t, uint64(5), results[0].BlockHeight)
			assert.Equal(t, uint32(0), results[0].TransactionIndex)
			assert.Equal(t, []access.TransactionRole{access.TransactionRoleAuthorizer}, results[0].Roles)
		})
	})

	t.Run("subsequent stores work after bootstrap", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			store, err := NewAccountTransactionsBootstrapper(storageDB, 1)
			require.NoError(t, err)

			account := unittest.RandomAddressFixture()
			txID1 := unittest.IdentifierFixture()
			txID2 := unittest.IdentifierFixture()

			// Bootstrap at height 1 with first tx
			err = storeBootstrapperTx(t, store, storageDB, 1, []access.AccountTransaction{
				{
					Address:          account,
					BlockHeight:      1,
					TransactionID:    txID1,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
				},
			})
			require.NoError(t, err)

			// Store at height 2
			err = storeBootstrapperTx(t, store, storageDB, 2, []access.AccountTransaction{
				{
					Address:          account,
					BlockHeight:      2,
					TransactionID:    txID2,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleInteraction},
				},
			})
			require.NoError(t, err)

			// Query both heights
			results, err := store.TransactionsByAddress(account, 1, 2)
			require.NoError(t, err)
			require.Len(t, results, 2)

			// Descending order: height 2 first, then height 1
			assert.Equal(t, txID2, results[0].TransactionID)
			assert.Equal(t, uint64(2), results[0].BlockHeight)
			assert.Equal(t, []access.TransactionRole{access.TransactionRoleInteraction}, results[0].Roles)

			assert.Equal(t, txID1, results[1].TransactionID)
			assert.Equal(t, uint64(1), results[1].BlockHeight)
			assert.Equal(t, []access.TransactionRole{access.TransactionRoleAuthorizer}, results[1].Roles)

			// Latest height should reflect both stores
			latest, err := store.LatestIndexedHeight()
			require.NoError(t, err)
			assert.Equal(t, uint64(2), latest)
		})
	})
}

func TestBootstrapper_NonConsecutiveStoreAfterBootstrap(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := NewAccountTransactionsBootstrapper(storageDB, 5)
		require.NoError(t, err)

		// Bootstrap at height 5
		err = storeBootstrapperTx(t, store, storageDB, 5, nil)
		require.NoError(t, err)

		// Attempt to store at height 7, skipping height 6
		err = storeBootstrapperTx(t, store, storageDB, 7, nil)
		require.Error(t, err)
		assert.False(t, errors.Is(err, storage.ErrAlreadyExists))
		assert.False(t, errors.Is(err, storage.ErrNotBootstrapped))
	})
}

func TestBootstrapper_PersistenceAcrossRestart(t *testing.T) {
	t.Parallel()

	unittest.RunWithTempDir(t, func(dir string) {
		account := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()

		// Phase 1: bootstrap via the bootstrapper
		func() {
			db := openPebbleDB(t, dir)
			defer db.Close()

			store, err := NewAccountTransactionsBootstrapper(db, 100)
			require.NoError(t, err)

			// Should be uninitialized
			_, err = store.FirstIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)

			err = storeBootstrapperTx(t, store, db, 100, []access.AccountTransaction{
				{
					Address:          account,
					BlockHeight:      100,
					TransactionID:    txID,
					TransactionIndex: 0,
					Roles:            []access.TransactionRole{access.TransactionRoleAuthorizer},
				},
			})
			require.NoError(t, err)
		}()

		// Phase 2: reopen DB and verify bootstrapper returns concrete store
		db := openPebbleDB(t, dir)
		defer db.Close()

		store, err := NewAccountTransactionsBootstrapper(db, 100)
		require.NoError(t, err)

		// Data should be persisted
		first, err := store.FirstIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(100), first)

		results, err := store.TransactionsByAddress(account, 100, 100)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, txID, results[0].TransactionID)
	})
}

func TestBootstrapper_DoubleBootstrapProtection(t *testing.T) {
	t.Parallel()

	lockManager := storage.NewTestingLockManager()
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		err := unittest.WithLock(t, lockManager, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapAccountTransactions(lctx, rw, storageDB, 1, nil)
				return bootstrapErr
			})
		})
		require.NoError(t, err)

		// Attempting to bootstrap again via initialize should fail
		err = unittest.WithLock(t, lockManager, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapAccountTransactions(lctx, rw, storageDB, 1, nil)
				return bootstrapErr
			})
		})
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
	})
}

// storeBootstrapperTx is a test helper that stores account transactions through the
// storage.AccountTransactions interface. Unlike storeAccountTransactions which uses the
// concrete type's db field, this helper accepts the DB explicitly.
func storeBootstrapperTx(
	tb testing.TB,
	store storage.AccountTransactionsBootstrapper,
	db storage.DB,
	height uint64,
	txData []access.AccountTransaction,
) error {
	tb.Helper()
	lockManager := storage.NewTestingLockManager()
	return unittest.WithLock(tb, lockManager, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
		return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return store.Store(lctx, rw, height, txData)
		})
	})
}

// openPebbleDB opens a Pebble database at the given directory and wraps it as a storage.DB.
func openPebbleDB(tb testing.TB, dir string) storage.DB {
	tb.Helper()
	pdb, err := pebble.Open(dir, &pebble.Options{})
	require.NoError(tb, err)
	return pebbleimpl.ToDB(pdb)
}
