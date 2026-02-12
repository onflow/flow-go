package extended_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended"
	extendedmock "github.com/onflow/flow-go/module/state_synchronization/indexer/extended/mock"
	"github.com/onflow/flow-go/state/protocol"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

const testTimeout = 5 * time.Second

// ===== Test Helpers =====

func newTestDB(t *testing.T) storage.DB {
	pdb, dbDir := unittest.TempPebbleDB(t)
	db := pebbleimpl.ToDB(pdb)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})
	return db
}

// mockIndexer wraps the mock with atomic state tracking.
type mockIndexer struct {
	*extendedmock.Indexer
	nextHeight *atomic.Uint64
	done       chan struct{}
}

// newMockIndexer creates a mock indexer using an atomic counter for NextHeight.
// Each successful IndexBlockData call advances the counter to data.Header.Height + 1.
// The done channel is closed when the targetHeight is processed (0 means never).
func newMockIndexer(
	t *testing.T,
	name string,
	startHeight uint64,
	targetHeight uint64,
) *mockIndexer {
	idx := extendedmock.NewIndexer(t)
	nextHeight := atomic.NewUint64(startHeight)
	done := make(chan struct{})
	var doneOnce sync.Once

	idx.On("Name").Return(name)

	idx.On("NextHeight").Return(
		func() uint64 { return nextHeight.Load() },
		func() error { return nil },
	)

	idx.On("IndexBlockData", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			data := args.Get(1).(extended.BlockData)
			nextHeight.Store(data.Header.Height + 1)
			if targetHeight > 0 && data.Header.Height == targetHeight {
				doneOnce.Do(func() { close(done) })
			}
		}).
		Return(nil)

	return &mockIndexer{Indexer: idx, nextHeight: nextHeight, done: done}
}

// newMockState returns a mock protocol.State where Params().SporkRootBlockHeight() returns 0.
func newMockState(t *testing.T) protocol.State {
	params := protocolmock.NewParams(t)
	params.On("SporkRootBlockHeight").Return(uint64(0))

	state := protocolmock.NewState(t)
	state.On("Params").Return(params)
	return state
}

type testSetup struct {
	db          storage.DB
	blocks      *storagemock.Blocks
	collections *storagemock.Collections
	events      *storagemock.Events
	results     *storagemock.LightTransactionResults
}

func newTestSetup(t *testing.T) *testSetup {
	return &testSetup{
		db:          newTestDB(t),
		blocks:      storagemock.NewBlocks(t),
		collections: storagemock.NewCollections(t),
		events:      storagemock.NewEvents(t),
		results:     storagemock.NewLightTransactionResults(t),
	}
}

// configureBackfill sets up storage mock expectations for backfill scenarios.
// blocks.ByHeight returns block fixtures, events.ByBlockID returns a single event.
func (s *testSetup) configureBackfill() {
	s.blocks.
		On("ByHeight", mock.AnythingOfType("uint64")).
		Return(func(height uint64) (*flow.Block, error) {
			block := unittest.BlockFixture(func(b *flow.Block) {
				b.Height = height
				b.Payload.Guarantees = nil
			})
			return block, nil
		})

	s.events.
		On("ByBlockID", mock.AnythingOfType("flow.Identifier")).
		Return([]flow.Event{unittest.EventFixture()}, nil)
}

func (s *testSetup) newExtendedIndexer(
	t *testing.T,
	state protocol.State,
	indexers []extended.Indexer,
	backfillDelay time.Duration,
) *extended.ExtendedIndexer {
	ext, err := extended.NewExtendedIndexer(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		s.db,
		storage.NewTestingLockManager(),
		state,
		s.blocks,
		s.collections,
		s.events,
		s.results,
		indexers,
		flow.Testnet,
		backfillDelay,
	)
	require.NoError(t, err)
	return ext
}

