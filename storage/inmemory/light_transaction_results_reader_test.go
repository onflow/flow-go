package inmemory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestLightTransactionResults_HappyPath(t *testing.T) {

	// Define block ID and transaction results
	blockID := unittest.IdentifierFixture()
	txResults := unittest.LightTransactionResultsFixture(10)

	ltx := NewLightTransactionResults(blockID, txResults)

	// Retrieve by BlockID and TransactionID
	retrievedTx, err := ltx.ByBlockIDTransactionID(blockID, txResults[0].TransactionID)
	require.NoError(t, err)
	assert.Equal(t, &txResults[0], retrievedTx)

	// Retrieve by BlockID and Index
	retrievedTxByIndex, err := ltx.ByBlockIDTransactionIndex(blockID, 0)
	require.NoError(t, err)
	assert.Equal(t, &txResults[0], retrievedTxByIndex)

	// Retrieve by BlockID
	retrievedTxs, err := ltx.ByBlockID(blockID)
	require.NoError(t, err)
	assert.Len(t, retrievedTxs, len(txResults))
	assert.Equal(t, txResults, retrievedTxs)
}
