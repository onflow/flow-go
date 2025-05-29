package unsynchronized

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestLightTransactionResultErrorMessages_HappyPath(t *testing.T) {
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

	// Extract structured data
	messages := storage.Data()
	require.Len(t, messages, len(errorMessages))
	require.ElementsMatch(t, messages, errorMessages)
}
