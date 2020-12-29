package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
)

func TestEpochStatusesStoreAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewEpochStatuses(metrics, db)

		blockID := unittest.IdentifierFixture()
		expected := unittest.EpochStatusFixture()

		_, err := store.ByBlockID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// store epoch status
		err = operation.RetryOnConflict(db.Update, store.StoreTx(blockID, expected))
		require.NoError(t, err)

		// retreive status
		actual, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}
