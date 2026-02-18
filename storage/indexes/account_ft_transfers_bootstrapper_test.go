package indexes

import (
	"errors"
	"math/big"
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

func TestFTBootstrapper_Constructor(t *testing.T) {
	t.Parallel()

	t.Run("uninitialized DB returns ErrNotBootstrapped from height methods", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			store, err := NewFungibleTokenTransfersBootstrapper(storageDB, 10)
			require.NoError(t, err)

			_, err = store.FirstIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)

			_, err = store.LatestIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})
	})

	t.Run("already-bootstrapped DB is immediately usable", func(t *testing.T) {
		RunWithBootstrappedFTTransferIndex(t, 5, nil, func(db storage.DB, _ storage.LockManager, _ *FungibleTokenTransfers) {
			store, err := NewFungibleTokenTransfersBootstrapper(db, 5)
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

func TestFTBootstrapper_PreBootstrapState(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := NewFungibleTokenTransfersBootstrapper(storageDB, 42)
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

func TestFTBootstrapper_StoreTriggersBootstrap(t *testing.T) {
	t.Parallel()

	t.Run("Store at initialStartHeight bootstraps the index", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			store, err := NewFungibleTokenTransfersBootstrapper(storageDB, 10)
			require.NoError(t, err)

			err = storeFTBootstrapperTransfers(t, store, storageDB, 10, nil)
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
			store, err := NewFungibleTokenTransfersBootstrapper(storageDB, 10)
			require.NoError(t, err)

			err = storeFTBootstrapperTransfers(t, store, storageDB, 11, nil)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)

			err = storeFTBootstrapperTransfers(t, store, storageDB, 9, nil)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})
	})
}

func TestFTBootstrapper_BootstrapWithData(t *testing.T) {
	t.Parallel()

	t.Run("bootstrap with transfer data persists and is queryable", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)

			firstHeight := uint64(5)
			store, err := NewFungibleTokenTransfersBootstrapper(storageDB, firstHeight)
			require.NoError(t, err)

			source := unittest.RandomAddressFixture()
			recipient := unittest.RandomAddressFixture()
			txID := unittest.IdentifierFixture()

			transfers := []access.FungibleTokenTransfer{
				{
					TransactionID:    txID,
					BlockHeight:      firstHeight,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    source,
					RecipientAddress: recipient,
					TokenType:        "A.0x1654653399040a61.FlowToken",
					Amount:           big.NewInt(1000),
				},
			}

			err = storeFTBootstrapperTransfers(t, store, storageDB, firstHeight, transfers)
			require.NoError(t, err)

			page, err := store.TransfersByAddress(source, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 1)
			assert.Equal(t, txID, page.Transfers[0].TransactionID)
			assert.Equal(t, uint64(5), page.Transfers[0].BlockHeight)
			assert.Equal(t, uint32(0), page.Transfers[0].TransactionIndex)
			assert.Equal(t, source, page.Transfers[0].SourceAddress)
			assert.Equal(t, recipient, page.Transfers[0].RecipientAddress)
			assert.Equal(t, "A.0x1654653399040a61.FlowToken", page.Transfers[0].TokenType)
			assert.Equal(t, 0, big.NewInt(1000).Cmp(page.Transfers[0].Amount))
		})
	})

	t.Run("subsequent stores work after bootstrap", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			store, err := NewFungibleTokenTransfersBootstrapper(storageDB, 1)
			require.NoError(t, err)

			source := unittest.RandomAddressFixture()
			recipient := unittest.RandomAddressFixture()
			txID1 := unittest.IdentifierFixture()
			txID2 := unittest.IdentifierFixture()

			err = storeFTBootstrapperTransfers(t, store, storageDB, 1, []access.FungibleTokenTransfer{
				{
					TransactionID:    txID1,
					BlockHeight:      1,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    source,
					RecipientAddress: recipient,
					TokenType:        "A.0x1654653399040a61.FlowToken",
					Amount:           big.NewInt(100),
				},
			})
			require.NoError(t, err)

			err = storeFTBootstrapperTransfers(t, store, storageDB, 2, []access.FungibleTokenTransfer{
				{
					TransactionID:    txID2,
					BlockHeight:      2,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    source,
					RecipientAddress: recipient,
					TokenType:        "A.0x1654653399040a61.FlowToken",
					Amount:           big.NewInt(200),
				},
			})
			require.NoError(t, err)

			page, err := store.TransfersByAddress(source, 100, nil, nil)
			require.NoError(t, err)
			require.Len(t, page.Transfers, 2)

			// Descending order: height 2 first, then height 1
			assert.Equal(t, txID2, page.Transfers[0].TransactionID)
			assert.Equal(t, uint64(2), page.Transfers[0].BlockHeight)

			assert.Equal(t, txID1, page.Transfers[1].TransactionID)
			assert.Equal(t, uint64(1), page.Transfers[1].BlockHeight)

			latest, err := store.LatestIndexedHeight()
			require.NoError(t, err)
			assert.Equal(t, uint64(2), latest)
		})
	})
}

