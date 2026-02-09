package extended_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended"
	extendedmock "github.com/onflow/flow-go/module/state_synchronization/indexer/extended/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

func newTestDB(t *testing.T) storage.DB {
	pdb, dbDir := unittest.TempPebbleDB(t)
	db := pebbleimpl.ToDB(pdb)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})
	return db
}

func newBackfillStores(t *testing.T) (storage.Blocks, storage.Collections, storage.Events) {
	blocks := storagemock.NewBlocks(t)
	collections := storagemock.NewCollections(t)
	events := storagemock.NewEvents(t)

	blocks.
		On("ByHeight", mock.AnythingOfType("uint64")).
		Return(func(height uint64) (*flow.Block, error) {
			block := unittest.BlockFixture(func(b *flow.Block) {
				b.Height = height
				b.Payload.Guarantees = nil
			})
			return block, nil
		}).
		Maybe()

	events.
		On("ByBlockID", mock.AnythingOfType("flow.Identifier")).
		Return([]flow.Event{}, nil).
		Maybe()

	return blocks, collections, events
}

func TestExtendedIndexer_AllLive(t *testing.T) {
	db := newTestDB(t)
	blocks, collections, events := newBackfillStores(t)

	startHeight := uint64(10)
	idx1 := extendedmock.NewIndexer(t)
	idx2 := extendedmock.NewIndexer(t)

	idx1.On("LatestIndexedHeight").Return(startHeight, nil).Once()
	idx2.On("LatestIndexedHeight").Return(startHeight+5, nil).Once()

	idx1.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == startHeight+1
	}), mock.Anything).Return(nil).Once()
	idx2.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == startHeight+1
	}), mock.Anything).Return(nil).Once()

	// 1 constructor + 1 live IndexBlockData = 2
	idx1.On("Name").Return("a")
	idx2.On("Name").Return("b")

	ext, err := extended.NewExtendedIndexer(
		zerolog.Nop(),
		db,
		[]extended.Indexer{idx1, idx2},
		metrics.NewNoopCollector(),
		extended.DefaultBackfillDelay,
		extended.DefaultBackfillMaxWorkers,
		flow.Testnet,
		blocks,
		collections,
		events,
		startHeight,
		storage.NewTestingLockManager(),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, ext.Backfill(ctx))

	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(startHeight+1))
	err = ext.IndexBlockData(header, nil, nil)
	require.NoError(t, err)
}

func TestExtendedIndexer_AllBackfilling(t *testing.T) {
	db := newTestDB(t)
	blocks, collections, events := newBackfillStores(t)

	startHeight := uint64(5)
	idx1 := extendedmock.NewIndexer(t)
	idx2 := extendedmock.NewIndexer(t)

	idx1.On("LatestIndexedHeight").Return(uint64(1), nil).Once()
	idx1.On("LatestIndexedHeight").Return(uint64(1), nil).Once()
	idx1.On("LatestIndexedHeight").Return(uint64(1), nil).Once()
	idx1.On("LatestIndexedHeight").Return(uint64(2), nil).Once()
	idx1.On("LatestIndexedHeight").Return(uint64(3), nil).Once()
	idx1.On("LatestIndexedHeight").Return(uint64(4), nil).Once()
	idx1.On("LatestIndexedHeight").Return(uint64(5), nil).Once()

	idx2.On("LatestIndexedHeight").Return(uint64(3), nil).Once()
	idx2.On("LatestIndexedHeight").Return(uint64(3), nil).Once()
	idx2.On("LatestIndexedHeight").Return(uint64(3), nil).Once()
	idx2.On("LatestIndexedHeight").Return(uint64(4), nil).Once()
	idx2.On("LatestIndexedHeight").Return(uint64(5), nil).Once()

	idx1.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 2
	}), mock.Anything).Return(nil).Once()
	idx1.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 3
	}), mock.Anything).Return(nil).Once()
	idx1.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 4
	}), mock.Anything).Return(nil).Once()
	idx1.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 5
	}), mock.Anything).Return(nil).Once()
	idx1.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == startHeight+1
	}), mock.Anything).Return(nil).Once()

	idx2.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 4
	}), mock.Anything).Return(nil).Once()
	idx2.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 5
	}), mock.Anything).Return(nil).Once()
	idx2.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == startHeight+1
	}), mock.Anything).Return(nil).Once()

	// 1 constructor + 4 backfill blocks + 1 live IndexBlockData = 6
	idx1.On("Name").Return("a")
	// 1 constructor + 2 backfill blocks + 1 live IndexBlockData = 4
	idx2.On("Name").Return("b")

	ext, err := extended.NewExtendedIndexer(
		zerolog.Nop(),
		db,
		[]extended.Indexer{idx1, idx2},
		metrics.NewNoopCollector(),
		extended.DefaultBackfillDelay,
		extended.DefaultBackfillMaxWorkers,
		flow.Testnet,
		blocks,
		collections,
		events,
		startHeight,
		storage.NewTestingLockManager(),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, ext.Backfill(ctx))

	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(startHeight+1))
	err = ext.IndexBlockData(header, nil, nil)
	require.NoError(t, err)
}

