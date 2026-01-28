package storehouse_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	"github.com/cockroachdb/pebble/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/ledger"
	ledgerconvert "github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	ledgercomplete "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
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
)

// TestLoadBackgroundIndexerEngine_StorehouseEnabled tests that LoadBackgroundIndexerEngine returns nil when enableStorehouse is true
func TestLoadBackgroundIndexerEngine_StorehouseEnabled(t *testing.T) {
	t.Parallel()

	log := unittest.Logger()
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
	// Set up execution data store (required when indexing is enabled)
	bs := blobs.NewBlobstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	executionDataStore := execution_data.NewExecutionDataStore(bs, execution_data.DefaultSerializer)
	resultsReader := storagemock.NewExecutionResults(t)
	blockExecutedNotifier := ingestion.NewBlockExecutedNotifier()
	followerDistributor := pubsub.NewFollowerDistributor()
	heightsPerSecond := uint64(10)
	engine, created, err := storehouse.LoadBackgroundIndexerEngine(
		log,
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
		heightsPerSecond,
	)

	require.NoError(t, err)
	require.NotNil(t, engine)
	require.True(t, created)
}

// TestLoadBackgroundIndexerEngine_BackgroundIndexingDisabled tests that LoadBackgroundIndexerEngine returns nil when enableBackgroundStorehouseIndexing is false
func TestLoadBackgroundIndexerEngine_BackgroundIndexingDisabled(t *testing.T) {
	t.Parallel()

	log := unittest.Logger()
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
	blockExecutedNotifier := ingestion.NewBlockExecutedNotifier()
	followerDistributor := pubsub.NewFollowerDistributor()
	heightsPerSecond := uint64(10)
	engine, created, err := storehouse.LoadBackgroundIndexerEngine(
		log,
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
		heightsPerSecond,
	)

	require.NoError(t, err)
	require.Nil(t, engine)
	require.False(t, created)
}

