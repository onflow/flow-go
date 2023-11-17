package finalizedreader

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/utils/unittest"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
)

func TestFinalizedReader(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		// prepare the storage.Headers instance
		metrics := metrics.NewNoopCollector()
		headers := badgerstorage.NewHeaders(metrics, db)
		block := unittest.BlockFixture()

		// store header
		err := headers.Store(block.Header)
		require.NoError(t, err)

		// index the header
		err = operation.RetryOnConflict(db.Update, operation.IndexBlockHeight(block.Header.Height, block.ID()))
		require.NoError(t, err)

		// verify is able to reader the finalized block ID
		reader := NewFinalizedReader(headers)
		finalized, err := reader.FinalizedBlockIDAtHeight(block.Header.Height)
		require.NoError(t, err)
		require.Equal(t, block.ID(), finalized)

		// verify is able to return storage.NotFound when the height is not finalized
		_, err = reader.FinalizedBlockIDAtHeight(block.Header.Height + 1)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound), err)
	})
}
