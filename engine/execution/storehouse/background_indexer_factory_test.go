package storehouse_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	"github.com/cockroachdb/pebble/v2"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/state/protocol/events"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	storagepebble "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
)

// TestLoadRegisterStore_Disabled tests that LoadRegisterStore returns nil when enableStorehouse is false
func TestLoadRegisterStore_Disabled(t *testing.T) {
	t.Parallel()

	log := unittest.Logger()
	state := protocolmock.NewState(t)
	headers := storagemock.NewHeaders(t)
	protocolEvents := events.NewDistributor()
	lastFinalizedHeight := uint64(100)
	collector := &metrics.NoopCollector{}
	enableStorehouse := false
	registerDir := t.TempDir()
	triedir := t.TempDir()
	importCheckpointWorkerCount := 1
	var importFunc storehouse.ImportRegistersFromCheckpoint = nil

	registerStore, closer, err := storehouse.LoadRegisterStore(
		log,
		state,
		headers,
		protocolEvents,
		lastFinalizedHeight,
		collector,
		enableStorehouse,
		registerDir,
		triedir,
		importCheckpointWorkerCount,
		importFunc,
	)

	require.NoError(t, err)
	require.Nil(t, registerStore)
	require.Nil(t, closer)
}

// TestLoadBackgroundIndexerEngine_StorehouseEnabled tests that LoadBackgroundIndexerEngine returns nil when enableStorehouse is true
func TestLoadBackgroundIndexerEngine_StorehouseEnabled(t *testing.T) {
	t.Parallel()

	log := unittest.Logger()
	enableStorehouse := true
	enableBackgroundStorehouseIndexing := true
	state := protocolmock.NewState(t)
	headers := storagemock.NewHeaders(t)
	protocolEvents := events.NewDistributor()
	lastFinalizedHeight := uint64(100)
	collector := &metrics.NoopCollector{}
	registerDir := t.TempDir()
	triedir := t.TempDir()
	importCheckpointWorkerCount := 1
	var importFunc storehouse.ImportRegistersFromCheckpoint = nil
	var executionDataStore execution_data.ExecutionDataGetter = nil
	resultsReader := storagemock.NewExecutionResults(t)
	blockExecutedNotifier := &mockBlockExecutedNotifier{}
	followerDistributor := pubsub.NewFollowerDistributor()
	engine, created, err := storehouse.LoadBackgroundIndexerEngine(
		log,
		enableStorehouse,
		enableBackgroundStorehouseIndexing,
		state,
		headers,
		protocolEvents,
		lastFinalizedHeight,
		collector,
		registerDir,
		triedir,
		importCheckpointWorkerCount,
		importFunc,
		executionDataStore,
		resultsReader,
		blockExecutedNotifier,
		followerDistributor,
	)

	require.NoError(t, err)
	require.Nil(t, engine)
	require.False(t, created)
}

// TestLoadBackgroundIndexerEngine_BackgroundIndexingDisabled tests that LoadBackgroundIndexerEngine returns nil when enableBackgroundStorehouseIndexing is false
func TestLoadBackgroundIndexerEngine_BackgroundIndexingDisabled(t *testing.T) {
	t.Parallel()

	log := unittest.Logger()
	enableStorehouse := false
	enableBackgroundStorehouseIndexing := false
	state := protocolmock.NewState(t)
	headers := storagemock.NewHeaders(t)
	protocolEvents := events.NewDistributor()
	lastFinalizedHeight := uint64(100)
	collector := &metrics.NoopCollector{}
	registerDir := t.TempDir()
	triedir := t.TempDir()
	importCheckpointWorkerCount := 1
	var importFunc storehouse.ImportRegistersFromCheckpoint = nil
	var executionDataStore execution_data.ExecutionDataGetter = nil
	resultsReader := storagemock.NewExecutionResults(t)
	blockExecutedNotifier := &mockBlockExecutedNotifier{}
	followerDistributor := pubsub.NewFollowerDistributor()
	engine, created, err := storehouse.LoadBackgroundIndexerEngine(
		log,
		enableStorehouse,
		enableBackgroundStorehouseIndexing,
		state,
		headers,
		protocolEvents,
		lastFinalizedHeight,
		collector,
		registerDir,
		triedir,
		importCheckpointWorkerCount,
		importFunc,
		executionDataStore,
		resultsReader,
		blockExecutedNotifier,
		followerDistributor,
	)

	require.NoError(t, err)
	require.Nil(t, engine)
	require.False(t, created)
}

