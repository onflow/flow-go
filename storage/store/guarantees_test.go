package store_test

import (
	"testing"

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
		metrics := metrics.NewNoopCollector()
		all := store.InitAll(metrics, db)
		blocks := all.Blocks
		guarantees := all.Guarantees

		s := store.NewGuarantees(metrics, db, 1000, 1000)

		// abiturary guarantees
		expected := unittest.CollectionGuaranteeFixture()

		block := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{expected})
		proposal := unittest.ProposalFromBlock(block)

		// retrieve guarantee without stored
		_, err := s.ByCollectionID(expected.ID())
		require.ErrorIs(t, err, storage.ErrNotFound)

		// store guarantee
		manager, lctx := unittest.LockManagerWithContext(t, storage.LockInsertBlock)
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx, rw, proposal)
		}))
		lctx.Release()

		// retreive by coll idx
		actual, err := guarantees.ByCollectionID(expected.ID())
		require.NoError(t, err)
		require.Equal(t, expected, actual)

		// OK to store a different block
		expected2 := unittest.CollectionGuaranteeFixture()
		block2 := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{expected2})
		proposal2 := unittest.ProposalFromBlock(block2)
		lctx2 := manager.NewContext()
		require.NoError(t, lctx2.AcquireLock(storage.LockInsertBlock))
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx2, rw, proposal2)
		}))
		lctx2.Release()
	})
}

func TestStoreDuplicateGuarantee(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		all := store.InitAll(metrics, db)
		blocks := all.Blocks
		store1 := all.Guarantees
		expected := unittest.CollectionGuaranteeFixture()
		block := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{expected})
		proposal := unittest.ProposalFromBlock(block)

		// store guarantee
		manager, lctx := unittest.LockManagerWithContext(t, storage.LockInsertBlock)
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx, rw, proposal)
		}))
		lctx.Release()

		// storage of the same guarantee should be idempotent
		block2 := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{expected})
		proposal2 := unittest.ProposalFromBlock(block2)
		lctx2 := manager.NewContext()
		require.NoError(t, lctx2.AcquireLock(storage.LockInsertBlock))
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx2, rw, proposal2)
		}))
		lctx2.Release()

		actual, err := store1.ByID(expected.ID())
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		actual, err = store1.ByCollectionID(expected.CollectionID)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

func TestStoreConflictingGuarantee(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		all := store.InitAll(metrics, db)
		blocks := all.Blocks
		store1 := all.Guarantees
		expected := unittest.CollectionGuaranteeFixture()
		block := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{expected})
		proposal := unittest.ProposalFromBlock(block)

		manager, lctx := unittest.LockManagerWithContext(t, storage.LockInsertBlock)
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx, rw, proposal)
		}))
		lctx.Release()

		// a differing guarantee for the same collection is potentially byzantine and should return an error
		conflicting := *expected
		conflicting.SignerIndices = []byte{99}
		block2 := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{&conflicting})
		proposal2 := unittest.ProposalFromBlock(block2)
		lctx2 := manager.NewContext()
		require.NoError(t, lctx2.AcquireLock(storage.LockInsertBlock))
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx2, rw, proposal2)
		}))
		lctx2.Release()

		actual, err := store1.ByID(expected.ID())
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		actual, err = store1.ByCollectionID(expected.CollectionID)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}
