package store_test

import (
	"errors"
	"fmt"
	mathRand "math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
)

func TestBatchStoringTransactionResults(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		st := store.NewTransactionResults(metrics, db, 1000)

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
		writeBatch := db.NewBatch()
		defer writeBatch.Close()

		err := st.BatchStore(blockID, txResults, writeBatch)
		require.NoError(t, err)

		err = writeBatch.Commit()
		require.NoError(t, err)

		for _, txResult := range txResults {
			actual, err := st.ByBlockIDTransactionID(blockID, txResult.TransactionID)
			require.NoError(t, err)
			assert.Equal(t, txResult, *actual)
		}

		// test loading from database
		newst := store.NewTransactionResults(metrics, db, 1000)
		for _, txResult := range txResults {
			actual, err := newst.ByBlockIDTransactionID(blockID, txResult.TransactionID)
			require.NoError(t, err)
			assert.Equal(t, txResult, *actual)
		}

		// check retrieving by index from both cache and db
		for i := len(txResults) - 1; i >= 0; i-- {
			actual, err := st.ByBlockIDTransactionIndex(blockID, uint32(i))
			require.NoError(t, err)
			assert.Equal(t, txResults[i], *actual)

			actual, err = newst.ByBlockIDTransactionIndex(blockID, uint32(i))
			require.NoError(t, err)
			assert.Equal(t, txResults[i], *actual)
		}
	})
}

func TestBatchStoreAndBatchRemoveTransactionResults(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		st := store.NewTransactionResults(metrics, db, 1000)

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

		// Store transaction results
		err := db.WithReaderBatchWriter(func(rbw storage.ReaderBatchWriter) error {
			return st.BatchStore(blockID, txResults, rbw)
		})
		require.NoError(t, err)

		// Retrieve transaction results
		for _, txResult := range txResults {
			actual, err := st.ByBlockIDTransactionID(blockID, txResult.TransactionID)
			require.NoError(t, err)
			assert.Equal(t, txResult, *actual)
		}

		// Remove transaction results
		err = st.RemoveByBlockID(blockID)
		require.NoError(t, err)

		// Retrieve transaction results
		for _, txResult := range txResults {
			_, err := st.ByBlockIDTransactionID(blockID, txResult.TransactionID)
			require.True(t, errors.Is(err, storage.ErrNotFound))
		}
	})
}

func TestReadingNotstTransaction(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		st := store.NewTransactionResults(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()
		txIndex := rand.Uint32()

		_, err := st.ByBlockIDTransactionID(blockID, txID)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		_, err = st.ByBlockIDTransactionIndex(blockID, txIndex)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestKeyConversion(t *testing.T) {
	blockID := unittest.IdentifierFixture()
	txID := unittest.IdentifierFixture()
	key := store.KeyFromBlockIDTransactionID(blockID, txID)
	bID, tID, err := store.KeyToBlockIDTransactionID(key)
	require.NoError(t, err)
	require.Equal(t, blockID, bID)
	require.Equal(t, txID, tID)
}

func TestIndexKeyConversion(t *testing.T) {
	blockID := unittest.IdentifierFixture()
	txIndex := mathRand.Uint32()
	key := store.KeyFromBlockIDIndex(blockID, txIndex)
	bID, tID, err := store.KeyToBlockIDIndex(key)
	require.NoError(t, err)
	require.Equal(t, blockID, bID)
	require.Equal(t, txIndex, tID)
}
