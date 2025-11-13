package factory

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/engine/access/collection_sync/fetcher"
	"github.com/onflow/flow-go/engine/common/requester"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// CreateSyncerConfig holds configuration parameters for creating a Syncer.
type CreateSyncerConfig struct {
	// MaxProcessing is the maximum number of jobs to process concurrently.
	MaxProcessing uint64
	// MaxSearchAhead is the maximum number of jobs beyond processedIndex to process. 0 means no limit.
	MaxSearchAhead uint64
}

// CreateSyncer creates a new Syncer component with all its dependencies.
// This function is in the collections package to avoid import cycles:
// - collections package already imports collection_sync (for interfaces)
// - CreateSyncer needs to create concrete types from collections package
// - Placing it in collection_sync would create: collection_sync -> collections -> collection_sync (cycle)
//
// Parameters:
//   - log: Logger for the component
//   - engineRegistry: Engine registry for creating the requester engine
//   - state: Protocol state
//   - me: Local node identity
//   - blocks: Blocks storage
//   - collections: Collections storage
//   - guarantees: Guarantees storage
//   - db: Database for storage operations
//   - processedFinalizedBlockHeight: Initializer for tracking processed block heights
//   - collectionExecutedMetric: Metrics collector for tracking collection indexing
//   - config: Configuration for the syncer
//
// Returns both the Syncer and JobProcessor so they can be reused in other components.
//
// No error returns are expected during normal operation.
func CreateSyncer(
	log zerolog.Logger,
	engineRegistry network.EngineRegistry,
	state protocol.State,
	me module.Local,
	blocks storage.Blocks,
	collStore storage.Collections,
	guarantees storage.Guarantees,
	db storage.DB,
	indexer collection_sync.BlockCollectionIndexer,
	processedFinalizedBlockHeight storage.ConsumerProgressInitializer,
	collectionExecutedMetric module.CollectionExecutedMetric,
	config CreateSyncerConfig,
) (*requester.Engine, collection_sync.Syncer, error) {
	// Create requester engine for requesting collections
	requestEng, err := requester.New(
		log.With().Str("entity", "collection").Logger(),
		metrics.NewNoopCollector(), // TODO: pass proper metrics if available
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
		guarantees,
	)

	// Create JobProcessor
	jobProcessor := fetcher.NewJobProcessor(
		mcq,
		indexer,
		collectionRequester,
		blocks,
		collStore,
	)

	// Register handler for received collections
	requestEng.WithHandle(func(originID flow.Identifier, entity flow.Entity) {
		collection, ok := entity.(*flow.Collection)
		if !ok {
			return
		}

		// Forward collection to JobProcessor, which handles MCQ, indexing, and completion
		err := jobProcessor.OnReceiveCollection(collection)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to process received collection")
			return
		}
	})

	// Create Syncer
	syncer, err := fetcher.NewSyncer(
		log,
		jobProcessor,
		processedFinalizedBlockHeight,
		state,
		blocks,
		config.MaxProcessing,
		config.MaxSearchAhead,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create syncer: %w", err)
	}

	return requestEng, syncer, nil
}
