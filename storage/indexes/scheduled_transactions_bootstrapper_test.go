package indexes_test

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes"
	"github.com/onflow/flow-go/storage/indexes/iterator"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

// storeBootstrapperScheduledTx is a helper that calls Store on the bootstrapper under the required lock.
func storeBootstrapperScheduledTx(
	tb testing.TB,
	store storage.ScheduledTransactionsIndexBootstrapper,
	db storage.DB,
	height uint64,
	txs []access.ScheduledTransaction,
) error {
	tb.Helper()
	lockManager := storage.NewTestingLockManager()
	return unittest.WithLock(tb, lockManager, storage.LockIndexScheduledTransactionsIndex, func(lctx lockctx.Context) error {
		return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return store.Store(lctx, rw, height, txs)
		})
	})
}

// openPebbleScheduledTxDB opens a pebble DB at dir for use in persistence tests.
func openPebbleScheduledTxDB(tb testing.TB, dir string) storage.DB {
	tb.Helper()
	pdb, err := pebble.Open(dir, &pebble.Options{})
	require.NoError(tb, err)
	return pebbleimpl.ToDB(pdb)
}

// collectBootstrapperAll collects results from the bootstrapper All method.
func collectBootstrapperAll(tb testing.TB, store storage.ScheduledTransactionsIndexBootstrapper, limit uint32, cursor *access.ScheduledTransactionCursor) ([]access.ScheduledTransaction, *access.ScheduledTransactionCursor) {
	tb.Helper()
	iter, err := store.All(cursor)
	require.NoError(tb, err)
	collected, nextCursor, err := iterator.CollectResults(iter, limit, nil)
	require.NoError(tb, err)
	return collected, nextCursor
}

// collectBootstrapperByAddress collects results from the bootstrapper ByAddress method.
func collectBootstrapperByAddress(tb testing.TB, store storage.ScheduledTransactionsIndexBootstrapper, addr access.ScheduledTransaction, limit uint32, cursor *access.ScheduledTransactionCursor) ([]access.ScheduledTransaction, *access.ScheduledTransactionCursor) {
	tb.Helper()
	iter, err := store.ByAddress(addr.TransactionHandlerOwner, cursor)
	require.NoError(tb, err)
	collected, nextCursor, err := iterator.CollectResults(iter, limit, nil)
	require.NoError(tb, err)
	return collected, nextCursor
}

func TestScheduledTransactionsBootstrapper_Uninitialized_Reads(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := indexes.NewScheduledTransactionsBootstrapper(storageDB, 10)
		require.NoError(t, err)

		_, err = store.ByID(1)
		require.ErrorIs(t, err, storage.ErrNotBootstrapped)

		_, err = store.ByAddress(unittest.RandomAddressFixture(), nil)
		require.ErrorIs(t, err, storage.ErrNotBootstrapped)

		_, err = store.All(nil)
		require.ErrorIs(t, err, storage.ErrNotBootstrapped)
	})
}

func TestScheduledTransactionsBootstrapper_FirstStore_WrongHeight(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := indexes.NewScheduledTransactionsBootstrapper(storageDB, 10)
		require.NoError(t, err)

		err = storeBootstrapperScheduledTx(t, store, storageDB, 11, nil)
		require.ErrorIs(t, err, storage.ErrNotBootstrapped)

		err = storeBootstrapperScheduledTx(t, store, storageDB, 9, nil)
		require.ErrorIs(t, err, storage.ErrNotBootstrapped)
	})
}

func TestScheduledTransactionsBootstrapper_FirstStore_BootstrapsAndSucceeds(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := indexes.NewScheduledTransactionsBootstrapper(storageDB, 10)
		require.NoError(t, err)

		addr := unittest.RandomAddressFixture()
		txs := []access.ScheduledTransaction{makeScheduledTx(1, addr)}

		err = storeBootstrapperScheduledTx(t, store, storageDB, 10, txs)
		require.NoError(t, err)

		// reads should work after bootstrap
		got, err := store.ByID(1)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), got.ID)

		byAddr, _ := collectBootstrapperByAddress(t, store, txs[0], 10, nil)
		require.Len(t, byAddr, 1)
		assert.Equal(t, uint64(1), byAddr[0].ID)

		all, _ := collectBootstrapperAll(t, store, 10, nil)
		require.Len(t, all, 1)
	})
}

