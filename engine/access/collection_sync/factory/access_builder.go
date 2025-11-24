package factory

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/state_synchronization"
	edrequester "github.com/onflow/flow-go/module/state_synchronization/requester"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

// ProcessedLastFullBlockHeightModuleResult contains the results from creating the processed last full block height module.
type ProcessedLastFullBlockHeightModuleResult struct {
	LastFullBlockHeight     *ProgressReader
	CollectionIndexedHeight storage.ConsumerProgress
}

// CreateProcessedLastFullBlockHeightModule initializes and syncs the progress trackers for collection sync.
//
// Parameters:
//   - log: Logger for logging operations
//   - state: Protocol state to get root block height
//   - db: Database for storing progress
//
// Returns:
//   - The result containing the ProgressReader and collection indexed height
//   - An error if the initialization fails
func CreateProcessedLastFullBlockHeightModule(
	log zerolog.Logger,
	state protocol.State,
	db storage.DB,
) (*ProcessedLastFullBlockHeightModuleResult, error) {
	rootBlockHeight := state.Params().FinalizedRoot().Height

	// Initialize ConsumeProgressLastFullBlockHeight
	progress, err := store.NewConsumerProgress(db, module.ConsumeProgressLastFullBlockHeight).Initialize(rootBlockHeight)
	if err != nil {
		return nil, err
	}

	lastProgress, err := progress.ProcessedIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to get last processed index for last full block height: %w", err)
	}

	// Sync ConsumeProgressLastFullBlockHeight and ConsumeProgressAccessFetchAndIndexedCollectionsBlockHeight
	// by taking the max value of each and updating both
	fetchAndIndexedTracker := store.NewConsumerProgress(db, module.ConsumeProgressAccessFetchAndIndexedCollectionsBlockHeight)
	fetchAndIndexed, err := fetchAndIndexedTracker.Initialize(rootBlockHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize fetch and indexed collections block height tracker: %w", err)
	}
	fetchAndIndexedValue, err := fetchAndIndexed.ProcessedIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to get fetch and indexed collections block height: %w", err)
	}

	// Take the max of both values
	maxValue := max(lastProgress, fetchAndIndexedValue)

	// Update both trackers if needed
	if lastProgress < maxValue {
		if err := progress.SetProcessedIndex(maxValue); err != nil {
			return nil, fmt.Errorf("failed to update last full block height: %w", err)
		}
		log.Info().
			Uint64("old_value", lastProgress).
			Uint64("new_value", maxValue).
			Str("tracker", module.ConsumeProgressLastFullBlockHeight).
			Msg("synced collection sync progress tracker")
	}

	if fetchAndIndexedValue < maxValue {
		if err := fetchAndIndexed.SetProcessedIndex(maxValue); err != nil {
			return nil, fmt.Errorf("failed to update fetch and indexed collections block height: %w", err)
		}
		log.Info().
			Uint64("old_value", fetchAndIndexedValue).
			Uint64("new_value", maxValue).
			Str("tracker", module.ConsumeProgressAccessFetchAndIndexedCollectionsBlockHeight).
			Msg("synced collection sync progress tracker")
	}

	if lastProgress == maxValue && fetchAndIndexedValue == maxValue {
		log.Info().
			Uint64("value", maxValue).
			Msg("collection sync progress trackers already in sync")
	}

	// Get the final synced value for ProgressReader
	finalProgress, err := progress.ProcessedIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to get final synced progress: %w", err)
	}

	// Create ProgressReader that aggregates progress from executionDataProcessor and collectionFetcher
	return &ProcessedLastFullBlockHeightModuleResult{
		LastFullBlockHeight:     NewProgressReader(finalProgress),
		CollectionIndexedHeight: progress,
	}, nil
}

