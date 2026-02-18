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
	"github.com/stretchr/testify/suite"
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
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

const testTimeout = 5 * time.Second

type ExtendedIndexerSuite struct {
	suite.Suite

	g *fixtures.GeneratorSuite

	db          storage.DB
	dbDir       string
	headers     *storagemock.Headers
	index       *storagemock.Index
	guarantees  *storagemock.Guarantees
	collections *storagemock.Collections
	events      *storagemock.Events
	results     *storagemock.LightTransactionResults

	ext    *extended.ExtendedIndexer
	cancel context.CancelFunc
}

func TestExtendedIndexer(t *testing.T) {
	suite.Run(t, new(ExtendedIndexerSuite))
}

func (s *ExtendedIndexerSuite) SetupTest() {
	s.g = fixtures.NewGeneratorSuite(fixtures.WithChainID(flow.Testnet))

	pdb, dbDir := unittest.TempPebbleDB(s.T())
	s.db = pebbleimpl.ToDB(pdb)
	s.dbDir = dbDir

	s.headers = storagemock.NewHeaders(s.T())
	s.index = storagemock.NewIndex(s.T())
	s.guarantees = storagemock.NewGuarantees(s.T())
	s.collections = storagemock.NewCollections(s.T())
	s.events = storagemock.NewEvents(s.T())
	s.results = storagemock.NewLightTransactionResults(s.T())
}

func (s *ExtendedIndexerSuite) TearDownTest() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.ext != nil {
		unittest.RequireCloseBefore(s.T(), s.ext.Done(), testTimeout, "timeout waiting for shutdown")
	}

	require.NoError(s.T(), s.db.Close())
	require.NoError(s.T(), os.RemoveAll(s.dbDir))
}

// configureStorage sets up storage mocks to return fixture data for exact IDs.
// Each mock uses exact argument matchers rather than mock.Anything.
// Expectations use Maybe() because timing-dependent tests may not call all of them.
// Correctness is verified via assertBackfilledBlockData/assertLiveBlockData assertions.
func (s *ExtendedIndexerSuite) configureStorage(blocks map[uint64]*blockFixtures) {
	for height, block := range blocks {
		blockID := block.Header.ID()

		s.headers.On("BlockIDByHeight", height).Return(blockID, nil).Maybe()
		s.headers.On("ByBlockID", blockID).Return(block.Header, nil).Maybe()
		s.index.On("ByBlockID", blockID).Return(block.Index, nil).Maybe()
		s.events.On("ByBlockID", blockID).Return(block.Events, nil).Maybe()

		for _, guarantee := range block.Guarantees {
			s.guarantees.On("ByID", guarantee.ID()).Return(guarantee, nil).Maybe()
		}

		for _, collection := range block.Collections {
			s.collections.On("ByID", collection.ID()).Return(collection, nil).Maybe()
		}
	}

	// Catch-all for heights not in the fixture map (e.g., when indexers overshoot the target).
	s.headers.On("BlockIDByHeight", mock.AnythingOfType("uint64")).Maybe().Return(flow.ZeroID, storage.ErrNotFound)
}

// newExtendedIndexer creates a new extended indexer with the given state, indexers, and backfill delay.
func (s *ExtendedIndexerSuite) newExtendedIndexer(
	state protocol.State,
	indexers []extended.Indexer,
	backfillDelay time.Duration,
) {
	ext, err := extended.NewExtendedIndexer(
		unittest.Logger(),
		metrics.NewNoopCollector(),
		s.db,
		storage.NewTestingLockManager(),
		state,
		s.headers,
		s.index,
		s.guarantees,
		s.collections,
		s.events,
		s.results,
		indexers,
		flow.Testnet,
		backfillDelay,
	)
	require.NoError(s.T(), err)
	s.ext = ext
}

// startComponent starts the extended indexer with an irrecoverable signaler context that requires no
// errors are thrown
func (s *ExtendedIndexerSuite) startComponent() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	signalerCtx := irrecoverable.NewMockSignalerContext(s.T(), ctx)
	s.ext.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(s.T(), testTimeout, s.ext)
}

// startComponentWithCallback starts the extended indexer with an irrecoverable signaler context that
// calls the provided callback when an error is thrown.
func (s *ExtendedIndexerSuite) startComponentWithCallback(fn func(error)) {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	signalerCtx := irrecoverable.NewMockSignalerContextWithCallback(s.T(), ctx, fn)
	s.ext.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(s.T(), testTimeout, s.ext)
}

