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
	err := ltx.Store(block.Hash(), txResults)
	require.NoError(t, err)

	// Retrieve by BlockID and TransactionID
	retrievedTx, err := ltx.ByBlockIDTransactionID(block.Hash(), txResults[0].TransactionID)
	require.NoError(t, err)
	assert.Equal(t, &txResults[0], retrievedTx)

	// Retrieve by BlockID and Index
	retrievedTxByIndex, err := ltx.ByBlockIDTransactionIndex(block.Hash(), 0)
	require.NoError(t, err)
	assert.Equal(t, &txResults[0], retrievedTxByIndex)

	// Retrieve by BlockID
	retrievedTxs, err := ltx.ByBlockID(block.Hash())
	require.NoError(t, err)
	assert.Len(t, retrievedTxs, len(txResults))
	assert.Equal(t, txResults, retrievedTxs)

	// Extract structured data
	ltxs := ltx.Data()
	assert.Len(t, ltxs, len(txResults))
	assert.ElementsMatch(t, txResults, ltxs)
}
