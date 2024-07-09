package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMyExecutionReceiptsStorage(t *testing.T) {
	withStore := func(t *testing.T, f func(store *bstorage.MyExecutionReceipts)) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			metrics := metrics.NewNoopCollector()
			results := bstorage.NewExecutionResults(metrics, db)
			receipts := bstorage.NewExecutionReceipts(metrics, db, results, bstorage.DefaultCacheSize)
			store := bstorage.NewMyExecutionReceipts(metrics, db, receipts)

			f(store)
		})
	}

	t.Run("store one get one", func(t *testing.T) {
		withStore(t, func(store *bstorage.MyExecutionReceipts) {
			block := unittest.BlockFixture()
			receipt1 := unittest.ReceiptForBlockFixture(&block)

			err := store.StoreMyReceipt(receipt1)
			require.NoError(t, err)

			actual, err := store.MyReceipt(block.ID())
			require.NoError(t, err)

			require.Equal(t, receipt1, actual)
		})
	})

	t.Run("store same for the same block", func(t *testing.T) {
		withStore(t, func(store *bstorage.MyExecutionReceipts) {
			block := unittest.BlockFixture()

			receipt1 := unittest.ReceiptForBlockFixture(&block)

			err := store.StoreMyReceipt(receipt1)
			require.NoError(t, err)

			err = store.StoreMyReceipt(receipt1)
			require.NoError(t, err)
		})
	})

	t.Run("store different receipt for same block should fail", func(t *testing.T) {
		withStore(t, func(store *bstorage.MyExecutionReceipts) {
			block := unittest.BlockFixture()

			executor1 := unittest.IdentifierFixture()
			executor2 := unittest.IdentifierFixture()

			receipt1 := unittest.ReceiptForBlockExecutorFixture(&block, executor1)
			receipt2 := unittest.ReceiptForBlockExecutorFixture(&block, executor2)

			err := store.StoreMyReceipt(receipt1)
			require.NoError(t, err)

			err = store.StoreMyReceipt(receipt2)
			require.Error(t, err)
		})
	})
}
