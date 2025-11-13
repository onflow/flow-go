package node_builder

import (
	"fmt"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/ingestion2"
	ingestion2collections "github.com/onflow/flow-go/engine/access/ingestion2/collections"
	ingestion2factory "github.com/onflow/flow-go/engine/access/ingestion2/factory"
	subscriptiontracker "github.com/onflow/flow-go/engine/access/subscription/tracker"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage/store"
)

// InitializeIngestionEngine creates and initializes the ingestion2 engine with collections.Syncer.
// This function should be called from within a Component function where node is available.
func InitializeIngestionEngine(builder *FlowAccessNodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
	// CollectionSyncer2 should already be created in the "collection syncer and job processor" module
	if builder.CollectionSyncer2 == nil {
		return nil, fmt.Errorf("CollectionSyncer2 must be created before ingestion engine")
	}

	// Create processedFinalizedBlockHeight if not already created
	processedFinalizedBlockHeight := store.NewConsumerProgress(builder.ProtocolDB, module.ConsumeProgressIngestionEngineBlockHeight)

	// Create FinalizedBlockProcessor
	finalizedBlockProcessor, err := ingestion2.NewFinalizedBlockProcessor(
		node.Logger,
		node.State,
		node.Storage.Blocks,
		node.Storage.Results,
		processedFinalizedBlockHeight,
		notNil(builder.collectionExecutedMetric),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create finalized block processor: %w", err)
	}

	// Create ingestion2 engine
	ingestEng, err := ingestion2.New(
		node.Logger,
		node.EngineRegistry,
		finalizedBlockProcessor,
		builder.CollectionSyncer2,
		node.Storage.Receipts,
		notNil(builder.collectionExecutedMetric),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create ingestion2 engine: %w", err)
	}

	builder.IngestEng = ingestEng

	// Create and initialize ingestionDependable if IndexerDependencies exists
	if builder.IndexerDependencies != nil {
		ingestionDependable := module.NewProxiedReadyDoneAware()
		builder.IndexerDependencies.Add(ingestionDependable)
		ingestionDependable.Init(ingestEng)
	}

	// Add OnFinalizedBlock consumer if FollowerDistributor exists
	if builder.FollowerDistributor != nil {
		builder.FollowerDistributor.AddOnBlockFinalizedConsumer(ingestEng.OnFinalizedBlock)
	}

	return ingestEng, nil
}

// InitializeExecutionDataCollectionIndexer creates and initializes the execution data collection indexer
// with ExecutionDataProcessor. This function should be called from within a Component function where node is available.
func InitializeExecutionDataCollectionIndexer(builder *FlowAccessNodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
	// ExecutionDataCache should already be created in BuildExecutionSyncComponents
	if builder.ExecutionDataCache == nil {
		return nil, fmt.Errorf("ExecutionDataCache must be created before execution data processor")
	}

	// Create execution data tracker for the processor
	// This is similar to the one created in state stream engine but used for collection indexing
	broadcaster := engine.NewBroadcaster()
	highestAvailableHeight, err := builder.ExecutionDataRequester.HighestConsecutiveHeight()
	if err != nil {
		return nil, fmt.Errorf("could not get highest consecutive height: %w", err)
	}

	useIndex := builder.executionDataIndexingEnabled
	executionDataTracker := subscriptiontracker.NewExecutionDataTracker(
		builder.Logger,
		node.State,
		builder.executionDataConfig.InitialBlockHeight,
		node.Storage.Headers,
		broadcaster,
		highestAvailableHeight,
		builder.EventsIndex,
		useIndex,
	)

	// Initialize processed height
	rootBlockHeight := node.State.Params().FinalizedRoot().Height
	syncAndIndexedCollectionsBlockHeight := store.NewConsumerProgress(builder.ProtocolDB, module.ConsumeProgressAccessSyncAndIndexedCollectionsBlockHeight)
	processedHeight, err := syncAndIndexedCollectionsBlockHeight.Initialize(rootBlockHeight)
	if err != nil {
		return nil, fmt.Errorf("could not initialize processed height: %w", err)
	}

	// Create BlockCollectionIndexer
	blockCollectionIndexer := ingestion2collections.NewBlockCollectionIndexer(
		notNil(builder.collectionExecutedMetric),
		node.StorageLockMgr,
		builder.ProtocolDB,
		notNil(builder.collections),
	)

	// Create execution data processor
	executionDataProcessor, err := ingestion2factory.CreateExecutionDataProcessor(
		notNil(builder.ExecutionDataCache),
		executionDataTracker,
		processedHeight,
		blockCollectionIndexer,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create execution data processor: %w", err)
	}

	// Setup requester to notify processor when new execution data is received
	if builder.ExecutionDataDistributor != nil {
		builder.ExecutionDataDistributor.AddOnExecutionDataReceivedConsumer(func(executionData *execution_data.BlockExecutionDataEntity) {
			executionDataTracker.OnExecutionData(executionData)
			executionDataProcessor.OnNewExectuionData()
		})
	}

	return executionDataProcessor, nil
}
