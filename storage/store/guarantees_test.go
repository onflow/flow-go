package store_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGuaranteeStoreRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		all := store.InitAll(metrics, db)
		blocks := all.Blocks
		guarantees := all.Guarantees

		s := store.NewGuarantees(metrics, db, 1000)

		// make block with a collection guarantee:
		expected := unittest.CollectionGuaranteeFixture()
		block := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{expected})

		// attempt to retrieve (still) unknown guarantee
		_, err := s.ByCollectionID(expected.ID())
		require.ErrorIs(t, err, storage.ErrNotFound)

		// store guarantee
		unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, block)
			})
		})

		// retrieve the guarantee by the ID of the collection
		actual, err := guarantees.ByCollectionID(expected.ID())
		require.NoError(t, err)
		require.Equal(t, expected, actual)

		// repeated storage of the same block should return
		lctx2 := lockManager.NewContext()
		require.NoError(t, lctx2.AcquireLock(storage.LockInsertBlock))
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx2, rw, block)
		})
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
		lctx2.Release()

		// OK to store a different block
		expected2 := unittest.CollectionGuaranteeFixture()
		block2 := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{expected2})
		lctx3 := lockManager.NewContext()
		require.NoError(t, lctx3.AcquireLock(storage.LockInsertBlock))
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx3, rw, block2)
		}))
		lctx3.Release()
	})
}
