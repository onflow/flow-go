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

// TestInsertProtocolState tests if basic badger operations on EpochProtocolState work as expected.
func TestInsertEpochProtocolState(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		expected := unittest.EpochStateFixture().MinEpochStateEntry

		epochProtocolStateEntryID := expected.ID()
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertEpochProtocolState(rw.Writer(), epochProtocolStateEntryID, expected)
		})
		require.NoError(t, err)

		var actual flow.MinEpochStateEntry
		err = operation.RetrieveEpochProtocolState(db.Reader(), epochProtocolStateEntryID, &actual)
		require.NoError(t, err)

		assert.Equal(t, expected, &actual)

		blockID := unittest.IdentifierFixture()
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexEpochProtocolState(rw.Writer(), blockID, epochProtocolStateEntryID)
		})
		require.NoError(t, err)

		var actualProtocolStateID flow.Identifier
		err = operation.LookupEpochProtocolState(db.Reader(), blockID, &actualProtocolStateID)
		require.NoError(t, err)

		assert.Equal(t, epochProtocolStateEntryID, actualProtocolStateID)
	})
}
