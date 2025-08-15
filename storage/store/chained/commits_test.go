package chained

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCommitsOnlyFirstHave(t *testing.T) {
	unittest.RunWithBadgerDBAndPebbleDB(t, func(bdb *badger.DB, pdb *pebble.DB) {
		bcommits := store.NewCommits(metrics.NewNoopCollector(), badgerimpl.ToDB(bdb))
		pcommits := store.NewCommits(metrics.NewNoopCollector(), pebbleimpl.ToDB(pdb))

		blockID := unittest.IdentifierFixture()
		commit := unittest.StateCommitmentFixture()

		chained := NewCommits(pcommits, bcommits)

		// not found
		_, err := chained.ByBlockID(blockID)
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)

		require.NoError(t, pebbleimpl.ToDB(pdb).WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexStateCommitment(rw.Writer(), blockID, commit)
		}))
		// only stored in first
		actual, err := chained.ByBlockID(blockID)
		require.NoError(t, err)

		require.Equal(t, commit, actual)
	})
}

func TestCommitsOnlySecondHave(t *testing.T) {
	unittest.RunWithBadgerDBAndPebbleDB(t, func(bdb *badger.DB, pdb *pebble.DB) {
		bcommits := store.NewCommits(metrics.NewNoopCollector(), badgerimpl.ToDB(bdb))
		pcommits := store.NewCommits(metrics.NewNoopCollector(), pebbleimpl.ToDB(pdb))

		blockID := unittest.IdentifierFixture()
		commit := unittest.StateCommitmentFixture()

		chained := NewCommits(pcommits, bcommits)
		// only stored in second
		require.NoError(t, badgerimpl.ToDB(bdb).WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexStateCommitment(rw.Writer(), blockID, commit)
		}))
		actual, err := chained.ByBlockID(blockID)
		require.NoError(t, err)

		require.Equal(t, commit, actual)
	})
}
