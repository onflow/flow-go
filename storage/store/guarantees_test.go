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

// TestGuaranteeStoreRetrieve tests storing and retrieving collection guarantees.
// Generally, collection guarantees are persisted as part of storing a block proposal -- we follow that approach here.
// We test the following:
//   - retrieving an unknown guarantee returns [storage.ErrNotFound]
//   - storing a guarantee as part of a block proposal and then retrieving it by its ID
//   - repeated storage of the same block returns [storage.ErrAlreadyExists]
//     and collection guarantee can still be retrieved
//   - storing a different block with holding the same guarantee also works
func TestGuaranteeStoreRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		all := store.InitAll(metrics, db, flow.Emulator)
		blocks := all.Blocks
		guarantees := all.Guarantees

		s := store.NewGuarantees(metrics, db, 1000, 1000)

		// make block with a collection guarantee:
		guarantee1 := unittest.CollectionGuaranteeFixture()
		block := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{guarantee1})
		proposal := unittest.ProposalFromBlock(block)

		// attempt to retrieve (still) unknown guarantee
		_, err := s.ByCollectionID(guarantee1.ID())
		require.ErrorIs(t, err, storage.ErrNotFound)

		// store guarantee
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, proposal)
			})
		})
		require.NoError(t, err)

		// retrieve the guarantee by the ID of the collection
		actual, err := guarantees.ByCollectionID(guarantee1.CollectionID)
		require.NoError(t, err)
		require.Equal(t, guarantee1, actual)

		// Repeated storage of the same block should return [storage.ErrAlreadyExists].
		// Yet, the guarantee can still be retrieved.
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx2 lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx2, rw, proposal)
			})
		})
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
		actual, err = guarantees.ByCollectionID(guarantee1.CollectionID)
		require.NoError(t, err)
		require.Equal(t, guarantee1, actual)

		// OK to store a different block holding the _same_ guarantee (this is possible across forks).
		guarantee2 := unittest.CollectionGuaranteeFixture()
		block2 := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{guarantee2, guarantee1})
		proposal2 := unittest.ProposalFromBlock(block2)
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx3 lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx3, rw, proposal2)
			})
		})
		require.NoError(t, err)
		// retrieving guarantee 1 (contained in both blocks) still works
		actual, err = guarantees.ByCollectionID(guarantee1.CollectionID)
		require.NoError(t, err)
		require.Equal(t, guarantee1, actual)
		// retrieving guarantee 2 (contained only in the second block):
		actual, err = guarantees.ByCollectionID(guarantee2.CollectionID)
		require.NoError(t, err)
		require.Equal(t, guarantee2, actual)
	})
}

// Storing the same guarantee should be idempotent
func TestStoreDuplicateGuarantee(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		all := store.InitAll(metrics, db, flow.Emulator)
		blocks := all.Blocks
		store1 := all.Guarantees
		expected := unittest.CollectionGuaranteeFixture()
		block := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{expected})
		proposal := unittest.ProposalFromBlock(block)

		// store guarantee
		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, proposal)
			})
		})
		require.NoError(t, err)

		// storage of the same guarantee should be idempotent
		block2 := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{expected})
		proposal2 := unittest.ProposalFromBlock(block2)
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx2 lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx2, rw, proposal2)
			})
		})
		require.NoError(t, err)

		actual, err := store1.ByID(expected.ID())
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		actual, err = store1.ByCollectionID(expected.CollectionID)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

// Storing a different guarantee for the same collection should return [storage.ErrDataMismatch]
func TestStoreConflictingGuarantee(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		all := store.InitAll(metrics, db, flow.Emulator)
		blocks := all.Blocks
		store1 := all.Guarantees
		expected := unittest.CollectionGuaranteeFixture()
		block := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{expected})
		proposal := unittest.ProposalFromBlock(block)

		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, proposal)
			})
		})
		require.NoError(t, err)

		// a differing guarantee for the same collection is potentially byzantine and should return [storage.ErrDataMismatch]
		conflicting := *expected
		conflicting.SignerIndices = []byte{99}
		block2 := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{&conflicting})
		proposal2 := unittest.ProposalFromBlock(block2)
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, proposal2)
			})
		})
		require.ErrorIs(t, err, storage.ErrDataMismatch)

		actual, err := store1.ByID(expected.ID())
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		actual, err = store1.ByCollectionID(expected.CollectionID)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}