func TestFTBootstrapper_StoreAtSameHeight(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := NewFungibleTokenTransfersBootstrapper(storageDB, 5)
		require.NoError(t, err)

		// Bootstrap at height 5
		err = storeFTBootstrapperTransfers(t, store, storageDB, 5, nil)
		require.NoError(t, err)

		// Store at height 5 again (no-op)
		err = storeFTBootstrapperTransfers(t, store, storageDB, 5, nil)
		require.NoError(t, err)

		latest, err := store.LatestIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(5), latest)
	})
}

func TestFTBootstrapper_StoreBelowLatest(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := NewFungibleTokenTransfersBootstrapper(storageDB, 5)
		require.NoError(t, err)

		// Bootstrap at height 5
		err = storeFTBootstrapperTransfers(t, store, storageDB, 5, nil)
		require.NoError(t, err)

		// Store at height 6
		err = storeFTBootstrapperTransfers(t, store, storageDB, 6, nil)
		require.NoError(t, err)

		// Store at height 4 (below latest=6)
		err = storeFTBootstrapperTransfers(t, store, storageDB, 4, nil)
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
	})
}

func TestFTBootstrapper_NonConsecutiveStoreAfterBootstrap(t *testing.T) {
	t.Parallel()

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		store, err := NewFungibleTokenTransfersBootstrapper(storageDB, 5)
		require.NoError(t, err)

		// Bootstrap at height 5
		err = storeFTBootstrapperTransfers(t, store, storageDB, 5, nil)
		require.NoError(t, err)

		// Attempt to store at height 7, skipping height 6
		err = storeFTBootstrapperTransfers(t, store, storageDB, 7, nil)
		require.Error(t, err)
		assert.False(t, errors.Is(err, storage.ErrAlreadyExists))
		assert.False(t, errors.Is(err, storage.ErrNotBootstrapped))
	})
}

func TestFTBootstrapper_DoubleBootstrapProtection(t *testing.T) {
	t.Parallel()

	lockManager := storage.NewTestingLockManager()
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		err := unittest.WithLock(t, lockManager, storage.LockIndexFungibleTokenTransfers, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapFungibleTokenTransfers(lctx, rw, storageDB, 1, nil)
				return bootstrapErr
			})
		})
		require.NoError(t, err)

		// Attempting to bootstrap again should fail
		err = unittest.WithLock(t, lockManager, storage.LockIndexFungibleTokenTransfers, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapFungibleTokenTransfers(lctx, rw, storageDB, 1, nil)
				return bootstrapErr
			})
		})
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
	})
}

func TestFTBootstrapper_PersistenceAcrossRestart(t *testing.T) {
	t.Parallel()

	unittest.RunWithTempDir(t, func(dir string) {
		source := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()

		func() {
			db := openPebbleDB(t, dir)
			defer db.Close()

			store, err := NewFungibleTokenTransfersBootstrapper(db, 100)
			require.NoError(t, err)

			_, err = store.FirstIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)

			err = storeFTBootstrapperTransfers(t, store, db, 100, []access.FungibleTokenTransfer{
				{
					TransactionID:    txID,
					BlockHeight:      100,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					SourceAddress:    source,
					RecipientAddress: recipient,
					TokenType:        "A.0x1654653399040a61.FlowToken",
					Amount:           big.NewInt(500),
				},
			})
			require.NoError(t, err)
		}()

		db := openPebbleDB(t, dir)
		defer db.Close()

		store, err := NewFungibleTokenTransfersBootstrapper(db, 100)
		require.NoError(t, err)

		first, err := store.FirstIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, uint64(100), first)

		page, err := store.TransfersByAddress(source, 100, nil, nil)
		require.NoError(t, err)
		require.Len(t, page.Transfers, 1)
		assert.Equal(t, txID, page.Transfers[0].TransactionID)
	})
}

func storeFTBootstrapperTransfers(
	tb testing.TB,
	store storage.FungibleTokenTransfersBootstrapper,
	db storage.DB,
	height uint64,
	transfers []access.FungibleTokenTransfer,
) error {
	tb.Helper()
	lockManager := storage.NewTestingLockManager()
	return unittest.WithLock(tb, lockManager, storage.LockIndexFungibleTokenTransfers, func(lctx lockctx.Context) error {
		return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return store.Store(lctx, rw, height, transfers)
		})
	})
}
