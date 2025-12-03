package factory

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/engine/access/collection_sync/fetcher"
	"github.com/onflow/flow-go/engine/common/requester"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// createFetcher creates a new Fetcher component with all its dependencies.
//
// Parameters:
//   - log: Logger for the component
//   - engineMetrics: Metrics collector for the requester engine
//   - engineRegistry: Engine registry for creating the requester engine
//   - state: Protocol state
//   - me: Local node identity
//   - blocks: Blocks storage
//   - processedFinalizedBlockHeight: Initializer for tracking processed block heights
//   - collectionSyncMetrics: Optional metrics collector for tracking collection sync progress
//   - lastFullBlockHeight: Progress reader to get the max of fetcher and syncer heights
//   - accessMetrics: Access metrics for reporting last_full_finalized_block_height
//   - config: Configuration for the fetcher
//
// Returns both the Fetcher and BlockProcessor so they can be reused in other components.
//
// No error returns are expected during normal operation.
func createFetcher(
	log zerolog.Logger,
	engineMetrics module.EngineMetrics,
	engineRegistry network.EngineRegistry,
	state protocol.State,
	me module.Local,
	blocks storage.Blocks,
	indexer collection_sync.BlockCollectionIndexer,
	processedFinalizedBlockHeight storage.ConsumerProgressInitializer,
	distributor hotstuff.Distributor,
	collectionSyncMetrics module.CollectionSyncMetrics,
	lastFullBlockHeight collection_sync.ProgressReader,
	accessMetrics module.AccessMetrics,
	config CreateFetcherConfig,
) (*requester.Engine, collection_sync.Fetcher, error) {
	// Create requester engine for requesting collections
	requestEng, err := requester.New(
		log.With().Str("entity", "collection").Logger(),
		engineMetrics,
		engineRegistry,
		me,
		state,
		channels.RequestCollections,
		filter.HasRole[flow.Identity](flow.RoleCollection),
		func() flow.Entity { return new(flow.Collection) },
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create requester engine: %w", err)
	}

	// Create MissingCollectionQueue
	mcq := fetcher.NewMissingCollectionQueue()

	// Create CollectionRequester
	collectionRequester := fetcher.NewCollectionRequester(
		requestEng,
		state,
	)

	// Create BlockProcessor
	blockProcessor := fetcher.NewBlockProcessor(
		log,
		mcq,
		indexer,
		collectionRequester,
		blocks,
	)

	// Register handler for received collections
	requestEng.WithHandle(func(originID flow.Identifier, entity flow.Entity) {
		collection, ok := entity.(*flow.Collection)
		if !ok {
			return
		}

		// Forward collection to BlockProcessor, which handles MCQ, indexing, and completion
		err := blockProcessor.OnReceiveCollection(originID, collection)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to process received collection")
			return
		}
	})

	// Create Fetcher with callback to update last_full_finalized_block_height metric
	collectionFetcher, err := fetcher.NewFetcher(
		log,
		blockProcessor,
		processedFinalizedBlockHeight,
		state,
		blocks,
		config.MaxProcessing,
		config.MaxSearchAhead,
		collectionSyncMetrics,
		func() {
			// Update last_full_finalized_block_height metric with the max of both heights
			accessMetrics.UpdateLastFullBlockHeight(lastFullBlockHeight.ProcessedHeight())
		},
		config.RetryInterval,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create fetcher: %w", err)
	}

	accessMetrics.UpdateLastFullBlockHeight(lastFullBlockHeight.ProcessedHeight())

	distributor.AddOnBlockFinalizedConsumer(func(_ *model.Block) {
		collectionFetcher.OnFinalizedBlock()
	})

	return requestEng, collectionFetcher, nil
}
