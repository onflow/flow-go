package chained

import (
	"fmt"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTxResultsOnlyFirstHave(t *testing.T) {
	unittest.RunWithBadgerDBAndPebbleDB(t, func(bdb *badger.DB, pdb *pebble.DB) {
		bresults := store.NewTransactionResults(metrics.NewNoopCollector(), badgerimpl.ToDB(bdb), 1)
		presults := store.NewTransactionResults(metrics.NewNoopCollector(), pebbleimpl.ToDB(pdb), 1)

		blockID := unittest.IdentifierFixture()
		txResults := make([]flow.TransactionResult, 0, 10)
		for i := 0; i < 10; i++ {
			txID := unittest.IdentifierFixture()
			expected := flow.TransactionResult{
				TransactionID: txID,
				ErrorMessage:  fmt.Sprintf("a runtime error %d", i),
			}
			txResults = append(txResults, expected)
		}

		chained := NewTransactionResults(presults, bresults)

		// not found
		actual, err := chained.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, actual, 0)

		// only stored in first
		require.NoError(t, pebbleimpl.ToDB(pdb).WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return presults.BatchStore(blockID, txResults, rw)
		}))
		actual, err = chained.ByBlockID(blockID)
		require.NoError(t, err)

		require.Equal(t, txResults, actual)
	})
}

func TestTxResultsOnlySecondHave(t *testing.T) {
	unittest.RunWithBadgerDBAndPebbleDB(t, func(bdb *badger.DB, pdb *pebble.DB) {
		bresults := store.NewTransactionResults(metrics.NewNoopCollector(), badgerimpl.ToDB(bdb), 1)
		presults := store.NewTransactionResults(metrics.NewNoopCollector(), pebbleimpl.ToDB(pdb), 1)

		blockID := unittest.IdentifierFixture()
		txResults := make([]flow.TransactionResult, 0, 10)
		for i := 0; i < 10; i++ {
			txID := unittest.IdentifierFixture()
			expected := flow.TransactionResult{
				TransactionID: txID,
				ErrorMessage:  fmt.Sprintf("a runtime error %d", i),
			}
			txResults = append(txResults, expected)
		}

		chained := NewTransactionResults(presults, bresults)

		// not found
		actual, err := chained.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, actual, 0)

		// only stored in second
		require.NoError(t, badgerimpl.ToDB(bdb).WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return bresults.BatchStore(blockID, txResults, rw)
		}))
		actual, err = chained.ByBlockID(blockID)
		require.NoError(t, err)

		require.Equal(t, txResults, actual)
	})
}
