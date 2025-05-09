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

		s := store.NewGuarantees(metrics, db, 1000)

		// abiturary guarantees
		expected := unittest.CollectionGuaranteeFixture()
		block := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{expected})

		// retrieve guarantee without stored
		_, err := s.ByCollectionID(expected.ID())
		require.ErrorIs(t, err, storage.ErrNotFound)

		// store guarantee
		manager, lctx := unittest.LockManagerWithContext(t, storage.LockInsertBlock)
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx, rw, block)
		}))
		lctx.Release()

		// retreive by coll idx
		actual, err := guarantees.ByCollectionID(expected.ID())
		require.NoError(t, err)
		require.Equal(t, expected, actual)

		// OK to store again
		lctx2 := manager.NewContext()
		require.NoError(t, lctx2.AcquireLock(storage.LockInsertBlock))
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx2, rw, block)
		}))
		lctx2.Release()
	})
}
