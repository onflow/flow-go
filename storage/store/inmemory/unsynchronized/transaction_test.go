package unsynchronized

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactions_HappyPath(t *testing.T) {
	txStore := NewTransactions()

	// Store the transaction
	tx := unittest.TransactionBodyFixture()
	err := txStore.Store(&tx)
	require.NoError(t, err, "storing transaction should not return an error")

	// Retrieve the transaction by ID
	retrievedTx, err := txStore.ByID(tx.ID())
	require.NoError(t, err, "retrieving stored transaction should not return an error")
	require.NotNil(t, retrievedTx, "retrieved transaction should not be nil")

	// Ensure the retrieved transaction matches the stored one
	require.Equal(t, &tx, retrievedTx, "retrieved transaction should match the stored transaction")
}
