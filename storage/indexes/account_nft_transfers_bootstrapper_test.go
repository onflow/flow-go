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

func TestNFTBootstrapper_Constructor(t *testing.T) {
	t.Parallel()

	t.Run("uninitialized DB returns ErrNotBootstrapped from height methods", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			store, err := NewNonFungibleTokenTransfersBootstrapper(storageDB, 10)
			require.NoError(t, err)

			_, err = store.FirstIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)

			_, err = store.LatestIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})
	})

	t.Run("already-bootstrapped DB is immediately usable", func(t *testing.T) {
		RunWithBootstrappedNFTTransferIndex(t, 5, nil, func(db storage.DB, _ storage.LockManager, _ *NonFungibleTokenTransfers) {
			store, err := NewNonFungibleTokenTransfersBootstrapper(db, 5)
			require.NoError(t, err)

			first, err := store.FirstIndexedHeight()
			require.NoError(t, err)
			assert.Equal(t, uint64(5), first)

			latest, err := store.LatestIndexedHeight()
			require.NoError(t, err)
			assert.Equal(t, uint64(5), latest)
		})
	})
}

func TestNFTBootstrapper_PreBootstrapState(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := NewNonFungibleTokenTransfersBootstrapper(storageDB, 42)
		require.NoError(t, err)

		t.Run("FirstIndexedHeight returns ErrNotBootstrapped", func(t *testing.T) {
			height, err := store.FirstIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
			assert.Equal(t, uint64(0), height)
		})

		t.Run("LatestIndexedHeight returns ErrNotBootstrapped", func(t *testing.T) {
			height, err := store.LatestIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
			assert.Equal(t, uint64(0), height)
		})

		t.Run("TransfersByAddress returns ErrNotBootstrapped", func(t *testing.T) {
			_, err := store.TransfersByAddress(unittest.RandomAddressFixture(), 100, nil, nil)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})

		t.Run("UninitializedFirstHeight returns initialStartHeight and false", func(t *testing.T) {
			height, initialized := store.UninitializedFirstHeight()
			assert.Equal(t, uint64(42), height)
			assert.False(t, initialized)
		})
	})
}

func TestNFTBootstrapper_StoreTriggersBootstrap(t *testing.T) {
	t.Parallel()

	t.Run("Store at initialStartHeight bootstraps the index", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			store, err := NewNonFungibleTokenTransfersBootstrapper(storageDB, 10)
			require.NoError(t, err)

			err = storeNFTBootstrapperTransfers(t, store, storageDB, 10, nil)
			require.NoError(t, err)

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
			store, err := NewNonFungibleTokenTransfersBootstrapper(storageDB, 10)
			require.NoError(t, err)

			err = storeNFTBootstrapperTransfers(t, store, storageDB, 11, nil)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)

			err = storeNFTBootstrapperTransfers(t, store, storageDB, 9, nil)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})
	})
}

func TestNFTBootstrapper_BootstrapWithData(t *testing.T) {
	t.Parallel()

	t.Run("bootstrap with transfer data persists and is queryable", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)

			firstHeight := uint64(5)
			store, err := NewNonFungibleTokenTransfersBootstrapper(storageDB, firstHeight)
			require.NoError(t, err)

			source := unittest.RandomAddressFixture()
			recipient := unittest.RandomAddressFixture()
			txID := unittest.IdentifierFixture()

			transfers := []access.NonFungibleTokenTransfer{
				{
					TransactionID:    txID,
					BlockHeight:      firstHeight,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    source,
					RecipientAddress: recipient,
					ID:               42,
				},
			}

			err = storeNFTBootstrapperTransfers(t, store, storageDB, firstHeight, transfers)
			require.NoError(t, err)

			page, err := store.TransfersByAddress(source, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 1)
			assert.Equal(t, txID, page.Transfers[0].TransactionID)
			assert.Equal(t, uint64(5), page.Transfers[0].BlockHeight)
			assert.Equal(t, uint32(0), page.Transfers[0].TransactionIndex)
			assert.Equal(t, source, page.Transfers[0].SourceAddress)
			assert.Equal(t, recipient, page.Transfers[0].RecipientAddress)
			assert.Equal(t, uint64(42), page.Transfers[0].ID)
		})
	})

	t.Run("subsequent stores work after bootstrap", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			store, err := NewNonFungibleTokenTransfersBootstrapper(storageDB, 1)
			require.NoError(t, err)

			source := unittest.RandomAddressFixture()
			recipient := unittest.RandomAddressFixture()
			txID1 := unittest.IdentifierFixture()
			txID2 := unittest.IdentifierFixture()

			err = storeNFTBootstrapperTransfers(t, store, storageDB, 1, []access.NonFungibleTokenTransfer{
				{
					TransactionID:    txID1,
					BlockHeight:      1,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    source,
					RecipientAddress: recipient,
					ID:               1,
				},
			})
			require.NoError(t, err)

			err = storeNFTBootstrapperTransfers(t, store, storageDB, 2, []access.NonFungibleTokenTransfer{
				{
					TransactionID:    txID2,
					BlockHeight:      2,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    source,
					RecipientAddress: recipient,
					ID:               2,
				},
			})
			require.NoError(t, err)

			page, err := store.TransfersByAddress(source, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 2)

			// Descending order: height 2 first, then height 1
			assert.Equal(t, txID2, page.Transfers[0].TransactionID)
			assert.Equal(t, uint64(2), page.Transfers[0].BlockHeight)
			assert.Equal(t, uint64(2), page.Transfers[0].ID)

			assert.Equal(t, txID1, page.Transfers[1].TransactionID)
			assert.Equal(t, uint64(1), page.Transfers[1].BlockHeight)
			assert.Equal(t, uint64(1), page.Transfers[1].ID)

			latest, err := store.LatestIndexedHeight()
			require.NoError(t, err)
			assert.Equal(t, uint64(2), latest)
		})
	})
}

