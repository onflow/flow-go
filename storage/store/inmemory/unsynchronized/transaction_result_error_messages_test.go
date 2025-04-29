package unsynchronized

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactionResultErrorMessages_HappyPath(t *testing.T) {
	storage := NewTransactionResultErrorMessages()

	// Define block ID and error messages
	block := unittest.BlockFixture()
	txResults := unittest.TransactionResultsFixture(2)
	errorMessages := []flow.TransactionResultErrorMessage{
		{
			TransactionID: txResults[0].TransactionID,
			Index:         0,
			ErrorMessage:  "dummy error message 0",
			ExecutorID:    unittest.IdentifierFixture(),
		},
		{
			TransactionID: txResults[1].TransactionID,
			Index:         1,
			ErrorMessage:  "dummy error message 1",
			ExecutorID:    unittest.IdentifierFixture(),
		},
	}

	// Store error messages
	err := storage.Store(block.ID(), errorMessages)
	require.NoError(t, err)

	// Retrieve by BlockID and TransactionID
	retrievedErrorMessage, err := storage.ByBlockIDTransactionID(block.ID(), errorMessages[0].TransactionID)
	require.NoError(t, err)
	assert.Equal(t, &errorMessages[0], retrievedErrorMessage)

	// Retrieve by BlockID and Index
	retrievedErrorMessageByIndex, err := storage.ByBlockIDTransactionIndex(block.ID(), 0)
	require.NoError(t, err)
	assert.Equal(t, &errorMessages[0], retrievedErrorMessageByIndex)

	// Retrieve by BlockID
	retrievedErrorMessages, err := storage.ByBlockID(block.ID())
	require.NoError(t, err)
	assert.Len(t, retrievedErrorMessages, len(errorMessages))
	assert.Equal(t, errorMessages, retrievedErrorMessages)
}

func TestTransactionResultErrorMessages_Persist(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		store := NewTransactionResultErrorMessages()
		block := unittest.BlockFixture()
		txResults := unittest.TransactionResultsFixture(2)
		errorMessages := []flow.TransactionResultErrorMessage{
			{
				TransactionID: txResults[0].TransactionID,
				Index:         0,
				ErrorMessage:  "dummy error message 0",
				ExecutorID:    unittest.IdentifierFixture(),
			},
		}

		// Store error messages
		err := store.Store(block.ID(), errorMessages)
		require.NoError(t, err)
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return store.AddToBatch(rw)
		}))

		// Get stored error message value
		reader, err := db.Reader()
		require.NoError(t, err)

		var actualTxResultErrorMessages []flow.TransactionResultErrorMessage
		err = operation.LookupTransactionResultErrorMessagesByBlockIDUsingIndex(reader, block.ID(), &actualTxResultErrorMessages)
		require.NoError(t, err)
		assert.Equal(t, errorMessages, actualTxResultErrorMessages)
	})
}
