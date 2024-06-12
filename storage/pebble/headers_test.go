package pebble_test

import (
	"errors"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestHeaderStoreRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		headers := pebblestorage.NewHeaders(metrics, db)

		block := unittest.BlockFixture()

		// store header
		err := headers.Store(block.Header)
		require.NoError(t, err)

		// index the header
		err = operation.IndexBlockHeight(block.Header.Height, block.ID())(db)
		require.NoError(t, err)

		// retrieve header by height
		actual, err := headers.ByHeight(block.Header.Height)
		require.NoError(t, err)
		require.Equal(t, block.Header, actual)
	})
}

func TestHeaderRetrieveWithoutStore(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		headers := pebblestorage.NewHeaders(metrics, db)

		header := unittest.BlockHeaderFixture()

		// retrieve header by height, should err as not store before height
		_, err := headers.ByHeight(header.Height)
		require.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

func TestRollbackExecutedBlock(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		headers := pebblestorage.NewHeaders(metrics, db)

		genesis := unittest.GenesisFixture()
		blocks := unittest.ChainFixtureFrom(4, genesis.Header)

		// store executed block
		require.NoError(t, headers.Store(blocks[3].Header))
		require.NoError(t, operation.InsertExecutedBlock(blocks[3].ID())(db))
		var executedBlockID flow.Identifier
		require.NoError(t, operation.RetrieveExecutedBlock(&executedBlockID)(db))
		require.Equal(t, blocks[3].ID(), executedBlockID)

		require.NoError(t, headers.Store(blocks[0].Header))
		require.NoError(t, headers.RollbackExecutedBlock(blocks[0].Header))
		require.NoError(t, operation.RetrieveExecutedBlock(&executedBlockID)(db))
		require.Equal(t, blocks[0].ID(), executedBlockID)
	})
}

func TestIndexedBatch(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		genesis := unittest.GenesisFixture()
		blocks := unittest.ChainFixtureFrom(4, genesis.Header)

		require.NoError(t, operation.InsertExecutedBlock(blocks[3].ID())(db))
		require.NoError(t, operation.InsertExecutedBlock(blocks[3].ID())(db))
	})
}
