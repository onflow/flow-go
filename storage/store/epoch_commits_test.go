package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEpochCommitStoreAndRetrieve tests that a commit can be sd, retrieved and attempted to be sd again without an error
func TestEpochCommitStoreAndRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		s := store.NewEpochCommits(metrics, db)

		// attempt to get a invalid commit
		_, err := s.ByID(unittest.IdentifierFixture())
		assert.ErrorIs(t, err, storage.ErrNotFound)

		// s a commit in db
		expected := unittest.EpochCommitFixture()
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return s.BatchStore(rw, expected)
		})
		require.NoError(t, err)

		// retrieve the commit by ID
		actual, err := s.ByID(expected.ID())
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// test storing same epoch commit
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return s.BatchStore(rw, expected)
		})
		require.NoError(t, err)
	})
}
