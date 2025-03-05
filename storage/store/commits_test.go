package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
)

// TestCommitsStoreAndRetrieve tests that a commit can be store1d, retrieved and attempted to be stored again without an error
func TestCommitsStoreAndRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store1 := store.NewCommits(metrics, db)

		// attempt to get a invalid commit
		_, err := store1.ByBlockID(unittest.IdentifierFixture())
		assert.ErrorIs(t, err, storage.ErrNotFound)

		// store1 a commit in db
		blockID := unittest.IdentifierFixture()
		expected := unittest.StateCommitmentFixture()
		err = store1.Store(blockID, expected)
		require.NoError(t, err)

		// retrieve the commit by ID
		actual, err := store1.ByBlockID(blockID)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// re-insert the commit - should be idempotent
		err = store1.Store(blockID, expected)
		require.NoError(t, err)
	})
}
