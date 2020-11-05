package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCommitsReadNonExist(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := &metrics.NoopCollector{}
		store := bstorage.NewCommits(metrics, db)

		blockID := unittest.IdentifierFixture()
		_, err := store.ByBlockID(blockID)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

func TestCommitsStoreRead(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := &metrics.NoopCollector{}
		store := bstorage.NewCommits(metrics, db)

		commit := unittest.StateCommitmentFixture()
		blockID := unittest.IdentifierFixture()
		err := store.Store(blockID, commit)
		require.NoError(t, err)

		actual, err := store.ByBlockID(blockID)
		require.NoError(t, err)

		require.Equal(t, commit, actual)
	})
}

func TestCommitsRemoveNonExist(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := &metrics.NoopCollector{}
		store := bstorage.NewCommits(metrics, db)

		blockID := unittest.IdentifierFixture()
		err := store.RemoveByBlockID(blockID)
		require.NoError(t, err)
	})
}

func TestCommitsStoreRemove(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := &metrics.NoopCollector{}
		store := bstorage.NewCommits(metrics, db)

		commit := unittest.StateCommitmentFixture()
		blockID := unittest.IdentifierFixture()
		err := store.Store(blockID, commit)
		require.NoError(t, err)

		err = store.RemoveByBlockID(blockID)
		require.NoError(t, err)

		storeWithoutCache := bstorage.NewCommits(metrics, db)
		// using store.ByBlockID will not work, because the removed commitment
		// was still cached
		_, err = storeWithoutCache.ByBlockID(blockID)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound))
	})
}
