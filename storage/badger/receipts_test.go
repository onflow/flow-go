package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestExecutionReceiptsStorage(t *testing.T) {
	withStore := func(t *testing.T, f func(store *bstorage.ExecutionReceipts)) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			metrics := metrics.NewNoopCollector()
			results := bstorage.NewExecutionResults(metrics, db)
			store := bstorage.NewExecutionReceipts(metrics, db, results, bstorage.DefaultCacheSize)
			f(store)
		})
	}

	t.Run("get empty", func(t *testing.T) {
		withStore(t, func(store *bstorage.ExecutionReceipts) {
			block := unittest.BlockFixture()
			receipts, err := store.ByBlockID(block.ID())
			require.NoError(t, err)
			require.Equal(t, 0, len(receipts))
		})
	})

	t.Run("store one get one", func(t *testing.T) {
		withStore(t, func(store *bstorage.ExecutionReceipts) {
			block := unittest.BlockFixture()
			receipt1 := unittest.ReceiptForBlockFixture(&block)

			err := store.Store(receipt1)
			require.NoError(t, err)

			actual, err := store.ByID(receipt1.ID())
			require.NoError(t, err)

			require.Equal(t, receipt1, actual)

			receipts, err := store.ByBlockID(block.ID())
			require.NoError(t, err)

			require.Equal(t, flow.ExecutionReceiptList{receipt1}, receipts)
		})
	})

	t.Run("store two for the same block", func(t *testing.T) {
		withStore(t, func(store *bstorage.ExecutionReceipts) {
			block := unittest.BlockFixture()

			executor1 := unittest.IdentifierFixture()
			executor2 := unittest.IdentifierFixture()

			receipt1 := unittest.ReceiptForBlockExecutorFixture(&block, executor1)
			receipt2 := unittest.ReceiptForBlockExecutorFixture(&block, executor2)

			err := store.Store(receipt1)
			require.NoError(t, err)

			err = store.Store(receipt2)
			require.NoError(t, err)

			receipts, err := store.ByBlockID(block.ID())
			require.NoError(t, err)

			require.ElementsMatch(t, []*flow.ExecutionReceipt{receipt1, receipt2}, receipts)
		})
	})

	t.Run("store two for different blocks", func(t *testing.T) {
		withStore(t, func(store *bstorage.ExecutionReceipts) {
			block1 := unittest.BlockFixture()
			block2 := unittest.BlockFixture()

			executor1 := unittest.IdentifierFixture()
			executor2 := unittest.IdentifierFixture()

			receipt1 := unittest.ReceiptForBlockExecutorFixture(&block1, executor1)
			receipt2 := unittest.ReceiptForBlockExecutorFixture(&block2, executor2)

			err := store.Store(receipt1)
			require.NoError(t, err)

			err = store.Store(receipt2)
			require.NoError(t, err)

			receipts1, err := store.ByBlockID(block1.ID())
			require.NoError(t, err)

			receipts2, err := store.ByBlockID(block2.ID())
			require.NoError(t, err)

			require.ElementsMatch(t, []*flow.ExecutionReceipt{receipt1}, receipts1)
			require.ElementsMatch(t, []*flow.ExecutionReceipt{receipt2}, receipts2)
		})
	})

	t.Run("indexing duplicated receipts should be ok", func(t *testing.T) {
		withStore(t, func(store *bstorage.ExecutionReceipts) {
			block1 := unittest.BlockFixture()

			executor1 := unittest.IdentifierFixture()
			receipt1 := unittest.ReceiptForBlockExecutorFixture(&block1, executor1)

			err := store.Store(receipt1)
			require.NoError(t, err)

			err = store.Store(receipt1)
			require.NoError(t, err)

			receipts, err := store.ByBlockID(block1.ID())
			require.NoError(t, err)

			require.ElementsMatch(t, []*flow.ExecutionReceipt{receipt1}, receipts)
		})
	})

	t.Run("indexing receipt from the same executor for same block should succeed", func(t *testing.T) {
		withStore(t, func(store *bstorage.ExecutionReceipts) {
			block1 := unittest.BlockFixture()

			executor1 := unittest.IdentifierFixture()

			receipt1 := unittest.ReceiptForBlockExecutorFixture(&block1, executor1)
			receipt2 := unittest.ReceiptForBlockExecutorFixture(&block1, executor1)

			err := store.Store(receipt1)
			require.NoError(t, err)

			err = store.Store(receipt2)
			require.NoError(t, err)

			receipts, err := store.ByBlockID(block1.ID())
			require.NoError(t, err)

			require.ElementsMatch(t, []*flow.ExecutionReceipt{receipt1, receipt2}, receipts)
		})
	})
}
