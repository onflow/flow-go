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
)

// TestCommitsStoreAndRetrieve tests that a commit can be stored, retrieved and attempted to be stored again without an error
func TestCommitsStoreAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewCommits(metrics, db)

		// attempt to get a invalid commit
		_, err := store.ByBlockID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// store a commit in db
		blockID := unittest.IdentifierFixture()
		expected := unittest.StateCommitmentFixture()
		err = store.Store(blockID, expected)
		require.NoError(t, err)

		// retrieve the commit by ID
		actual, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// re-insert the commit - should be idempotent
		err = store.Store(blockID, expected)
		require.NoError(t, err)
	})
}
