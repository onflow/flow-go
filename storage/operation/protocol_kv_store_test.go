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

// TestInsertProtocolKVStore tests if basic badger operations on ProtocolKVStore work as expected.
func TestInsertProtocolKVStore(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		expected := &flow.PSKeyValueStoreData{
			Version: 2,
			Data:    unittest.RandomBytes(32),
		}

		kvStoreStateID := unittest.IdentifierFixture()
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertProtocolKVStore(rw.Writer(), kvStoreStateID, expected)
		})
		require.NoError(t, err)

		var actual flow.PSKeyValueStoreData
		err = operation.RetrieveProtocolKVStore(db.Reader(), kvStoreStateID, &actual)
		require.NoError(t, err)

		assert.Equal(t, expected, &actual)

		blockID := unittest.IdentifierFixture()
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexProtocolKVStore(rw.Writer(), blockID, kvStoreStateID)
		})
		require.NoError(t, err)

		var actualProtocolKVStoreID flow.Identifier
		err = operation.LookupProtocolKVStore(db.Reader(), blockID, &actualProtocolKVStoreID)
		require.NoError(t, err)

		assert.Equal(t, kvStoreStateID, actualProtocolKVStoreID)
	})
}
