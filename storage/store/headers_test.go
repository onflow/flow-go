package store_test

import (
	"sync"
	"testing"

	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestHeaderStoreRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		headers := store.NewHeaders(metrics, db)

		block := unittest.BlockFixture()

		// store header
		err := headers.Store(block.Header)
		require.NoError(t, err)

		// index the header
		indexing := &sync.Mutex{}
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexBlockHeight(indexing, rw, block.Header.Height, block.ID())
		})
		require.NoError(t, err)

		// retrieve header by height
		actual, err := headers.ByHeight(block.Header.Height)
		require.NoError(t, err)
		require.Equal(t, block.Header, actual)
	})
}

func TestHeaderRetrieveWithoutStore(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		headers := store.NewHeaders(metrics, db)

		header := unittest.BlockHeaderFixture()

		// retrieve header by height, should err as not store before height
		_, err := headers.ByHeight(header.Height)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}
