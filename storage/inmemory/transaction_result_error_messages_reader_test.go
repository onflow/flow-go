package inmemory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestLightTransactionResultErrorMessages_HappyPath(t *testing.T) {

	// Define block ID and error messages
	blockID := unittest.IdentifierFixture()
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

	storage := NewTransactionResultErrorMessages(blockID, errorMessages)

	// Retrieve by BlockID and TransactionID
	retrievedErrorMessage, err := storage.ByBlockIDTransactionID(blockID, errorMessages[0].TransactionID)
	require.NoError(t, err)
	assert.Equal(t, &errorMessages[0], retrievedErrorMessage)

	// Retrieve by BlockID and Index
	retrievedErrorMessageByIndex, err := storage.ByBlockIDTransactionIndex(blockID, 0)
	require.NoError(t, err)
	assert.Equal(t, &errorMessages[0], retrievedErrorMessageByIndex)

	// Retrieve by BlockID
	retrievedErrorMessages, err := storage.ByBlockID(blockID)
	require.NoError(t, err)
	assert.Len(t, retrievedErrorMessages, len(errorMessages))
	assert.Equal(t, errorMessages, retrievedErrorMessages)
}