// provideBlock provides a block to the extended indexer.
// This is used when testing backfill mode and the actual block data is not read
func (s *ExtendedIndexerSuite) provideBlock(height uint64) {
	header := s.g.Headers().Fixture(fixtures.Header.WithHeight(height))
	require.NoError(s.T(), s.ext.IndexBlockData(header, nil, nil))
}

// provideLiveBlock provides a live block to the extended indexer with complete data.
func (s *ExtendedIndexerSuite) provideLiveBlock(block *blockFixtures) {
	require.NoError(s.T(), s.ext.IndexBlockData(block.Header, block.allTransactions(), block.Events))
}

// ===== Assertions =====

// assertBackfilledBlockData verifies that the received BlockData for a backfilled block contains
// the expected user transactions from the guarantee->collection->transaction pipeline, followed by
// system transactions appended by the system collection builder, and all events grouped by tx index.
func (s *ExtendedIndexerSuite) assertBackfilledBlockData(m *mockIndexer, fixture *blockFixtures) {
	s.T().Helper()
	data := m.blockDataForHeight(fixture.Header.Height)
	require.NotNil(s.T(), data, "no block data received for height %d", fixture.Header.Height)

	assert.Equal(s.T(), fixture.Header, data.Header)

	// Backfilled data should contain user transactions + system transactions in exact order.
	expectedTxs := fixture.allTransactions()
	require.Len(s.T(), data.Transactions, len(expectedTxs),
		"transaction count mismatch at height %d: expected %d user + %d system = %d total, got %d",
		fixture.Header.Height,
		len(fixture.userTransactions()), len(fixture.SystemCollection.Transactions), len(expectedTxs),
		len(data.Transactions))

	for i, expectedTx := range expectedTxs {
		assert.Equal(s.T(), expectedTx.ID(), data.Transactions[i].ID(), "transaction %d mismatch at height %d", i, fixture.Header.Height)
	}

	s.assertEventGroups(data.Events, fixture.Events)
}

// assertLiveBlockData verifies that the received BlockData for a live block matches
// the expected fixture data: user transactions followed by system transactions, and all
// events grouped by tx index.
func (s *ExtendedIndexerSuite) assertLiveBlockData(m *mockIndexer, fixture *blockFixtures) {
	s.T().Helper()
	data := m.blockDataForHeight(fixture.Header.Height)
	require.NotNil(s.T(), data, "no block data received for height %d", fixture.Header.Height)

	assert.Equal(s.T(), fixture.Header, data.Header)

	// Live data contains user transactions followed by system transactions.
	expectedTxs := fixture.allTransactions()

	require.Len(s.T(), data.Transactions, len(expectedTxs), "transaction count mismatch at height %d", fixture.Header.Height)
	for i, expectedTx := range expectedTxs {
		assert.Equal(s.T(), expectedTx.ID(), data.Transactions[i].ID(), "transaction %d mismatch at height %d", i, fixture.Header.Height)
	}

	s.assertEventGroups(data.Events, fixture.Events)
}

// assertEventGroups verifies that the actual event map matches the expected events when grouped
// by transaction index. Each group is compared element-by-element to ensure correct ordering.
func (s *ExtendedIndexerSuite) assertEventGroups(actual map[uint32][]flow.Event, allEvents []flow.Event) {
	s.T().Helper()

	// Build expected groups independently from the production groupEventsByTxIndex.
	expected := make(map[uint32][]flow.Event)
	for _, event := range allEvents {
		expected[event.TransactionIndex] = append(expected[event.TransactionIndex], event)
	}

	require.Equal(s.T(), len(expected), len(actual), "event group count mismatch")
	for txIndex, expectedGroup := range expected {
		actualGroup, ok := actual[txIndex]
		require.True(s.T(), ok, "missing event group for tx index %d", txIndex)
		require.Len(s.T(), actualGroup, len(expectedGroup), "event count mismatch for tx index %d", txIndex)

		// must be in the same order
		assert.Equal(s.T(), expectedGroup, actualGroup, "event mismatch at tx index %d", txIndex)
	}
}

// ===== Tests =====

