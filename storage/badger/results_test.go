package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestResultsRemoveNonExist(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store := bstorage.NewExecutionResults(db)

		blockID := unittest.IdentifierFixture()
		err := store.RemoveByBlockID(blockID)
		require.NoError(t, err)
	})
}

func TestResultsStoreRemove(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store := bstorage.NewExecutionResults(db)

		result := unittest.ExecutionResultFixture()
		blockID := unittest.IdentifierFixture()
		err := store.Store(result)
		require.NoError(t, err)

		err = store.Index(blockID, result.ID())
		require.NoError(t, err)

		err = store.RemoveByBlockID(blockID)
		require.NoError(t, err)

		_, err = store.ByBlockID(blockID)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound))
	})
}
