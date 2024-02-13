package badger

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TesKeyValueStoreStorage tests if the KV store is stored, retrieved and indexed correctly
func TestKeyValueStoreStorage(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := NewProtocolKVStore(metrics, db, DefaultProtocolKVStoreCacheSize, DefaultProtocolKVStoreByBlockIDCacheSize)

		expected := &storage.KeyValueStoreData{
			Version: 2,
			Data:    unittest.RandomBytes(32),
		}
		stateID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()

		// store protocol state and auxiliary info
		err := transaction.Update(db, func(tx *transaction.Tx) error {
			err := store.StoreTx(stateID, expected)(tx)
			require.NoError(t, err)
			return store.IndexTx(blockID, stateID)(tx)
		})
		require.NoError(t, err)

		// fetch protocol state
		actual, err := store.ByID(stateID)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// fetch protocol state by block ID
		actualByBlockID, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		assert.Equal(t, expected, actualByBlockID)
	})
}