// TestAllLive verifies that when all indexers are caught up to the live height,
// calling IndexBlockData processes the block for all indexers in a single iteration.
func (s *ExtendedIndexerSuite) TestAllLive() {
	liveHeight := uint64(11)

	block := generateBlockFixtures(s.T(), s.g, liveHeight)

	idx1 := newMockIndexer(s.T(), "a", liveHeight, liveHeight)
	idx2 := newMockIndexer(s.T(), "b", liveHeight, liveHeight)

	s.newExtendedIndexer(protocolmock.NewState(s.T()), []extended.Indexer{idx1, idx2}, time.Hour)
	s.startComponent()

	s.provideLiveBlock(block)

	unittest.RequireCloseBefore(s.T(), idx1.done, testTimeout, "timeout waiting for idx1")
	unittest.RequireCloseBefore(s.T(), idx2.done, testTimeout, "timeout waiting for idx2")

	s.assertLiveBlockData(idx1, block)
	s.assertLiveBlockData(idx2, block)
}

// TestAllBackfilling verifies that when all indexers are behind, the timer-driven
// loop fetches data from storage and processes each indexer independently until they reach a target.
func (s *ExtendedIndexerSuite) TestAllBackfilling() {
	blocks := make(map[uint64]*blockFixtures)
	for h := uint64(2); h <= 6; h++ {
		blocks[h] = generateBlockFixtures(s.T(), s.g, h)
	}
	s.configureStorage(blocks)

	// idx1 starts at 2, idx2 starts at 4 -- both backfill from storage.
	idx1 := newMockIndexer(s.T(), "a", 2, 6)
	idx2 := newMockIndexer(s.T(), "b", 4, 6)

	s.newExtendedIndexer(newMockState(s.T()), []extended.Indexer{idx1, idx2}, time.Millisecond)
	s.startComponent()

	// No IndexBlockData call -- backfill is driven entirely by the timer and storage.
	unittest.RequireCloseBefore(s.T(), idx1.done, testTimeout, "timeout waiting for idx1 backfill")
	unittest.RequireCloseBefore(s.T(), idx2.done, testTimeout, "timeout waiting for idx2 backfill")

	// Verify backfilled data exercises the guarantee->collection->transaction pipeline.
	for h := uint64(2); h <= 6; h++ {
		s.assertBackfilledBlockData(idx1, blocks[h])
	}
	for h := uint64(4); h <= 6; h++ {
		s.assertBackfilledBlockData(idx2, blocks[h])
	}
}

// TestSplitLiveAndBackfill verifies that one indexer processes live data immediately
// while another backfills from storage concurrently in the same iteration loop.
func (s *ExtendedIndexerSuite) TestSplitLiveAndBackfill() {
	liveHeight := uint64(8)

	// Generate storage fixtures for all heights that may be queried during backfill.
	// The live indexer at liveHeight may also attempt a storage lookup before the live block arrives.
	blocks := make(map[uint64]*blockFixtures)
	for h := uint64(3); h <= liveHeight; h++ {
		blocks[h] = generateBlockFixtures(s.T(), s.g, h)
	}
	s.configureStorage(blocks)

	liveIdx := newMockIndexer(s.T(), "live", liveHeight, liveHeight)
	backfillIdx := newMockIndexer(s.T(), "backfill", 3, liveHeight)

	s.newExtendedIndexer(newMockState(s.T()), []extended.Indexer{liveIdx, backfillIdx}, time.Millisecond)
	s.startComponent()

	// Use provideLiveBlock so both the live and backfill paths use the same block header and data.
	// This allows asserting height liveHeight for all indexers regardless of which path was used.
	s.provideLiveBlock(blocks[liveHeight])

	unittest.RequireCloseBefore(s.T(), liveIdx.done, testTimeout, "timeout waiting for live indexer")
	unittest.RequireCloseBefore(s.T(), backfillIdx.done, testTimeout, "timeout waiting for backfill indexer")

	// Verify the live indexer received the correct data at liveHeight.
	s.assertLiveBlockData(liveIdx, blocks[liveHeight])

	// Verify backfilled data for all heights, including liveHeight.
	// backfillIdx may process liveHeight via either the live or storage path, but both
	// produce the same data since provideLiveBlock provides allTransactions().
	for h := uint64(3); h <= liveHeight; h++ {
		s.assertBackfilledBlockData(backfillIdx, blocks[h])
	}
}

