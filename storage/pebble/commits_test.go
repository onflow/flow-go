package pebble

import (
	"errors"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestCommitsStoreAndRetrieve tests that a commit can be stored, retrieved and attempted to be stored again without an error
func TestCommitsStoreAndRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		store := NewCommits(metrics, db)

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