func TestNFTBootstrapper_StoreAtSameHeight(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := NewNonFungibleTokenTransfersBootstrapper(storageDB, 5)
		require.NoError(t, err)

		// Bootstrap at height 5
		err = storeNFTBootstrapperTransfers(t, store, storageDB, 5, nil)
		require.NoError(t, err)

		// Store at height 5 again (no-op)
		err = storeNFTBootstrapperTransfers(t, store, storageDB, 5, nil)
		require.NoError(t, err)

		latest, err := store.LatestIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(5), latest)
	})
}

func TestNFTBootstrapper_StoreBelowLatest(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := NewNonFungibleTokenTransfersBootstrapper(storageDB, 5)
		require.NoError(t, err)

		// Bootstrap at height 5
		err = storeNFTBootstrapperTransfers(t, store, storageDB, 5, nil)
		require.NoError(t, err)

		// Store at height 6
		err = storeNFTBootstrapperTransfers(t, store, storageDB, 6, nil)
		require.NoError(t, err)

		// Store at height 4 (below latest=6)
		err = storeNFTBootstrapperTransfers(t, store, storageDB, 4, nil)
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
	})
}

func TestNFTBootstrapper_NonConsecutiveStoreAfterBootstrap(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := NewNonFungibleTokenTransfersBootstrapper(storageDB, 5)
		require.NoError(t, err)

		// Bootstrap at height 5
		err = storeNFTBootstrapperTransfers(t, store, storageDB, 5, nil)
		require.NoError(t, err)

		// Attempt to store at height 7, skipping height 6
		err = storeNFTBootstrapperTransfers(t, store, storageDB, 7, nil)
		require.Error(t, err)
		assert.False(t, errors.Is(err, storage.ErrAlreadyExists))
		assert.False(t, errors.Is(err, storage.ErrNotBootstrapped))
	})
}

func TestNFTBootstrapper_DoubleBootstrapProtection(t *testing.T) {
	t.Parallel()

	lockManager := storage.NewTestingLockManager()
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		err := unittest.WithLock(t, lockManager, storage.LockIndexNonFungibleTokenTransfers, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapNonFungibleTokenTransfers(lctx, rw, storageDB, 1, nil)
				return bootstrapErr
			})
		})
		require.NoError(t, err)

		// Attempting to bootstrap again should fail
		err = unittest.WithLock(t, lockManager, storage.LockIndexNonFungibleTokenTransfers, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapNonFungibleTokenTransfers(lctx, rw, storageDB, 1, nil)
				return bootstrapErr
			})
		})
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
	})
}

func TestNFTBootstrapper_PersistenceAcrossRestart(t *testing.T) {
	t.Parallel()

	unittest.RunWithTempDir(t, func(dir string) {
		source := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()

		func() {
			db := openPebbleDB(t, dir)
			defer db.Close()

			store, err := NewNonFungibleTokenTransfersBootstrapper(db, 100)
			require.NoError(t, err)

			_, err = store.FirstIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)

			err = storeNFTBootstrapperTransfers(t, store, db, 100, []access.NonFungibleTokenTransfer{
				{
					TransactionID:    txID,
					BlockHeight:      100,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    source,
					RecipientAddress: recipient,
					ID:               77,
				},
			})
			require.NoError(t, err)
		}()

		db := openPebbleDB(t, dir)
		defer db.Close()

		store, err := NewNonFungibleTokenTransfersBootstrapper(db, 100)
		require.NoError(t, err)

		first, err := store.FirstIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(100), first)

		page, err := store.TransfersByAddress(source, 100, nil, nil)
		require.NoError(t, err)
		require.Len(t, page.Transfers, 1)
		assert.Equal(t, txID, page.Transfers[0].TransactionID)
		assert.Equal(t, uint64(77), page.Transfers[0].ID)
	})
}

func storeNFTBootstrapperTransfers(
	tb testing.TB,
	store storage.NonFungibleTokenTransfersBootstrapper,
	db storage.DB,
	height uint64,
	transfers []access.NonFungibleTokenTransfer,
) error {
	tb.Helper()
	lockManager := storage.NewTestingLockManager()
	return unittest.WithLock(tb, lockManager, storage.LockIndexNonFungibleTokenTransfers, func(lctx lockctx.Context) error {
		return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return store.Store(lctx, rw, height, transfers)
		})
	})
}