// TestCatchUpAndContinueLive verifies that an indexer backfills from storage,
// catches up to the live height, and then processes subsequent live blocks via IndexBlockData.
func (s *ExtendedIndexerSuite) TestCatchUpAndContinueLive() {
	liveHeight := uint64(5)

	// Generate storage fixtures for heights 2..liveHeight+1 so the backfill can access all needed data.
	blocks := make(map[uint64]*blockFixtures)
	for h := uint64(2); h <= liveHeight+1; h++ {
		blocks[h] = generateBlockFixtures(s.T(), s.g, h)
	}
	s.configureStorage(blocks)

	// idx starts at height 2, needs to backfill 2..5, then process live block at 6.
	idx := newMockIndexer(s.T(), "a", 2, liveHeight+1)

	s.newExtendedIndexer(newMockState(s.T()), []extended.Indexer{idx}, time.Millisecond)
	s.startComponent()

	// Wait until the indexer has processed at least one block from storage before providing
	// the live block, ensuring the "catch up" path is exercised.
	require.Eventually(s.T(), func() bool {
		return idx.nextHeight.Load() > 2
	}, testTimeout, time.Millisecond, "indexer should have made backfill progress from storage")

	// Provide a live block -- the indexer should process it after catching up.
	s.provideBlock(liveHeight + 1)

	unittest.RequireCloseBefore(s.T(), idx.done, testTimeout, "timeout waiting for catch-up and live processing")

	// Verify backfilled data for heights that came from storage.
	for h := uint64(2); h <= liveHeight; h++ {
		s.assertBackfilledBlockData(idx, blocks[h])
	}
}

// TestUninitializedBeforeLiveData verifies that when the component starts and no
// IndexBlockData has been called yet, the timer-driven loop still processes blocks from storage.
func (s *ExtendedIndexerSuite) TestUninitializedBeforeLiveData() {
	blocks := make(map[uint64]*blockFixtures)
	for h := uint64(5); h <= 8; h++ {
		blocks[h] = generateBlockFixtures(s.T(), s.g, h)
	}
	s.configureStorage(blocks)

	// Indexer starts at height 5 -- storage has data, but no live block provided.
	idx := newMockIndexer(s.T(), "a", 5, 8)

	s.newExtendedIndexer(newMockState(s.T()), []extended.Indexer{idx}, time.Millisecond)
	s.startComponent()

	// No IndexBlockData call. hasBackfillingIndexers returns true when latestBlockData==nil,
	// so the timer keeps firing and blockData fetches from storage.
	unittest.RequireCloseBefore(s.T(), idx.done, testTimeout, "timeout waiting for processing without live data")

	for h := uint64(5); h <= 8; h++ {
		s.assertBackfilledBlockData(idx, blocks[h])
	}
}

// TestUninitializedNotFoundThenCatchUp verifies that when the component starts,
// storage initially returns ErrNotFound, and the indexer retries on subsequent timer ticks.
// Once storage has data, the indexer processes it successfully.
func (s *ExtendedIndexerSuite) TestUninitializedNotFoundThenCatchUp() {
	block := generateBlockFixtures(s.T(), s.g, 5)
	blockID := block.Header.ID()

	// BlockIDByHeight returns ErrNotFound until the available flag is set.
	available := atomic.NewBool(false)
	s.headers.
		On("BlockIDByHeight", uint64(5)).
		Return(func(uint64) (flow.Identifier, error) {
			if !available.Load() {
				return flow.ZeroID, storage.ErrNotFound
			}
			return blockID, nil
		})

	// Remaining storage mocks use exact fixture data (only called after available=true).
	s.headers.On("ByBlockID", blockID).Return(block.Header, nil)
	s.index.On("ByBlockID", blockID).Return(block.Index, nil)
	s.events.On("ByBlockID", blockID).Return(block.Events, nil)
	for _, guarantee := range block.Guarantees {
		s.guarantees.On("ByID", guarantee.ID()).Return(guarantee, nil)
	}
	for _, collection := range block.Collections {
		s.collections.On("ByID", collection.ID()).Return(collection, nil)
	}

	idx := newMockIndexer(s.T(), "a", 5, 5)

	s.newExtendedIndexer(newMockState(s.T()), []extended.Indexer{idx}, time.Millisecond)
	s.startComponent()

	// Let a few timer iterations pass with ErrNotFound -- indexer should not have been called.
	require.Never(s.T(), func() bool {
		return idx.nextHeight.Load() > 2
	}, 50*time.Millisecond, time.Millisecond, "indexer should not have advanced")

	// Make data available -- next timer tick should process it.
	available.Store(true)

	unittest.RequireCloseBefore(s.T(), idx.done, testTimeout, "timeout waiting for indexer after data became available")
	assert.Equal(s.T(), uint64(6), idx.nextHeight.Load())

	s.assertBackfilledBlockData(idx, block)
}

