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

func TestBlockChildrenIndexUpdateLookup(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		blockID := unittest.IdentifierFixture()
		childrenIDs := unittest.IdentifierListFixture(8)
		var retrievedIDs flow.IdentifierList

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertBlockChildren(rw.Writer(), blockID, childrenIDs)
		})
		require.NoError(t, err)
		err = operation.RetrieveBlockChildren(db.Reader(), blockID, &retrievedIDs)
		require.NoError(t, err)
		assert.Equal(t, childrenIDs, retrievedIDs)

		altIDs := unittest.IdentifierListFixture(4)
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertBlockChildren(rw.Writer(), blockID, altIDs)
		})
		require.NoError(t, err)
		err = operation.RetrieveBlockChildren(db.Reader(), blockID, &retrievedIDs)
		require.NoError(t, err)
		assert.Equal(t, altIDs, retrievedIDs)
	})
}
