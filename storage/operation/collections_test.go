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
			var actual flow.LightCollection
			err := operation.RetrieveCollection(db.Reader(), expected.ID(), &actual)
			assert.Error(t, err)
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("Save", func(t *testing.T) {
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertCollection(rw.Writer(), &expected)
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
			assert.ErrorIs(t, err, storage.ErrNotFound)

			// Remove again should not error
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.RemoveCollection(rw.Writer(), expected.ID())
			})
			require.NoError(t, err)
		})
	})
}
