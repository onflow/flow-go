package pebble_test

import (
	"errors"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	pebblestorage "github.com/onflow/flow-go/storage/pebble"
)

// TestEpochCommitStoreAndRetrieve tests that a commit can be stored, retrieved and attempted to be stored again without an error
func TestEpochCommitStoreAndRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		store := pebblestorage.NewEpochCommits(metrics, db)

		// attempt to get a invalid commit
		_, err := store.ByID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// store a commit in db
		expected := unittest.EpochCommitFixture()
		err = store.StoreTx(expected)(db)
		require.NoError(t, err)

		// retrieve the commit by ID
		actual, err := store.ByID(expected.ID())
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// test storing same epoch commit
		err = store.StoreTx(expected)(db)
		require.NoError(t, err)
	})
}
