package store_test

import (
	"errors"
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

func TestCommitStoreAndRemove(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := store.NewCommits(metrics, db)

		// Create and store a commit
		blockID := unittest.IdentifierFixture()
		expected := unittest.StateCommitmentFixture()
		err := store.Store(blockID, expected)
		require.NoError(t, err)

		// Ensure it exists
		commit, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		assert.Equal(t, expected, commit)

		// Remove it
		err = store.RemoveByBlockID(blockID)
		require.NoError(t, err)

		// Ensure it no longer exists
		_, err = store.ByBlockID(blockID)
		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})
}
