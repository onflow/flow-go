package operation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInstanceParams_InsertRetrieve verifies that InstanceParams can be
// correctly stored and retrieved from the database as a single encoded
// structure.
func TestInstanceParams_InsertRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		instanceParams := operation.EncodableInstanceParams{
			FinalizedRootID:  unittest.IdentifierFixture(),
			SealedRootID:     unittest.IdentifierFixture(),
			SporkRootBlockID: unittest.IdentifierFixture(),
		}

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertInstanceParams(rw, instanceParams)
		})
		require.NoError(t, err)

		var actual operation.EncodableInstanceParams
		err = operation.RetrieveInstanceParams(db.Reader(), &actual)
		require.NoError(t, err)

		require.Equal(t, instanceParams, actual)
	})
}
