package badger_test

import (
	"errors"
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
		err = operation.RetryOnConflict(db.Update, operation.IndexBlockHeight(block.Header.Height, block.ID()))
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
		require.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

func TestTimeStampRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		headers := badgerstorage.NewHeaders(metrics, db)

		header := unittest.BlockHeaderFixture()

		// store header
		err := headers.Store(&header)
		require.NoError(t, err)

		timeStamp, err := headers.TimestampByBlockID(header.ID())
		require.NoError(t, err)
		expectedTimeStamp := header.Timestamp
		require.Equal(t, expectedTimeStamp, timeStamp)
	})
}
