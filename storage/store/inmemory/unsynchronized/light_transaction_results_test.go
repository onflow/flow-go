package unsynchronized

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestLightTransactionResults_HappyPath(t *testing.T) {
	ltx := NewLightTransactionResults()

	// Define block ID and transaction results
	block := unittest.BlockFixture()
	txResults := unittest.LightTransactionResultsFixture(10)

	// Store transaction results
	err := ltx.Store(block.ID(), txResults)
	require.NoError(t, err)

	// Retrieve by BlockID and TransactionID
	retrievedTx, err := ltx.ByBlockIDTransactionID(block.ID(), txResults[0].TransactionID)
	require.NoError(t, err)
	assert.Equal(t, &txResults[0], retrievedTx)

	// Retrieve by BlockID and Index
	retrievedTxByIndex, err := ltx.ByBlockIDTransactionIndex(block.ID(), 0)
	require.NoError(t, err)
	assert.Equal(t, &txResults[0], retrievedTxByIndex)

	// Retrieve by BlockID
	retrievedTxs, err := ltx.ByBlockID(block.ID())
	require.NoError(t, err)
	assert.Len(t, retrievedTxs, len(txResults))
	assert.Equal(t, txResults, retrievedTxs)
}
