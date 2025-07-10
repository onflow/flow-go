package badger_test

import (
	"testing"

	"github.com/onflow/flow-go/storage/badger/operation"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
)

func TestHeaderStoreRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		headers := badgerstorage.NewHeaders(metrics, db)

		block := unittest.BlockFixture()

		// store header
		err := headers.Store(block.Header)
		require.NoError(t, err)

		// index the header
		err = operation.RetryOnConflict(db.Update, operation.IndexFinalizedBlockByHeight(block.Header.Height, block.ID()))
		require.NoError(t, err)

		// retrieve header by height
		actual, err := headers.ByHeight(block.Header.Height)
		require.NoError(t, err)
		require.Equal(t, block.Header, actual)
	})
}

func TestHeaderRetrieveWithoutStore(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		headers := badgerstorage.NewHeaders(metrics, db)

		header := unittest.BlockHeaderFixture()

		// retrieve header by height, should err as not store before height
		_, err := headers.ByHeight(header.Height)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestHeaderGetByView(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		headers := badgerstorage.NewHeaders(metrics, db)

		block := unittest.BlockFixture()

		// store header
		err := headers.Store(block.Header)
		require.NoError(t, err)

		// Verify storing the block does not index the view automatically. It would not be safe to
		// do that, since a byzantine leader might produce multiple proposals for the same view.
		_, err = headers.BlockIDByView(block.Header.View)
		require.ErrorIs(t, err, storage.ErrNotFound)

		// Though, HotStuff guarantees that for each view at most one block is certified. Hence, we can index
		// a block by its view _only_ once the block is certified. When a block is certified is decided by the
		// higher-level consensus logic, which then triggers the indexing of the block by its view via a
		// dedicated method call:
		require.NoError(t, db.Update(operation.IndexCertifiedBlockByView(block.Header.View, block.ID())))

		// verify that the block ID can be retrieved by view
		indexedID, err := headers.BlockIDByView(block.Header.View)
		require.NoError(t, err)
		require.Equal(t, block.ID(), indexedID)

		// verify that the block header can be retrieved by view
		actual, err := headers.ByView(block.Header.View)
		require.NoError(t, err)
		require.Equal(t, block.Header, actual)
	})
}
