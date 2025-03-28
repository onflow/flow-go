package store_test

import (
	"fmt"
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

func TestStoringTransactionResultErrorMessages(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store1 := store.NewTransactionResultErrorMessages(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()

		// test db Exists by block id
		exists, err := store1.Exists(blockID)
		require.NoError(t, err)
		require.False(t, exists)

		// check retrieving by ByBlockID
		messages, err := store1.ByBlockID(blockID)
		require.NoError(t, err)
		require.Nil(t, messages)

		txErrorMessages := make([]flow.TransactionResultErrorMessage, 0)
		for i := 0; i < 10; i++ {
			expected := flow.TransactionResultErrorMessage{
				TransactionID: unittest.IdentifierFixture(),
				ErrorMessage:  fmt.Sprintf("a runtime error %d", i),
				ExecutorID:    unittest.IdentifierFixture(),
				Index:         rand.Uint32(),
			}
			txErrorMessages = append(txErrorMessages, expected)
		}
		err = store1.Store(blockID, txErrorMessages)
		require.NoError(t, err)

		// test db Exists by block id
		exists, err = store1.Exists(blockID)
		require.NoError(t, err)
		require.True(t, exists)

		// check retrieving by ByBlockIDTransactionID
		for _, txErrorMessage := range txErrorMessages {
			actual, err := store1.ByBlockIDTransactionID(blockID, txErrorMessage.TransactionID)
			require.NoError(t, err)
			assert.Equal(t, txErrorMessage, *actual)
		}

		// check retrieving by ByBlockIDTransactionIndex
		for _, txErrorMessage := range txErrorMessages {
			actual, err := store1.ByBlockIDTransactionIndex(blockID, txErrorMessage.Index)
			require.NoError(t, err)
			assert.Equal(t, txErrorMessage, *actual)
		}

		// check retrieving by ByBlockID
		actual, err := store1.ByBlockID(blockID)
		require.NoError(t, err)
		assert.Equal(t, txErrorMessages, actual)

		// test loading from database
		newStore := store.NewTransactionResultErrorMessages(metrics, db, 1000)
		for _, txErrorMessage := range txErrorMessages {
			actual, err := newStore.ByBlockIDTransactionID(blockID, txErrorMessage.TransactionID)
			require.NoError(t, err)
			assert.Equal(t, txErrorMessage, *actual)
		}

		// check retrieving by index from both cache and db
		for i, txErrorMessage := range txErrorMessages {
			actual, err := store1.ByBlockIDTransactionIndex(blockID, txErrorMessage.Index)
			require.NoError(t, err)
			assert.Equal(t, txErrorMessages[i], *actual)

			actual, err = newStore.ByBlockIDTransactionIndex(blockID, txErrorMessage.Index)
			require.NoError(t, err)
			assert.Equal(t, txErrorMessages[i], *actual)
		}
	})
}

func TestReadingNotStoreTransactionResultErrorMessage(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store1 := store.NewTransactionResultErrorMessages(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()
		txIndex := rand.Uint32()

		_, err := store1.ByBlockIDTransactionID(blockID, txID)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		_, err = store1.ByBlockIDTransactionIndex(blockID, txIndex)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}