func TestExtendedIndexer_SplitLiveAndBackfill(t *testing.T) {
	db := newTestDB(t)
	blocks, collections, events := newBackfillStores(t)

	startHeight := uint64(7)
	live := extendedmock.NewIndexer(t)
	backfill := extendedmock.NewIndexer(t)

	live.On("LatestIndexedHeight").Return(startHeight, nil).Once()
	backfill.On("LatestIndexedHeight").Return(uint64(2), nil).Once()
	backfill.On("LatestIndexedHeight").Return(uint64(2), nil).Once()
	backfill.On("LatestIndexedHeight").Return(uint64(2), nil).Once()
	backfill.On("LatestIndexedHeight").Return(uint64(3), nil).Once()
	backfill.On("LatestIndexedHeight").Return(uint64(4), nil).Once()
	backfill.On("LatestIndexedHeight").Return(uint64(5), nil).Once()
	backfill.On("LatestIndexedHeight").Return(uint64(6), nil).Once()
	backfill.On("LatestIndexedHeight").Return(uint64(7), nil).Once()
	backfill.On("LatestIndexedHeight").Return(uint64(8), nil).Once()

	live.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == startHeight+1
	}), mock.Anything).Return(nil).Once()
	live.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == startHeight+2
	}), mock.Anything).Return(nil).Once()

	backfill.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 3
	}), mock.Anything).Return(nil).Once()
	backfill.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 4
	}), mock.Anything).Return(nil).Once()
	backfill.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 5
	}), mock.Anything).Return(nil).Once()
	backfill.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 6
	}), mock.Anything).Return(nil).Once()
	backfill.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 7
	}), mock.Anything).Return(nil).Once()
	backfill.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 8
	}), mock.Anything).Return(nil).Once()
	backfill.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == startHeight+2
	}), mock.Anything).Return(nil).Once()

	// 1 constructor + 2 live IndexBlockData = 3
	live.On("Name").Return("live")
	// 1 constructor + 6 backfill blocks + 1 live IndexBlockData (after promotion) = 8
	backfill.On("Name").Return("backfill")

	ext, err := extended.NewExtendedIndexer(
		zerolog.Nop(),
		db,
		[]extended.Indexer{live, backfill},
		metrics.NewNoopCollector(),
		extended.DefaultBackfillDelay,
		extended.DefaultBackfillMaxWorkers,
		flow.Testnet,
		blocks,
		collections,
		events,
		startHeight,
		storage.NewTestingLockManager(),
	)
	require.NoError(t, err)

	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(startHeight+1))
	err = ext.IndexBlockData(header, nil, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, ext.Backfill(ctx))

	header = unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(startHeight+2))
	err = ext.IndexBlockData(header, nil, nil)
	require.NoError(t, err)
}

func TestExtendedIndexer_BackfillPromotion(t *testing.T) {
	db := newTestDB(t)
	blocks, collections, events := newBackfillStores(t)

	startHeight := uint64(4)
	idx := extendedmock.NewIndexer(t)

	idx.On("LatestIndexedHeight").Return(uint64(1), nil).Once()
	idx.On("LatestIndexedHeight").Return(uint64(1), nil).Once()
	idx.On("LatestIndexedHeight").Return(uint64(1), nil).Once()
	idx.On("LatestIndexedHeight").Return(uint64(2), nil).Once()
	idx.On("LatestIndexedHeight").Return(uint64(3), nil).Once()
	idx.On("LatestIndexedHeight").Return(uint64(4), nil).Once()

	idx.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 2
	}), mock.Anything).Return(nil).Once()
	idx.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 3
	}), mock.Anything).Return(nil).Once()
	idx.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 4
	}), mock.Anything).Return(nil).Once()
	idx.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == startHeight+1 // handled by the live indexer
	}), mock.Anything).Return(nil).Once()

	// 1 constructor + 3 backfill blocks + 1 live IndexBlockData (after promotion) = 5
	idx.On("Name").Return("b")

	ext, err := extended.NewExtendedIndexer(
		zerolog.Nop(),
		db,
		[]extended.Indexer{idx},
		metrics.NewNoopCollector(),
		extended.DefaultBackfillDelay,
		extended.DefaultBackfillMaxWorkers,
		flow.Testnet,
		blocks,
		collections,
		events,
		startHeight,
		storage.NewTestingLockManager(),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// backfill runs and completes successfully
	require.NoError(t, ext.Backfill(ctx))

	// index the first block. should be handled by IndexBlockData
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(startHeight+1))
	err = ext.IndexBlockData(header, nil, nil)
	require.NoError(t, err)
}