// startComponent starts the ExtendedIndexer component and registers cleanup.
func startComponent(t *testing.T, ext *extended.ExtendedIndexer) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	ext.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, testTimeout, ext)

	t.Cleanup(func() {
		cancel()
		unittest.RequireCloseBefore(t, ext.Done(), testTimeout, "timeout waiting for shutdown")
	})
}

// startComponentWithCallback starts the component with an error callback on the signaler context.
func startComponentWithCallback(
	t *testing.T,
	ext *extended.ExtendedIndexer,
	fn func(error),
) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContextWithCallback(t, ctx, fn)
	ext.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, testTimeout, ext)
	return cancel
}

func provideBlock(t *testing.T, ext *extended.ExtendedIndexer, height uint64) {
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(height))
	require.NoError(t, ext.IndexBlockData(header, nil, nil))
}

// ===== Tests =====

// TestExtendedIndexer_AllLive verifies that when all indexers are caught up to the live height,
// calling IndexBlockData processes the block for all indexers in a single iteration.
func TestExtendedIndexer_AllLive(t *testing.T) {
	setup := newTestSetup(t)
	liveHeight := uint64(11)

	idx1 := newMockIndexer(t, "a", liveHeight, liveHeight)
	idx2 := newMockIndexer(t, "b", liveHeight, liveHeight)

	ext := setup.newExtendedIndexer(t, protocolmock.NewState(t), []extended.Indexer{idx1, idx2}, time.Hour)
	startComponent(t, ext)

	provideBlock(t, ext, liveHeight)

	unittest.RequireCloseBefore(t, idx1.done, testTimeout, "timeout waiting for idx1")
	unittest.RequireCloseBefore(t, idx2.done, testTimeout, "timeout waiting for idx2")

	// Both should have advanced past liveHeight
	assert.Equal(t, liveHeight+1, idx1.nextHeight.Load())
	assert.Equal(t, liveHeight+1, idx2.nextHeight.Load())
}

// TestExtendedIndexer_AllBackfilling verifies that when all indexers are behind, the timer-driven
// loop fetches data from storage and processes each indexer independently until they reach a target.
func TestExtendedIndexer_AllBackfilling(t *testing.T) {
	setup := newTestSetup(t)
	setup.configureBackfill()

	// idx1 starts at 2, idx2 starts at 4 — both backfill from storage
	idx1 := newMockIndexer(t, "a", 2, 6)
	idx2 := newMockIndexer(t, "b", 4, 6)

	ext := setup.newExtendedIndexer(t, newMockState(t), []extended.Indexer{idx1, idx2}, time.Millisecond)
	startComponent(t, ext)

	// No IndexBlockData call — backfill is driven entirely by the timer and storage
	unittest.RequireCloseBefore(t, idx1.done, testTimeout, "timeout waiting for idx1 backfill")
	unittest.RequireCloseBefore(t, idx2.done, testTimeout, "timeout waiting for idx2 backfill")
}

// TestExtendedIndexer_SplitLiveAndBackfill verifies that one indexer processes live data immediately
// while another backfills from storage concurrently in the same iteration loop.
func TestExtendedIndexer_SplitLiveAndBackfill(t *testing.T) {
	setup := newTestSetup(t)
	setup.configureBackfill()
	liveHeight := uint64(8)

	// live indexer already at liveHeight
	liveIdx := newMockIndexer(t, "live", liveHeight, liveHeight)

	// backfill indexer needs to catch up from height 3 to liveHeight
	backfillIdx := newMockIndexer(t, "backfill", 3, liveHeight)

	ext := setup.newExtendedIndexer(t, newMockState(t), []extended.Indexer{liveIdx, backfillIdx}, time.Millisecond)
	startComponent(t, ext)

	provideBlock(t, ext, liveHeight)

	unittest.RequireCloseBefore(t, liveIdx.done, testTimeout, "timeout waiting for live indexer")
	unittest.RequireCloseBefore(t, backfillIdx.done, testTimeout, "timeout waiting for backfill indexer")
}

