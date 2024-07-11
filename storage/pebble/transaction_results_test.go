package badger_test

import (
	"fmt"
	mathRand "math/rand"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	bstorage "github.com/onflow/flow-go/storage/badger"
)

func TestBatchStoringTransactionResults(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewTransactionResults(metrics, db, 1000)

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
			require.NoError(t, err)
			assert.Equal(t, txResult, *actual)
		}

		// test loading from database
		newStore := bstorage.NewTransactionResults(metrics, db, 1000)
		for _, txResult := range txResults {
			actual, err := newStore.ByBlockIDTransactionID(blockID, txResult.TransactionID)
			require.NoError(t, err)
			assert.Equal(t, txResult, *actual)
		}

		// check retrieving by index from both cache and db
		for i := len(txResults) - 1; i >= 0; i-- {
			actual, err := store.ByBlockIDTransactionIndex(blockID, uint32(i))
			require.NoError(t, err)
			assert.Equal(t, txResults[i], *actual)

			actual, err = newStore.ByBlockIDTransactionIndex(blockID, uint32(i))
			require.NoError(t, err)
			assert.Equal(t, txResults[i], *actual)
		}
	})
}

func TestReadingNotStoreTransaction(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewTransactionResults(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()
		txIndex := rand.Uint32()

		_, err := store.ByBlockIDTransactionID(blockID, txID)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		_, err = store.ByBlockIDTransactionIndex(blockID, txIndex)
		assert.ErrorIs(t, err, storage.ErrNotFound)
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

func TestIndexKeyConversion(t *testing.T) {
	blockID := unittest.IdentifierFixture()
	txIndex := mathRand.Uint32()
	key := bstorage.KeyFromBlockIDIndex(blockID, txIndex)
	bID, tID, err := bstorage.KeyToBlockIDIndex(key)
	require.NoError(t, err)
	require.Equal(t, blockID, bID)
	require.Equal(t, txIndex, tID)
}