// TestLoadBackgroundIndexerEngine_Bootstrap tests the happy case where the background indexer engine
// bootstraps by importing the checkpoint and then stops, notifying metrics about the last saved height
func TestLoadBackgroundIndexerEngine_Bootstrap(t *testing.T) {
	t.Parallel()

	const (
		startHeight        = 100
		registerStoreStart = 100
	)

	log := unittest.Logger()
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
	blockExecutedNotifier := ingestion.NewBlockExecutedNotifier()
	followerDistributor := pubsub.NewFollowerDistributor()
	heightsPerSecond := uint64(10)

	// Create background indexer engine
	engine, created, err := storehouse.LoadBackgroundIndexerEngine(
		log,
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
		heightsPerSecond,
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

// TestLoadBackgroundIndexerEngine_Indexing tests that the background indexer engine
// indexes additional finalized and executed blocks after bootstrapping
func TestLoadBackgroundIndexerEngine_Indexing(t *testing.T) {
	t.Parallel()

	const (
		startHeight        = 100
		registerStoreStart = 100
		numBlocks          = 5
	)

	log := unittest.Logger()
	enableBackgroundStorehouseIndexing := true

	// Set up protocol state
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

	// Create headers storage and blocks
	headers := storagemock.NewHeaders(t)
	parentBlockID := sealedRoot.ID()
	headers.On("BlockIDByHeight", uint64(startHeight)).Return(parentBlockID, nil)

	blocks := make([]*flow.Block, numBlocks)
	parentHeader := sealedRoot
	for i := 0; i < numBlocks; i++ {
		height := startHeight + uint64(i) + 1
		block := unittest.BlockWithParentFixture(parentHeader)
		blocks[i] = block
		headers.On("ByHeight", height).Return(block.ToHeader(), nil)
		// Mock BlockIDByHeight for all heights - the finalized reader needs this
		headers.On("BlockIDByHeight", height).Return(block.ID(), nil)
		parentHeader = block.ToHeader()
	}

	// Initially set finalized snapshot to return startHeight (no blocks finalized yet)
	// This prevents the initial notification from trying to process blocks
	initialFinalHeader := unittest.BlockHeaderFixture()
	initialFinalHeader.Height = startHeight
	finalSnapshot.On("Head").Return(initialFinalHeader, nil)

	protocolEvents := events.NewDistributor()
	lastFinalizedHeight := uint64(startHeight)

	// Use a mock metrics collector to detect when the target height is reached
	mockMetrics := modulemock.NewExecutionMetrics(t)
	targetHeight := uint64(registerStoreStart + numBlocks)
	heightReached := make(chan uint64, 1)

	// Set up expectation to signal when target height is reached
	// Also log all metric calls to help debug
	mockMetrics.On("ExecutionLastFinalizedExecutedBlockHeight", mock.AnythingOfType("uint64")).Run(func(args mock.Arguments) {
		height := args.Get(0).(uint64)
		log.Info().Msgf("Metrics reported finalized and executed height: %d (target: %d)", height, targetHeight)
		if height >= targetHeight {
			select {
			case heightReached <- height:
			default:
			}
		}
	}).Return()

	collector := mockMetrics
	registerDir := t.TempDir()
	triedir := t.TempDir()
	importCheckpointWorkerCount := 1

	// Bootstrap the register database
	bootstrappedDB := storagepebble.NewBootstrappedRegistersWithPathForTest(t, registerDir, startHeight, startHeight)
	require.NoError(t, bootstrappedDB.Close())

	// Provide a no-op import function since database is already bootstrapped
	importFunc := storehouse.ImportRegistersFromCheckpoint(func(logger zerolog.Logger, checkpointFile string, checkpointHeight uint64, checkpointRootHash ledger.RootHash, pdb *pebble.DB, workerCount int) error {
		return nil
	})

	// Set up execution data store
	bs := blobs.NewBlobstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	executionDataStore := execution_data.NewExecutionDataStore(bs, execution_data.DefaultSerializer)

	// Set up execution results reader
	resultsReader := storagemock.NewExecutionResults(t)

	// Create execution data and results for all blocks
	for i := 0; i < numBlocks; i++ {
		block := blocks[i]

		// Create valid register entries and convert to trie update
		registerEntries := flow.RegisterEntries{
			{
				Key: flow.RegisterID{
					Owner: "owner",
					Key:   fmt.Sprintf("key%d", i),
				},
				Value: []byte(fmt.Sprintf("value%d", i)),
			},
		}

		// Convert register entries to ledger keys and values
		keys := make([]ledger.Key, 0, len(registerEntries))
		values := make([]ledger.Value, 0, len(registerEntries))
		for _, entry := range registerEntries {
			key := ledgerconvert.RegisterIDToLedgerKey(entry.Key)
			keys = append(keys, key)
			values = append(values, entry.Value)
		}

		// Create trie update from keys and values
		update, err := ledger.NewUpdate(ledger.DummyState, keys, values)
		require.NoError(t, err)
		trieUpdate, err := pathfinder.UpdateToTrieUpdate(update, ledgercomplete.DefaultPathFinderVersion)
		require.NoError(t, err)

		chunkData := unittest.ChunkExecutionDataFixture(t, 100, unittest.WithTrieUpdate(trieUpdate))
		execData := unittest.BlockExecutionDataFixture(
			unittest.WithBlockExecutionDataBlockID(block.ID()),
			unittest.WithChunkExecutionDatas(chunkData),
		)

		// Add execution data to store
		executionDataID, err := executionDataStore.Add(context.Background(), execData)
		require.NoError(t, err)

		// Verify the execution data can be retrieved
		retrievedData, err := executionDataStore.Get(context.Background(), executionDataID)
		require.NoError(t, err)
		require.NotNil(t, retrievedData)

		// Create execution result
		result := unittest.ExecutionResultFixture(
			unittest.WithBlock(block),
		)
		result.ExecutionDataID = executionDataID

		// Set up mock to return execution result
		// Use mock.MatchedBy to match any block ID for this block's height
		resultsReader.On("ByBlockID", mock.MatchedBy(func(blockID flow.Identifier) bool {
			return blockID == block.ID()
		})).Return(result, nil).Maybe()
	}

	// Set up block executed notifier and follower distributor
	blockExecutedNotifier := ingestion.NewBlockExecutedNotifier()
	followerDistributor := pubsub.NewFollowerDistributor()
	heightsPerSecond := uint64(10)

	// Create background indexer engine
	engine, created, err := storehouse.LoadBackgroundIndexerEngine(
		log,
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
		heightsPerSecond,
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

	// Finalize blocks one by one and wait for each to propagate
	// The finalized reader subscribes to protocolEvents during bootstrapping, so it will receive these events
	// We need to finalize them sequentially so the finalized reader's lastHeight is updated correctly
	// The FinalizedReader's BlockFinalized method is called synchronously, so we don't need long waits
	for i := 0; i < numBlocks; i++ {
		block := blocks[i]
		protocolEvents.BlockFinalized(block.ToHeader())
		// Small wait to ensure the event is processed
		time.Sleep(50 * time.Millisecond)
	}

	// Wait a bit for any async processing
	time.Sleep(500 * time.Millisecond)

	// Update finalized snapshot to return the last finalized block AFTER finalizing
	// This allows the background indexer to know which blocks are finalized
	// Remove the previous mock and set a new one that returns height 105
	finalizedBlock := blocks[numBlocks-1]
	finalSnapshot.ExpectedCalls = nil // Clear previous expectations
	finalSnapshot.On("Head").Return(finalizedBlock.ToHeader(), nil)

	// Notify the follower distributor to trigger the background indexer
	for i := 0; i < numBlocks; i++ {
		block := blocks[i]
		hotstuffBlock := &model.Block{
			BlockID:    block.ID(),
			View:       block.ToHeader().View,
			ProposerID: unittest.IdentifierFixture(),
		}
		followerDistributor.OnFinalizedBlock(hotstuffBlock)
	}

	// Wait for finalization events to propagate
	time.Sleep(500 * time.Millisecond)

	// Notify that blocks were executed (this triggers indexing)
	// The background indexer will process all finalized and executed blocks sequentially
	// We may need to trigger multiple times as blocks get processed
	for attempt := 0; attempt < 10; attempt++ {
		blockExecutedNotifier.OnExecuted()
		time.Sleep(200 * time.Millisecond)

		// Check if we've reached the target height
		select {
		case reachedHeight := <-heightReached:
			require.Equal(t, targetHeight, reachedHeight, "expected target height %d to be reached", targetHeight)
			goto indexingComplete
		default:
			// Continue trying
		}
	}

	// Final attempt - wait for the target height to be reached
	unittest.RequireReturnsBefore(t, func() {
		select {
		case reachedHeight := <-heightReached:
			require.Equal(t, targetHeight, reachedHeight, "expected target height %d to be reached", targetHeight)
		case <-time.After(30 * time.Second):
			t.Fatal("timeout waiting for target height to be reached")
		}
	}, 35*time.Second, "all blocks should be indexed and metrics notified")

indexingComplete:

	// Stop the engine
	cancel()
	unittest.RequireReturnsBefore(t, func() {
		<-engine.Done()
	}, 5*time.Second, "engine should stop")

	// Wait a bit for the database to be fully closed
	time.Sleep(100 * time.Millisecond)

	// Verify register store has indexed all blocks
	// Initialize with the target height so the FinalizedReader knows about all finalized blocks
	registerStore, closer, err := storehouse.LoadRegisterStore(
		log,
		state,
		headers,
		protocolEvents,
		targetHeight, // Use targetHeight instead of registerStoreStart so FinalizedReader knows about all blocks
		collector,
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

	// The register store should have indexed up to the target height
	latestHeight := registerStore.LastFinalizedAndExecutedHeight()
	require.GreaterOrEqual(t, latestHeight, targetHeight,
		"register store should have indexed at least %d heights (from %d to %d), but got %d",
		numBlocks, registerStoreStart+1, targetHeight, latestHeight)
}
