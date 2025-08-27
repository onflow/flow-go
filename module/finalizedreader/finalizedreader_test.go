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
		err := headers.Store(unittest.ProposalHeaderFromHeader(block.ToHeader()))
		require.NoError(t, err)

		// index the header
		err = db.Update(operation.IndexBlockHeight(block.Height, block.ID()))
		require.NoError(t, err)

		// verify is able to reader the finalized block ID
		reader := NewFinalizedReader(headers, block.Height)
		finalized, err := reader.FinalizedBlockIDAtHeight(block.Height)
		require.NoError(t, err)
		require.Equal(t, block.ID(), finalized)

		// verify is able to return storage.NotFound when the height is not finalized
		_, err = reader.FinalizedBlockIDAtHeight(block.Height + 1)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound), err)

		// finalize one more block
		block2 := unittest.BlockWithParentFixture(block.ToHeader())
		require.NoError(t, headers.Store(unittest.ProposalHeaderFromHeader(block2.ToHeader())))
		err = db.Update(operation.IndexBlockHeight(block2.Height, block2.ID()))
		require.NoError(t, err)
		reader.BlockFinalized(block2.ToHeader())

		// should be able to retrieve the block
		finalized, err = reader.FinalizedBlockIDAtHeight(block2.Height)
		require.NoError(t, err)
		require.Equal(t, block2.ID(), finalized)

		// should noop and no panic
		reader.BlockProcessable(block.ToHeader(), block2.ParentQC())
	})
}