func TestExtendedIndexer_ErrorCases(t *testing.T) {
	db := newTestDB(t)
	blocks, collections, events := newBackfillStores(t)

	startHeight := uint64(3)
	idx1 := extendedmock.NewIndexer(t)
	idx2 := extendedmock.NewIndexer(t)

	idx1.On("LatestIndexedHeight").Return(startHeight, nil).Once()
	idx2.On("LatestIndexedHeight").Return(startHeight, nil).Once()

	// 1 constructor only (IndexBlockData errors before Name is called)
	idx1.On("Name").Return("a")
	idx2.On("Name").Return("b")

	indexerFailureError := errors.New("indexer failed")
	idx1.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == startHeight+1
	}), mock.Anything).Return(indexerFailureError).Once()

	ext, err := extended.NewExtendedIndexer(
		zerolog.Nop(),
		db,
		[]extended.Indexer{idx1, idx2},
		metrics.NewNoopCollector(),
		extended.DefaultBackfillDelay,
		extended.DefaultBackfillMaxWorkers,
		flow.Testnet,
		blocks,
		collections,
		events,
		startHeight,
		storage.NewTestingLockManager(),
	)
	require.NoError(t, err)

	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(startHeight+1))
	err = ext.IndexBlockData(header, nil, nil)
	assert.ErrorIs(t, err, indexerFailureError)

	header = unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(startHeight+3))
	err = ext.IndexBlockData(header, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot index future height")
}

func TestExtendedIndexer_BackfillErrors(t *testing.T) {
	db := newTestDB(t)
	blocks, collections, events := newBackfillStores(t)

	startHeight := uint64(5)
	idx := extendedmock.NewIndexer(t)

	idx.On("LatestIndexedHeight").Return(uint64(2), nil).Once()
	idx.On("LatestIndexedHeight").Return(uint64(2), nil).Once()
	idx.On("LatestIndexedHeight").Return(uint64(2), nil).Once()

	// 1 constructor only (backfill errors before Name is called for metrics)
	idx.On("Name").Return("a")

	indexerFailureError := errors.New("backfill failed")
	idx.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 3
	}), mock.Anything).Return(indexerFailureError).Once()

	ext, err := extended.NewExtendedIndexer(
		zerolog.Nop(),
		db,
		[]extended.Indexer{idx},
		metrics.NewNoopCollector(),
		extended.DefaultBackfillDelay,
		extended.DefaultBackfillMaxWorkers,
		flow.Testnet,
		blocks,
		collections,
		events,
		startHeight,
		storage.NewTestingLockManager(),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = ext.Backfill(ctx)
	assert.ErrorIs(t, err, indexerFailureError)
}

func TestExtendedIndexer_BackfillAlreadyExists(t *testing.T) {
	db := newTestDB(t)
	blocks, collections, events := newBackfillStores(t)

	startHeight := uint64(4)
	idx := extendedmock.NewIndexer(t)

	idx.On("LatestIndexedHeight").Return(uint64(1), nil).Once()
	idx.On("LatestIndexedHeight").Return(uint64(1), nil).Once()
	idx.On("LatestIndexedHeight").Return(uint64(1), nil).Once()
	idx.On("LatestIndexedHeight").Return(uint64(2), nil).Once()
	idx.On("LatestIndexedHeight").Return(uint64(3), nil).Once()
	idx.On("LatestIndexedHeight").Return(uint64(4), nil).Once()

	idx.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 2
	}), mock.Anything).Return(extended.ErrAlreadyIndexed).Once()
	idx.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 3
	}), mock.Anything).Return(nil).Once()
	idx.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == 4
	}), mock.Anything).Return(nil).Once()
	idx.On("IndexBlockData", mock.Anything, mock.MatchedBy(func(data extended.BlockData) bool {
		return data.Header.Height == startHeight+1
	}), mock.Anything).Return(nil).Once()

	// 1 constructor + 3 backfill blocks (including ErrAlreadyIndexed) + 1 live IndexBlockData = 5
	idx.On("Name").Return("a")

	ext, err := extended.NewExtendedIndexer(
		zerolog.Nop(),
		db,
		[]extended.Indexer{idx},
		metrics.NewNoopCollector(),
		extended.DefaultBackfillDelay,
		extended.DefaultBackfillMaxWorkers,
		flow.Testnet,
		blocks,
		collections,
		events,
		startHeight,
		storage.NewTestingLockManager(),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, ext.Backfill(ctx))

	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(startHeight+1))
	err = ext.IndexBlockData(header, nil, nil)
	require.NoError(t, err)
}

func TestExtendedIndexer_FutureHeight(t *testing.T) {
	db := newTestDB(t)
	blocks, collections, events := newBackfillStores(t)

	startHeight := uint64(10)
	idx := extendedmock.NewIndexer(t)

	idx.On("LatestIndexedHeight").Return(startHeight, nil).Once()
	// 1 constructor only (future height check fails before reaching indexers)
	idx.On("Name").Return("a")

	ext, err := extended.NewExtendedIndexer(
		zerolog.Nop(),
		db,
		[]extended.Indexer{idx},
		metrics.NewNoopCollector(),
		extended.DefaultBackfillDelay,
		extended.DefaultBackfillMaxWorkers,
		flow.Testnet,
		blocks,
		collections,
		events,
		startHeight,
		storage.NewTestingLockManager(),
	)
	require.NoError(t, err)

	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(startHeight+2))
	err = ext.IndexBlockData(header, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("cannot index future height %d", startHeight+2))
}