// ===== Error Handling Tests =====

// TestIndexerError verifies that when a sub-indexer returns an unexpected error,
// it is thrown via the irrecoverable context.
func (s *ExtendedIndexerSuite) TestIndexerError() {
	liveHeight := uint64(11)
	indexerErr := errors.New("indexer failed")

	idx := extendedmock.NewIndexer(s.T())
	idx.On("Name").Return("a")
	idx.On("NextHeight").Return(liveHeight, nil)
	idx.On("IndexBlockData", mock.Anything, mock.Anything, mock.Anything).Return(indexerErr).Once()

	s.newExtendedIndexer(protocolmock.NewState(s.T()), []extended.Indexer{idx}, time.Hour)

	thrown := make(chan error, 1)
	s.startComponentWithCallback(func(err error) {
		thrown <- err
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
	index       *storagemock.Index
	headers     *storagemock.Headers
	guarantees  *storagemock.Guarantees
	collections *storagemock.Collections
	events      *storagemock.Events
	results     *storagemock.LightTransactionResults
}

func newTestSetup(t *testing.T) *testSetup {
	return &testSetup{
		db:          newTestDB(t),
		index:       storagemock.NewIndex(t),
		headers:     storagemock.NewHeaders(t),
		guarantees:  storagemock.NewGuarantees(t),
		collections: storagemock.NewCollections(t),
		events:      storagemock.NewEvents(t),
		results:     storagemock.NewLightTransactionResults(t),
	}
}

// configureBackfill sets up storage mock expectations for backfill scenarios.
// headers.BlockIDByHeight returns a deterministic block ID, headers.ByBlockID returns the
// corresponding header, index.ByBlockID returns an empty block index, and events.ByBlockID
// returns a single event.
func (s *testSetup) configureBackfill() {
	var headersByID sync.Map

	s.headers.
		On("BlockIDByHeight", mock.AnythingOfType("uint64")).
		Return(func(height uint64) (flow.Identifier, error) {
			header := unittest.BlockHeaderFixture()
			header.Height = height
			headersByID.Store(header.ID(), header)
			return header.ID(), nil
		})

	s.headers.
		On("ByBlockID", mock.AnythingOfType("flow.Identifier")).
		Return(func(blockID flow.Identifier) (*flow.Header, error) {
			val, ok := headersByID.Load(blockID)
			if !ok {
				return nil, storage.ErrNotFound
			}
			return val.(*flow.Header), nil
		})

	s.index.
		On("ByBlockID", mock.AnythingOfType("flow.Identifier")).
		Return(&flow.Index{}, nil)

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
		s.index,
		s.headers,
		s.guarantees,
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
	headers := storagemock.NewHeaders(t)
	index := storagemock.NewIndex(t)
	events := storagemock.NewEvents(t)

	var headersByID sync.Map

	available := atomic.NewBool(false)
	headers.
		On("BlockIDByHeight", mock.AnythingOfType("uint64")).
		Return(func(height uint64) (flow.Identifier, error) {
			if !available.Load() {
				return flow.ZeroID, storage.ErrNotFound
			}
			header := unittest.BlockHeaderFixture()
			header.Height = height
			headersByID.Store(header.ID(), header)
			return header.ID(), nil
		})

	headers.
		On("ByBlockID", mock.AnythingOfType("flow.Identifier")).
		Return(func(blockID flow.Identifier) (*flow.Header, error) {
			val, ok := headersByID.Load(blockID)
			if !ok {
				return nil, storage.ErrNotFound
			}
			return val.(*flow.Header), nil
		})

	index.
		On("ByBlockID", mock.AnythingOfType("flow.Identifier")).
		Return(&flow.Index{}, nil)

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
		index,
		headers,
		storagemock.NewGuarantees(t),
		storagemock.NewCollections(t),
		events,
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
	assert.Contains(t, err.Error(), fmt.Sprintf("indexing block skipped: expected height %d, got %d", 12, 13))
}
