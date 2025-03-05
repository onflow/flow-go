package operation_test

import (
	"errors"
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
			var actual flow.LightCollection
			err := operation.RetrieveCollection(db.Reader(), expected.ID(), &actual)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, storage.ErrNotFound))
		})

		t.Run("Save", func(t *testing.T) {
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertCollection(rw.Writer(), &expected)
			})
			require.NoError(t, err)

			var actual flow.LightCollection
			err = operation.RetrieveCollection(db.Reader(), expected.ID(), &actual)
			assert.NoError(t, err)

			assert.Equal(t, expected, actual)
		})

		t.Run("Remove", func(t *testing.T) {
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.RemoveCollection(rw.Writer(), expected.ID())
			})
			require.NoError(t, err)

			var actual flow.LightCollection
			err = operation.RetrieveCollection(db.Reader(), expected.ID(), &actual)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, storage.ErrNotFound))

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
				err := operation.InsertCollection(rw.Writer(), &expected)
				assert.NoError(t, err)
				err = operation.IndexCollectionPayload(rw.Writer(), blockID, expected.Transactions)
				assert.NoError(t, err)
				return nil
			})

			var actual flow.LightCollection
			err := operation.LookupCollectionPayload(db.Reader(), blockID, &actual.Transactions)
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

			err := operation.RetrieveCollectionID(db.Reader(), transactionID, &actual)
			assert.NoError(t, err)

			assert.Equal(t, expected, actual)
		})
	})
}
