package operation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestRetrieveAllTxResultsForBlock verifies the working of persisting, indexing and retrieving
// [flow.LightTransactionResult] by block, transaction ID, and transaction index.
func TestRetrieveAllTxResultsForBlock(t *testing.T) {
	t.Run("looking up transaction results for unknown block yields empty list", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			unknownBlockID := unittest.IdentifierFixture()
			transactionResults := make([]flow.LightTransactionResult, 0)

			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.LookupLightTransactionResultsByBlockIDUsingIndex(rw.GlobalReader(), unknownBlockID, &transactionResults)
			})
			require.NoError(t, err)
			require.Empty(t, transactionResults)
		})
	})
}
