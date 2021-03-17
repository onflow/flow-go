package badger_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	bstorage "github.com/onflow/flow-go/storage/badger"
)

func TestBatchStoringTransactionResults(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewTransactionResults(metrics, db)

		blockID := unittest.IdentifierFixture()
		txResults := make([]flow.TransactionResult, 0)
		for i := 0; i < 10; i++ {
			txID := unittest.IdentifierFixture()
			expected := flow.TransactionResult{
				TransactionID: txID,
				ErrorMessage:  fmt.Sprintf("a runtime error %d", i),
			}
			txResults = append(txResults, expected)
		}
		writeBatch := bstorage.NewBatch(db)
		err := store.BatchStore(blockID, txResults, writeBatch)
		require.NoError(t, err)

		err = writeBatch.Flush()
		require.NoError(t, err)

		for _, txResult := range txResults {
			actual, err := store.ByBlockIDTransactionID(blockID, txResult.TransactionID)
			require.Nil(t, err)
			assert.Equal(t, txResult, *actual)
		}
	})
}

func TestReadingNotStoreTransaction(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewTransactionResults(metrics, db)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()

		_, err := store.ByBlockIDTransactionID(blockID, txID)
		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

func TestKeyConversion(t *testing.T) {
	blockID := unittest.IdentifierFixture()
	txID := unittest.IdentifierFixture()
	key := bstorage.KeyFromBlockIDTransactionID(blockID, txID)
	bID, tID, err := bstorage.KeyToBlockIDTransactionID(key)
	require.NoError(t, err)
	require.Equal(t, blockID, bID)
	require.Equal(t, txID, tID)
}
