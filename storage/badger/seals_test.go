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

func TestRetrieveWithoutStore(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewSeals(metrics, db)

		_, err := store.ByID(unittest.IdentifierFixture())
		require.True(t, errors.Is(err, storage.ErrNotFound))

		_, err = store.ByBlockID(unittest.IdentifierFixture())
		require.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

func TestSealStoreRetrieveByBlockID(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewSeals(metrics, db)

		expected := unittest.SealFixture()

		// store seal
		err := store.Store(expected)
		require.NoError(t, err)

		// retrieve seal
		seal, err := store.ByID(expected.ID())
		require.NoError(t, err)
		require.Equal(t, expected, seal)

		seal, err = store.ByBlockID(expected.BlockID)
		require.NoError(t, err)
		require.Equal(t, expected, seal)
	})
}
