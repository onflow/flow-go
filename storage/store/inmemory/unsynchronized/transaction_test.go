package unsynchronized

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
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

func TestTransactions_Persist(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		txStore := NewTransactions()
		tx := unittest.TransactionBodyFixture()

		// Store transaction
		err := txStore.Store(&tx)
		require.NoError(t, err, "storing transaction should not return an error")
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return txStore.AddToBatch(rw)
		}))

		// Get light transaction
		var actualTx flow.TransactionBody
		err = operation.RetrieveTransaction(db.Reader(), tx.ID(), &actualTx)
		require.NoError(t, err)
		require.Equal(t, tx, actualTx)
	})
}