// TestLoadBackgroundIndexerEngine_HappyCase tests the happy case where the background indexer engine
// bootstraps by importing the checkpoint and then stops, notifying metrics about the last saved height
func TestLoadBackgroundIndexerEngine_HappyCase(t *testing.T) {
	t.Parallel()

	const (
		startHeight        = 100
		registerStoreStart = 100
	)

	log := unittest.Logger()
	enableStorehouse := false
	enableBackgroundStorehouseIndexing := true

	// Set up protocol state with finalized blocks
	state := protocolmock.NewState(t)
	finalSnapshot := protocolmock.NewSnapshot(t)
	params := protocolmock.NewParams(t)
	sealedRoot := unittest.BlockHeaderFixture()
	sealedRoot.Height = startHeight
	seal := unittest.Seal.Fixture()
	seal.BlockID = sealedRoot.ID()
	params.On("SealedRoot").Return(sealedRoot).Maybe()
	params.On("Seal").Return(seal).Maybe()
	state.On("Final").Return(finalSnapshot).Maybe()
	state.On("Params").Return(params).Maybe()

	// Create headers storage
	headers := storagemock.NewHeaders(t)
	// Mock BlockIDByHeight for the finalized reader initialization
	// The finalized reader calls this during initialization to check the last finalized height
	parentBlockID := sealedRoot.ID()
	headers.On("BlockIDByHeight", uint64(startHeight)).Return(parentBlockID, nil).Maybe()

	// Mock Head() for the indexer - it may be called when trying to index additional blocks
	// We'll stop the engine before it actually indexes, but the call might happen
	initialFinalHeader := unittest.BlockHeaderFixture()
	initialFinalHeader.Height = startHeight
	finalSnapshot.On("Head").Return(initialFinalHeader, nil).Maybe()

	protocolEvents := events.NewDistributor()
	lastFinalizedHeight := uint64(startHeight)

	// Use a mock metrics collector to detect when the checkpoint height is reported
	mockMetrics := modulemock.NewExecutionMetrics(t)
	checkpointHeight := uint64(registerStoreStart)
	heightReached := make(chan uint64, 1)

	// Set up expectation to signal when checkpoint height is reported
	// This will be called during bootstrapping when the checkpoint is imported
	mockMetrics.On("ExecutionLastFinalizedExecutedBlockHeight", mock.MatchedBy(func(height uint64) bool {
		log.Info().Msgf("Metrics reported finalized and executed height: %d", height)
		if height == checkpointHeight {
			select {
			case heightReached <- height:
			default:
			}
			return true
		}
		return true
	})).Return().Maybe()

	collector := mockMetrics
	registerDir := t.TempDir()
	triedir := t.TempDir()
	importCheckpointWorkerCount := 1
	// Bootstrap the register database before creating the engine
	// This ensures the database is ready when the engine tries to use it
	bootstrappedDB := storagepebble.NewBootstrappedRegistersWithPathForTest(t, registerDir, startHeight, startHeight)
	require.NoError(t, bootstrappedDB.Close())

	// Provide a no-op import function since database is already bootstrapped
	importFunc := storehouse.ImportRegistersFromCheckpoint(func(logger zerolog.Logger, checkpointFile string, checkpointHeight uint64, checkpointRootHash ledger.RootHash, pdb *pebble.DB, workerCount int) error {
		// Database is already bootstrapped, so this is a no-op
		return nil
	})

	// Set up execution data store
	bs := blobs.NewBlobstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	executionDataStore := execution_data.NewExecutionDataStore(bs, execution_data.DefaultSerializer)

	// Set up execution results reader
	resultsReader := storagemock.NewExecutionResults(t)

	// Set up block executed notifier and follower distributor (required by the engine)
	// These are needed even though we stop after bootstrapping
	blockExecutedNotifier := &mockBlockExecutedNotifier{}
	followerDistributor := pubsub.NewFollowerDistributor()

	// Create background indexer engine
	engine, created, err := storehouse.LoadBackgroundIndexerEngine(
		log,
		enableStorehouse,
		enableBackgroundStorehouseIndexing,
		state,
		headers,
		protocolEvents,
		lastFinalizedHeight,
		collector,
		registerDir,
		triedir,
		importCheckpointWorkerCount,
		importFunc,
		executionDataStore,
		resultsReader,
		blockExecutedNotifier,
		followerDistributor,
	)

	require.NoError(t, err)
	require.NotNil(t, engine)
	require.True(t, created)

	// Start the engine
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
	defer cancel()

	engine.Start(ctx)
	unittest.RequireReturnsBefore(t, func() {
		<-engine.Ready()
	}, 5*time.Second, "engine should be ready")

	// Wait for bootstrapping to complete and metrics to be notified
	// The bootstrapper imports the checkpoint and notifies metrics about the last saved height
	// We use the metric notification as the signal to stop everything and assert
	unittest.RequireReturnsBefore(t, func() {
		// Wait for metrics to be notified about the last saved height from checkpoint import
		// The checkpoint import happens during bootstrapping and notifies metrics
		select {
		case reachedHeight := <-heightReached:
			// The checkpoint was imported at registerStoreStart, so metrics should report that height
			require.Equal(t, uint64(registerStoreStart), reachedHeight, "expected checkpoint height %d to be reported", registerStoreStart)
			// Stop the engine immediately after metrics notification (before it tries to index additional blocks)
			cancel()
		case <-time.After(30 * time.Second):
			t.Fatal("timeout waiting for bootstrapping to complete and metrics to be notified")
		}
	}, 35*time.Second, "bootstrapping should complete and notify metrics")

	// Wait for the engine to fully shut down (including closing the database)
	unittest.RequireReturnsBefore(t, func() {
		<-engine.Done()
	}, 5*time.Second, "engine should stop")

	// Wait a bit for the database to be fully closed
	time.Sleep(100 * time.Millisecond)

	// Verify register store has data from checkpoint
	// After the engine stops, we can load the register store and check its latest height
	// The register store should have the checkpoint height from bootstrapping
	registerStore, closer, err := storehouse.LoadRegisterStore(
		log,
		state,
		headers,
		protocolEvents,
		registerStoreStart,
		collector,
		true, // enableStorehouse
		registerDir,
		triedir,
		importCheckpointWorkerCount,
		importFunc,
	)
	require.NoError(t, err)
	require.NotNil(t, registerStore)
	defer func() {
		if closer != nil {
			require.NoError(t, closer.Close())
		}
	}()

	// The register store should have the checkpoint height from bootstrapping
	// Since we only bootstrap (import checkpoint) and don't index additional blocks,
	// the latest height should be the checkpoint height
	latestHeight := registerStore.LastFinalizedAndExecutedHeight()
	require.Equal(t, uint64(registerStoreStart), latestHeight,
		"register store should have checkpoint height %d, but got %d",
		registerStoreStart, latestHeight)
}

// mockBlockExecutedNotifier is a simple mock implementation of BlockExecutedNotifier
type mockBlockExecutedNotifier struct {
	mu        sync.Mutex
	consumers []func()
}

func (m *mockBlockExecutedNotifier) AddConsumer(callback func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consumers = append(m.consumers, callback)
}
