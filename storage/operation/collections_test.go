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

func TestCollections(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		expected := unittest.CollectionFixture(2).Light()

		t.Run("Retrieve nonexistant", func(t *testing.T) {
			reader, err := db.Reader()
			require.NoError(t, err)

			var actual flow.LightCollection
			err = operation.RetrieveCollection(reader, expected.ID(), &actual)
			assert.Error(t, err)
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("Save", func(t *testing.T) {
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertCollection(rw.Writer(), &expected)
			})
			require.NoError(t, err)

			reader, err := db.Reader()
			require.NoError(t, err)

			var actual flow.LightCollection
			err = operation.RetrieveCollection(reader, expected.ID(), &actual)
			assert.NoError(t, err)

			assert.Equal(t, expected, actual)
		})

		t.Run("Remove", func(t *testing.T) {
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.RemoveCollection(rw.Writer(), expected.ID())
			})
			require.NoError(t, err)

			reader, err := db.Reader()
			require.NoError(t, err)

			var actual flow.LightCollection
			err = operation.RetrieveCollection(reader, expected.ID(), &actual)
			assert.Error(t, err)
			assert.ErrorIs(t, err, storage.ErrNotFound)

			// Remove again should not error
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.RemoveCollection(rw.Writer(), expected.ID())
			})
			require.NoError(t, err)
		})

		t.Run("Index and lookup", func(t *testing.T) {
			expected := unittest.CollectionFixture(1).Light()
			blockID := unittest.IdentifierFixture()

			_ = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				err := operation.UpsertCollection(rw.Writer(), &expected)
				assert.NoError(t, err)
				err = operation.IndexCollectionPayload(rw.Writer(), blockID, expected.Transactions)
				assert.NoError(t, err)
				return nil
			})

			reader, err := db.Reader()
			require.NoError(t, err)

			var actual flow.LightCollection
			err = operation.LookupCollectionPayload(reader, blockID, &actual.Transactions)
			assert.NoError(t, err)
			assert.Equal(t, expected, actual)
		})

		t.Run("Index and lookup by transaction ID", func(t *testing.T) {
			expected := unittest.IdentifierFixture()
			transactionID := unittest.IdentifierFixture()
			actual := flow.Identifier{}

			_ = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				err := operation.UnsafeIndexCollectionByTransaction(rw.Writer(), transactionID, expected)
				assert.NoError(t, err)
				return nil
			})

			reader, err := db.Reader()
			require.NoError(t, err)

			err = operation.LookupCollectionByTransaction(reader, transactionID, &actual)
			assert.NoError(t, err)

			assert.Equal(t, expected, actual)
		})
	})
}
