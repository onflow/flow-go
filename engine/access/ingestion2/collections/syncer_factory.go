package collections

import (
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/ingestion2"
	"github.com/onflow/flow-go/engine/common/requester"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
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
	// EDILagThreshold is the threshold in blocks. If (blockHeight - ediHeight) > threshold, fetch collections.
	// Set to a very large value to effectively disable fetching and rely only on EDI.
	EDILagThreshold uint64
}

// CreateSyncerResult holds the results of CreateSyncer.
type CreateSyncerResult struct {
	Syncer       *Syncer
	JobProcessor ingestion2.JobProcessor
	RequestEng   *requester.Engine
}

// CreateSyncer creates a new Syncer component with all its dependencies.
// This function is in the collections package to avoid import cycles:
// - collections package already imports ingestion2 (for interfaces)
// - CreateSyncer needs to create concrete types from collections package
// - Placing it in ingestion2 would create: ingestion2 -> collections -> ingestion2 (cycle)
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
//   - lockManager: Lock manager for coordinating database access
//   - processedFinalizedBlockHeight: Initializer for tracking processed block heights
//   - collectionExecutedMetric: Metrics collector for tracking collection indexing
//   - processedHeightRecorder: Recorder for execution data processed heights (can be nil if EDI is disabled)
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
	collections storage.Collections,
	guarantees storage.Guarantees,
	db storage.DB,
	lockManager lockctx.Manager,
	processedFinalizedBlockHeight storage.ConsumerProgressInitializer,
	collectionExecutedMetric module.CollectionExecutedMetric,
	processedHeightRecorder execution_data.ProcessedHeightRecorder,
	config CreateSyncerConfig,
) (*CreateSyncerResult, error) {
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
		return nil, fmt.Errorf("could not create requester engine: %w", err)
	}

	// Create MissingCollectionQueue
	mcq := NewMissingCollectionQueue()

	// Create BlockCollectionIndexer
	indexer := NewBlockCollectionIndexer(
		collectionExecutedMetric,
		lockManager,
		db,
		collections,
	)

	// Create CollectionRequester
	collectionRequester := NewCollectionRequester(
		requestEng,
		state,
		guarantees,
	)

	// Wrap ProcessedHeightRecorder as EDIHeightProvider if provided
	var ediHeightProvider ingestion2.EDIHeightProvider
	if processedHeightRecorder != nil {
		ediHeightProvider = NewProcessedHeightRecorderWrapper(processedHeightRecorder)
	}

	// Create JobProcessor
	jobProcessor := NewJobProcessor(
		mcq,
		indexer,
		collectionRequester,
		blocks,
		collections,
		ediHeightProvider,
		config.EDILagThreshold,
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
	syncer, err := NewSyncer(
		log,
		jobProcessor,
		processedFinalizedBlockHeight,
		state,
		blocks,
		config.MaxProcessing,
		config.MaxSearchAhead,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create syncer: %w", err)
	}

	return &CreateSyncerResult{
		Syncer:       syncer,
		JobProcessor: jobProcessor,
		RequestEng:   requestEng,
	}, nil
}
