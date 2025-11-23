package fetcher

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Fetcher is a component that consumes finalized block jobs and processes them
// to index collections. It uses a job consumer with windowed throttling to prevent node overload.
type Fetcher struct {
	component.Component

	consumer       *jobqueue.ComponentConsumer
	blockProcessor collection_sync.BlockProcessor
	workSignal     engine.Notifier
	metrics        module.CollectionSyncMetrics
}

var _ collection_sync.Fetcher = (*Fetcher)(nil)
var _ collection_sync.ProgressReader = (*Fetcher)(nil)
var _ component.Component = (*Fetcher)(nil)

// NewFetcher creates a new Fetcher component.
//
// Parameters:
//   - log: Logger for the component
//   - blockProcessor: BlockProcessor implementation for processing collection indexing jobs
//   - progressInitializer: Initializer for tracking processed block heights
//   - state: Protocol state for reading finalized block information
//   - blocks: Blocks storage for reading blocks by height
//   - maxProcessing: Maximum number of jobs to process concurrently
//   - maxSearchAhead: Maximum number of jobs beyond processedIndex to process. 0 means no limit
//   - metrics: Optional metrics collector for reporting collection fetched height
//
// No error returns are expected during normal operation.
func NewFetcher(
	log zerolog.Logger,
	blockProcessor collection_sync.BlockProcessor, progressInitializer storage.ConsumerProgressInitializer,
	state protocol.State,
	blocks storage.Blocks,
	maxProcessing uint64, // max number of blocks to fetch collections
	maxSearchAhead uint64, // max number of blocks beyond the next unfullfilled height to fetch collections for
	metrics module.CollectionSyncMetrics, // optional metrics collector
) (*Fetcher, error) {
	workSignal := engine.NewNotifier()

	// Read the default index from the sealed root height
	defaultIndex := state.Params().SealedRoot().Height

	// Create a Jobs instance that reads finalized blocks by height
	// each job is a finalized block
	jobs := jobqueue.NewFinalizedBlockReader(state, blocks)

	// Create an adapter function that wraps the BlockProcessor interface
	// For each finalized block, a worker is responsible for fetching all collections for that block.
	// Since a block may contain multiple collections and fetching is asynchronous, tracking which
	// collections have been received requires stateful management. The blockProcessor handles this
	// by fetching collections and maintaining state about which collections are still missing.
	// It also invokes the done callback to notify the job consumer when the worker has completed
	// processing all collections for the block.
	//
	// The processor function translates a job (finalized block) and passes it to the blockProcessor
	// to fetch and index the associated collections.
	processorFunc := func(ctx irrecoverable.SignalerContext, job module.Job, done func()) {
		// Convert job to block
		block, err := jobs.ConvertJobToBlock(job)
		if err != nil {
			ctx.Throw(fmt.Errorf("could not convert job to block: %w", err))
			return
		}

		err = blockProcessor.FetchCollections(ctx, block, done)
		if err != nil {
			ctx.Throw(fmt.Errorf("failed to process collection indexing job: %w", err))
		}
	}

	consumer, err := jobqueue.NewComponentConsumer(
		log.With().Str("coll_sync", "fetcher").Logger(),
		workSignal.Channel(),
		progressInitializer,
		jobs,
		defaultIndex,
		processorFunc,
		maxProcessing,
		maxSearchAhead,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection syncing consumer: %w", err)
	}

	f := &Fetcher{
		Component:      consumer,
		consumer:       consumer,
		blockProcessor: blockProcessor,
		workSignal:     workSignal,
		metrics:        metrics,
	}

	// Set up post-notifier to update metrics when a job is done
	// Only update metrics when the processed height actually changes, since processedIndex
	// only advances when consecutive jobs complete, not on every individual job completion.
	consumer.SetPostNotifier(func(jobID module.JobID) {
		metrics.CollectionFetchedHeight(f.ProcessedHeight())
		metrics.MissingCollectionQueueSize(f.blockProcessor.MissingCollectionQueueSize())
	})

	return f, nil
}

// OnFinalizedBlock is called when a new block is finalized. It notifies the job consumer
// that new work is available.
func (s *Fetcher) OnFinalizedBlock() {
	s.workSignal.Notify()
}

// LastProcessedIndex returns the last processed job index.
func (s *Fetcher) ProcessedHeight() uint64 {
	return s.consumer.LastProcessedIndex()
}

// Size returns the number of in-memory jobs that the consumer is processing.
// Optional methods, not required for operation but useful for monitoring.
func (s *Fetcher) Size() uint {
	return s.consumer.Size()
}