func TestScheduledTransactionsBootstrapper_SecondStore_Succeeds(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := indexes.NewScheduledTransactionsBootstrapper(storageDB, 5)
		require.NoError(t, err)

		addr := unittest.RandomAddressFixture()
		tx1 := makeScheduledTx(1, addr)
		tx2 := makeScheduledTx(2, addr)

		err = storeBootstrapperScheduledTx(t, store, storageDB, 5, []access.ScheduledTransaction{tx1})
		require.NoError(t, err)

		err = storeBootstrapperScheduledTx(t, store, storageDB, 6, []access.ScheduledTransaction{tx2})
		require.NoError(t, err)

		byAddr, _ := collectBootstrapperByAddress(t, store, tx1, 10, nil)
		require.Len(t, byAddr, 2)

		latest, err := store.LatestIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(6), latest)
	})
}

func TestScheduledTransactionsBootstrapper_UninitializedFirstHeight(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := indexes.NewScheduledTransactionsBootstrapper(storageDB, 42)
		require.NoError(t, err)

		height, initialized := store.UninitializedFirstHeight()
		assert.Equal(t, uint64(42), height)
		assert.False(t, initialized)

		err = storeBootstrapperScheduledTx(t, store, storageDB, 42, nil)
		require.NoError(t, err)

		height, initialized = store.UninitializedFirstHeight()
		assert.Equal(t, uint64(42), height)
		assert.True(t, initialized)
	})
}

func TestScheduledTransactionsBootstrapper_HeightMethods_Uninitialized(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := indexes.NewScheduledTransactionsBootstrapper(storageDB, 42)
		require.NoError(t, err)

		height, err := store.FirstIndexedHeight()
		require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		assert.Equal(t, uint64(0), height)

		height, err = store.LatestIndexedHeight()
		require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		assert.Equal(t, uint64(0), height)
	})
}

func TestScheduledTransactionsBootstrapper_HeightMethods_Initialized(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := indexes.NewScheduledTransactionsBootstrapper(storageDB, 7)
		require.NoError(t, err)

		err = storeBootstrapperScheduledTx(t, store, storageDB, 7, nil)
		require.NoError(t, err)

		first, err := store.FirstIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(7), first)

		latest, err := store.LatestIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(7), latest)
	})
}

func TestScheduledTransactionsBootstrapper_AlreadyBootstrapped(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedScheduledTxIndex(t, 5, func(db storage.DB, _ storage.LockManager, _ *indexes.ScheduledTransactionsIndex) {
		store, err := indexes.NewScheduledTransactionsBootstrapper(db, 5)
		require.NoError(t, err)

		first, err := store.FirstIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(5), first)
	})
}

func TestScheduledTransactionsBootstrapper_DoubleBootstrapProtection(t *testing.T) {
	t.Parallel()

	lockManager := storage.NewTestingLockManager()
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)

		err := unittest.WithLock(t, lockManager, storage.LockIndexScheduledTransactionsIndex, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := indexes.BootstrapScheduledTransactions(lctx, rw, storageDB, 1, nil)
				return bootstrapErr
			})
		})
		require.NoError(t, err)

		err = unittest.WithLock(t, lockManager, storage.LockIndexScheduledTransactionsIndex, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := indexes.BootstrapScheduledTransactions(lctx, rw, storageDB, 1, nil)
				return bootstrapErr
			})
		})
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
	})
}

func TestScheduledTransactionsBootstrapper_PersistenceAcrossRestart(t *testing.T) {
	t.Parallel()

	unittest.RunWithTempDir(t, func(dir string) {
		addr := unittest.RandomAddressFixture()
		tx := makeScheduledTx(99, addr)

		func() {
			db := openPebbleScheduledTxDB(t, dir)
			defer db.Close()

			store, err := indexes.NewScheduledTransactionsBootstrapper(db, 100)
			require.NoError(t, err)

			_, err = store.FirstIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)

			err = storeBootstrapperScheduledTx(t, store, db, 100, []access.ScheduledTransaction{tx})
			require.NoError(t, err)
		}()

		db := openPebbleScheduledTxDB(t, dir)
		defer db.Close()

		store, err := indexes.NewScheduledTransactionsBootstrapper(db, 100)
		require.NoError(t, err)

		first, err := store.FirstIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(100), first)

		got, err := store.ByID(99)
		require.NoError(t, err)
		assert.Equal(t, uint64(99), got.ID)
	})
}