// TestExtendedIndexer_CatchUpAndContinueLive verifies that an indexer backfills from storage,
// catches up to the live height, and then processes subsequent live blocks via IndexBlockData.
func TestExtendedIndexer_CatchUpAndContinueLive(t *testing.T) {
	setup := newTestSetup(t)
	setup.configureBackfill()
	liveHeight := uint64(5)

	// idx starts at height 2, needs to backfill 2..5, then process live block at 6
	idx := newMockIndexer(t, "a", 2, liveHeight+1)

	ext := setup.newExtendedIndexer(t, newMockState(t), []extended.Indexer{idx}, time.Millisecond)
	startComponent(t, ext)

	// Give backfill some time to make progress from storage
	time.Sleep(50 * time.Millisecond)

	// Provide a live block — the indexer should process it after catching up
	provideBlock(t, ext, liveHeight+1)

	unittest.RequireCloseBefore(t, idx.done, testTimeout, "timeout waiting for catch-up and live processing")
}

// TestExtendedIndexer_UninitializedBeforeLiveData verifies that when the component starts and no
// IndexBlockData has been called yet, the timer-driven loop still processes blocks from storage.
// This covers the case where latestBlockData is nil but storage has data available.
func TestExtendedIndexer_UninitializedBeforeLiveData(t *testing.T) {
	setup := newTestSetup(t)
	setup.configureBackfill()

	// Indexer starts at height 5 — storage has data, but no live block provided
	idx := newMockIndexer(t, "a", 5, 8)

	ext := setup.newExtendedIndexer(t, newMockState(t), []extended.Indexer{idx}, time.Millisecond)
	startComponent(t, ext)

	// No IndexBlockData call. hasBackfillingIndexers returns true when latestBlockData==nil,
	// so the timer keeps firing and blockData fetches from storage.
	unittest.RequireCloseBefore(t, idx.done, testTimeout, "timeout waiting for processing without live data")
}

// TestExtendedIndexer_UninitializedNotFoundThenCatchUp verifies that when the component starts,
// storage initially returns ErrNotFound, and the indexer retries on subsequent timer ticks.
// Once storage has data, the indexer processes it successfully.
func TestExtendedIndexer_UninitializedNotFoundThenCatchUp(t *testing.T) {
	db := newTestDB(t)

	// Storage starts returning ErrNotFound, then switches to returning data after a delay.
	blocks := storagemock.NewBlocks(t)
	collections := storagemock.NewCollections(t)
	events := storagemock.NewEvents(t)

	available := atomic.NewBool(false)
	blocks.
		On("ByHeight", mock.AnythingOfType("uint64")).
		Return(func(height uint64) (*flow.Block, error) {
			if !available.Load() {
				return nil, storage.ErrNotFound
			}
			block := unittest.BlockFixture(func(b *flow.Block) {
				b.Height = height
				b.Payload.Guarantees = nil
			})
			return block, nil
		})

	events.
		On("ByBlockID", mock.AnythingOfType("flow.Identifier")).
		Return([]flow.Event{unittest.EventFixture()}, nil)

	idx := newMockIndexer(t, "a", 5, 5)

	ext, err := extended.NewExtendedIndexer(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		db,
		storage.NewTestingLockManager(),
		newMockState(t),
		blocks, collections, events,
		storagemock.NewLightTransactionResults(t),
		[]extended.Indexer{idx},
		flow.Testnet,
		time.Millisecond,
	)
	require.NoError(t, err)
	startComponent(t, ext)

	// Let a few timer iterations pass with ErrNotFound — indexer should not have been called
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, uint64(5), idx.nextHeight.Load(), "indexer should not have advanced")

	// Make data available — next timer tick should process it
	available.Store(true)

	unittest.RequireCloseBefore(t, idx.done, testTimeout, "timeout waiting for indexer after data became available")
	assert.Equal(t, uint64(6), idx.nextHeight.Load())
}

// ===== Error Handling Tests =====

