package operation

import (
	"github.com/onflow/flow-go/storage"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInsertProtocolKVStore tests if basic badger operations on ProtocolKVStore work as expected.
func TestInsertProtocolKVStore(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := &storage.KeyValueStoreData{
			Version: 2,
			Data:    unittest.RandomBytes(32),
		}

		kvStoreStateID := unittest.IdentifierFixture()
		err := db.Update(InsertProtocolKVStore(kvStoreStateID, expected))
		require.Nil(t, err)

		var actual storage.KeyValueStoreData
		err = db.View(RetrieveProtocolKVStore(kvStoreStateID, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)

		blockID := unittest.IdentifierFixture()
		err = db.Update(IndexProtocolKVStore(blockID, kvStoreStateID))
		require.Nil(t, err)

		var actualProtocolKVStoreID flow.Identifier
		err = db.View(LookupProtocolKVStore(blockID, &actualProtocolKVStoreID))
		require.Nil(t, err)

		assert.Equal(t, kvStoreStateID, actualProtocolKVStoreID)
	})
}
