package inmemory

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactions_HappyPath(t *testing.T) {
	tx := unittest.TransactionBodyFixture()
	txStore := NewTransactions([]*flow.TransactionBody{&tx})

	// Retrieve the transaction by ID
	retrievedTx, err := txStore.ByID(tx.ID())
	require.NoError(t, err, "retrieving stored transaction should not return an error")
	require.NotNil(t, retrievedTx, "retrieved transaction should not be nil")

	// Ensure the retrieved transaction matches the stored one
	require.Equal(t, &tx, retrievedTx, "retrieved transaction should match the stored transaction")
}