// TestExtendedIndexer_IndexerError verifies that when a sub-indexer returns an unexpected error,
// it is thrown via the irrecoverable context.
func TestExtendedIndexer_IndexerError(t *testing.T) {
	setup := newTestSetup(t)
	liveHeight := uint64(11)
	indexerErr := errors.New("indexer failed")

	idx := extendedmock.NewIndexer(t)
	idx.On("Name").Return("a")
	idx.On("NextHeight").Return(liveHeight, nil)
	idx.On("IndexBlockData", mock.Anything, mock.Anything, mock.Anything).Return(indexerErr).Once()

	ext := setup.newExtendedIndexer(t, protocolmock.NewState(t), []extended.Indexer{idx}, time.Hour)

	thrown := make(chan error, 1)
	cancel := startComponentWithCallback(t, ext, func(err error) {
		thrown <- err
	})
	defer cancel()

	provideBlock(t, ext, liveHeight)

	select {
	case err := <-thrown:
		assert.ErrorIs(t, err, indexerErr)
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for thrown error")
	}
}

// TestExtendedIndexer_BackfillError verifies that errors during backfill are thrown
// via the irrecoverable context.
func TestExtendedIndexer_BackfillError(t *testing.T) {
	setup := newTestSetup(t)
	setup.configureBackfill()
	backfillErr := errors.New("backfill failed")

	idx := extendedmock.NewIndexer(t)
	idx.On("Name").Return("a")
	idx.On("NextHeight").Return(uint64(3), nil)
	idx.On("IndexBlockData", mock.Anything, mock.Anything, mock.Anything).Return(backfillErr).Once()

	ext := setup.newExtendedIndexer(t, newMockState(t), []extended.Indexer{idx}, time.Millisecond)

	thrown := make(chan error, 1)
	cancel := startComponentWithCallback(t, ext, func(err error) {
		thrown <- err
	})
	defer cancel()

	select {
	case err := <-thrown:
		assert.ErrorIs(t, err, backfillErr)
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for thrown error")
	}
}

// TestExtendedIndexer_AlreadyIndexedSkipped verifies that ErrAlreadyIndexed from a sub-indexer
// is treated as a skip rather than an error.
func TestExtendedIndexer_AlreadyIndexedSkipped(t *testing.T) {
	setup := newTestSetup(t)
	liveHeight := uint64(11)

	idx1 := extendedmock.NewIndexer(t)
	idx2 := extendedmock.NewIndexer(t)
	idx2.On("Name").Return("b")
	idx1.On("NextHeight").Return(liveHeight, nil)
	idx2.On("NextHeight").Return(liveHeight, nil)
	// idx1 returns ErrAlreadyIndexed — should be skipped without error
	idx1.On("IndexBlockData", mock.Anything, mock.Anything, mock.Anything).
		Return(extended.ErrAlreadyIndexed).Once()

	// idx2 succeeds — signals done
	done := make(chan struct{})
	idx2.On("IndexBlockData", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once().Run(func(args mock.Arguments) {
		close(done)
	})

	ext := setup.newExtendedIndexer(t, protocolmock.NewState(t), []extended.Indexer{idx1, idx2}, time.Hour)
	startComponent(t, ext)

	provideBlock(t, ext, liveHeight)

	unittest.RequireCloseBefore(t, done, testTimeout, "timeout waiting for idx2")
}

// TestExtendedIndexer_NonSequentialHeight verifies that IndexBlockData rejects non-sequential heights
// after the first block has been provided.
func TestExtendedIndexer_NonSequentialHeight(t *testing.T) {
	setup := newTestSetup(t)

	idx := newMockIndexer(t, "a", 11, 0)
	ext := setup.newExtendedIndexer(t, protocolmock.NewState(t), []extended.Indexer{idx}, time.Hour)
	startComponent(t, ext)

	// First call succeeds
	provideBlock(t, ext, 11)

	// Non-sequential height is rejected
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(13))
	err := ext.IndexBlockData(header, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("expected height %d, but got %d", 12, 13))
}
