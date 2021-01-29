package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
)

func TestIndexStoreRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewIndex(metrics, db)

		blockID := unittest.IdentifierFixture()
		expected := unittest.IndexFixture()

		// retreive without store
		_, err := store.ByBlockID(blockID)
		require.True(t, errors.Is(err, storage.ErrNotFound))

		// store index
		err = store.Store(blockID, expected)
		require.NoError(t, err)

		// retreive index
		actual, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}