// CreateExecutionDataProcessorComponent creates an execution data processor component.
// It creates an execution data processor if execution data sync is enabled and collection sync mode is not "collection_only".
//
// Parameters:
//   - log: Logger for logging operations
//   - executionDataSyncEnabled: Whether execution data sync is enabled
//   - collectionSyncMode: The collection sync mode
//   - executionDataCache: Execution data cache
//   - executionDataRequester: Execution data requester
//   - collectionIndexedHeight: Consumer progress for collection indexed height
//   - blockCollectionIndexer: Block collection indexer
//   - collectionSyncMetrics: Collection sync metrics
//   - lastFullBlockHeight: Progress reader to register the processor with
//   - distributor: Execution data distributor to notify on new execution data
//
// Returns:
//   - The processor component (or NoopReadyDoneAware if not created)
//   - An error if the processor creation fails
func CreateExecutionDataProcessorComponent(
	log zerolog.Logger,
	executionDataSyncEnabled bool,
	collectionSyncMode CollectionSyncMode,
	executionDataCache execution_data.ExecutionDataCache,
	executionDataRequester state_synchronization.ExecutionDataRequester,
	collectionIndexedHeight storage.ConsumerProgress,
	blockCollectionIndexer collection_sync.BlockCollectionIndexer,
	collectionSyncMetrics module.CollectionSyncMetrics,
	lastFullBlockHeight *ProgressReader,
	distributor *edrequester.ExecutionDataDistributor,
) (module.ReadyDoneAware, error) {
	shouldCreate := collectionSyncMode.ShouldCreateExecutionDataProcessor(executionDataSyncEnabled)
	if !shouldCreate {
		// Log when execution data processor is not created
		if !executionDataSyncEnabled {
			log.Info().
				Str("collection_sync_mode", collectionSyncMode.String()).
				Bool("execution_data_sync_enabled", executionDataSyncEnabled).
				Msg("execution data processor not created: execution data sync is disabled")
		} else if collectionSyncMode == CollectionSyncModeCollectionOnly {
			log.Info().
				Str("collection_sync_mode", collectionSyncMode.String()).
				Bool("execution_data_sync_enabled", executionDataSyncEnabled).
				Msg("execution data processor not created: collection sync mode is 'collection_only'")
		}

		return &module.NoopReadyDoneAware{}, nil
	}

	log.Info().
		Str("collection_sync_mode", collectionSyncMode.String()).
		Bool("execution_data_sync_enabled", executionDataSyncEnabled).
		Msg("creating execution data processor")

	if executionDataCache == nil {
		return nil, fmt.Errorf("ExecutionDataCache must be created before execution data processor")
	}

	// Create execution data processor
	executionDataProcessor, err := createExecutionDataProcessor(
		log,
		executionDataCache,
		executionDataRequester,
		collectionIndexedHeight,
		blockCollectionIndexer,
		func(indexedHeight uint64) {
			collectionSyncMetrics.CollectionSyncedHeight(indexedHeight)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not create execution data processor: %w", err)
	}

	// Register with ProgressReader
	lastFullBlockHeight.SetExecutionDataProcessor(executionDataProcessor)

	distributor.AddOnExecutionDataReceivedConsumer(func(executionData *execution_data.BlockExecutionDataEntity) {
		executionDataProcessor.OnNewExectuionData()
	})

	return executionDataProcessor, nil
}

// CollectionSyncFetcherComponentResult contains the results from creating the collection sync fetcher component.
type CollectionSyncFetcherComponentResult struct {
	Fetcher   module.ReadyDoneAware
	Requester module.ReadyDoneAware
}

// CreateCollectionSyncFetcherComponent creates a collection fetcher and requester engine based on the collection sync mode.
//
// Parameters:
//   - log: Logger for logging operations
//   - executionDataSyncEnabled: Whether execution data sync is enabled
//   - collectionSyncMode: The collection sync mode
//   - engineMetrics: Engine metrics
//   - engineRegistry: Engine registry
//   - state: Protocol state
//   - me: Local node identity
//   - blocks: Blocks storage
//   - db: Database for storage operations
//   - blockCollectionIndexer: Block collection indexer
//   - followerDistributor: Follower distributor
//   - collectionExecutedMetric: Collection executed metric
//   - collectionSyncMetrics: Collection sync metrics
//   - maxProcessing: Maximum number of concurrent processing jobs
//   - maxSearchAhead: Maximum number of blocks to search ahead
//   - lastFullBlockHeight: Progress reader to register the fetcher with
//
// Returns:
//   - The result containing the fetcher component, requester component, and requester engine
//   - An error if the fetcher creation fails
func CreateCollectionSyncFetcherComponent(
	log zerolog.Logger,
	executionDataSyncEnabled bool,
	collectionSyncMode CollectionSyncMode,
	engineMetrics module.EngineMetrics,
	engineRegistry network.EngineRegistry,
	state protocol.State,
	me module.Local,
	blocks storage.Blocks,
	db storage.DB,
	blockCollectionIndexer collection_sync.BlockCollectionIndexer,
	followerDistributor hotstuff.Distributor,
	collectionExecutedMetric module.CollectionExecutedMetric,
	collectionSyncMetrics module.CollectionSyncMetrics,
	maxProcessing uint64,
	maxSearchAhead uint64,
	lastFullBlockHeight *ProgressReader,
) (*CollectionSyncFetcherComponentResult, error) {
	// Create fetcher if:
	// 1. collectionSync is "execution_and_collection" (always create, even with execution data sync)
	// 2. collectionSync is "collection_only" (always create)
	// 3. collectionSync is "execution_first" and execution data sync is disabled
	shouldCreateFetcher := collectionSyncMode.ShouldCreateFetcher(executionDataSyncEnabled)

	if !shouldCreateFetcher {
		// skip if execution data sync is enabled and not in execution_and_collection or collection_only mode
		// because the execution data contains the collections, so no need to fetch them separately.
		// otherwise, if both fetching and syncing are enabled, they might slow down each other,
		// because the database operation requires locking.
		log.Info().
			Str("collection_sync_mode", collectionSyncMode.String()).
			Bool("execution_data_sync_enabled", executionDataSyncEnabled).
			Msg("collection sync fetcher not created: execution data sync is enabled and collection sync mode is not 'execution_and_collection' or 'collection_only'")
		return &CollectionSyncFetcherComponentResult{
			Fetcher:   &module.NoopReadyDoneAware{},
			Requester: &module.NoopReadyDoneAware{},
		}, nil
	}

	log.Info().
		Str("collection_sync_mode", collectionSyncMode.String()).
		Bool("execution_data_sync_enabled", executionDataSyncEnabled).
		Msg("creating collection sync fetcher")

	// Fetcher always uses ConsumeProgressAccessFetchAndIndexedCollectionsBlockHeight
	// to avoid contention with execution data processor which uses ConsumeProgressLastFullBlockHeight
	fetchAndIndexedCollectionsBlockHeight := store.NewConsumerProgress(db, module.ConsumeProgressAccessFetchAndIndexedCollectionsBlockHeight)

	// Create fetcher and requesterEng
	requesterEng, fetcher, err := createFetcher(
		log,
		engineMetrics,
		engineRegistry,
		state,
		me,
		blocks,
		blockCollectionIndexer,
		fetchAndIndexedCollectionsBlockHeight,
		followerDistributor,
		collectionSyncMetrics,
		CreateFetcherConfig{
			MaxProcessing:  maxProcessing,
			MaxSearchAhead: maxSearchAhead,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("could not create collection fetcher: %w", err)
	}

	// Register with ProgressReader
	lastFullBlockHeight.SetCollectionFetcher(fetcher)

	return &CollectionSyncFetcherComponentResult{
		Fetcher:   fetcher,
		Requester: requesterEng,
	}, nil
}
