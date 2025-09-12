package operation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSporkID_InsertRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		sporkRootBlockID := unittest.IdentifierFixture()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexSporkRootBlock(rw.Writer(), sporkRootBlockID)
		})
		require.NoError(t, err)

		var actual flow.Identifier
		err = operation.RetrieveSporkRootBlockID(db.Reader(), &actual)
		require.NoError(t, err)

		assert.Equal(t, sporkRootBlockID, actual)
	})
}
