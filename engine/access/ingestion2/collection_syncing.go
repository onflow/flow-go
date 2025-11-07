package ingestion2

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Tracks missing collections per height and invokes job callbacks when complete.
type MissingCollectionQueue interface {
	EnqueueMissingCollections(blockHeight uint64, ids []flow.Identifier, callback func()) error
	OnIndexedForBlock(blockHeight uint64) // mark done (post‑indexing)

	// On receipt of a collection, MCQ updates internal state and, if a block
	// just became complete, returns: (collections, height, true).
	// Otherwise, returns (nil, 0, false).
	OnReceivedCollection(collection *flow.Collection) ([]*flow.Collection, uint64, bool)
}

// Requests collections by their IDs.
type CollectionRequester interface {
	RequestCollections(ids []flow.Identifier) error
}

// BlockCollectionIndexer stores and indexes collections for a given block height.
type BlockCollectionIndexer interface {
	// OnReceivedCollectionsForBlock stores and indexes collections for a given block height.
	OnReceivedCollectionsForBlock(blockHeight uint64, cols []*flow.Collection) error
}

// Implements the job lifecycle for a single block height.
type JobProcessor interface {
	ProcessJob(ctx irrecoverable.SignalerContext, job module.Job, done func()) error
	OnReceivedCollectionsForBlock(blockHeight uint64, cols []*flow.Collection) error // called by EDI or requester
}

// EDIHeightProvider provides the latest height for which execution data indexer has collections.
// This can be nil if execution data indexing is disabled.
type EDIHeightProvider interface {
	HighestIndexedHeight() (uint64, error)
}

// Syncer is a component that consumes finalized block jobs and processes them
// to index collections. It uses a job consumer with windowed throttling to prevent node overload.
type Syncer struct {
	component.Component

	consumer     *jobqueue.ComponentConsumer
	jobProcessor JobProcessor
	workSignal   engine.Notifier
}

// NewSyncer creates a new Syncer component.
//
// Parameters:
//   - log: Logger for the component
//   - jobProcessor: JobProcessor implementation for processing collection indexing jobs
//   - progressInitializer: Initializer for tracking processed block heights
//   - state: Protocol state for reading finalized block information
//   - blocks: Blocks storage for reading blocks by height
//   - maxProcessing: Maximum number of jobs to process concurrently
//   - maxSearchAhead: Maximum number of jobs beyond processedIndex to process. 0 means no limit
//
// No error returns are expected during normal operation.
func NewSyncer(
	log zerolog.Logger,
	jobProcessor JobProcessor,
	progressInitializer storage.ConsumerProgressInitializer,
	state protocol.State,
	blocks storage.Blocks,
	maxProcessing uint64, // max number of blocks to fetch collections
	maxSearchAhead uint64, // max number of blocks beyond the next unfullfilled height to fetch collections for
) (*Syncer, error) {
	workSignal := engine.NewNotifier()

	// Read the default index from the finalized root height
	defaultIndex := state.Params().FinalizedRoot().Height

	// Create a Jobs instance that reads finalized blocks by height
	jobs := jobqueue.NewFinalizedBlockReader(state, blocks)

	// Create an adapter function that wraps the JobProcessor interface
	processorFunc := func(ctx irrecoverable.SignalerContext, job module.Job, done func()) {
		err := jobProcessor.ProcessJob(ctx, job, done)
		if err != nil {
			ctx.Throw(fmt.Errorf("failed to process collection indexing job: %w", err))
		}
	}

	consumer, err := jobqueue.NewComponentConsumer(
		log.With().Str("component", "collection-syncing").Logger(),
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

	return &Syncer{
		Component:    consumer,
		consumer:     consumer,
		jobProcessor: jobProcessor,
		workSignal:   workSignal,
	}, nil
}

// OnFinalizedBlock is called when a new block is finalized. It notifies the job consumer
// that new work is available.
func (s *Syncer) OnFinalizedBlock() {
	s.workSignal.Notify()
}

// LastProcessedIndex returns the last processed job index.
func (s *Syncer) LastProcessedIndex() uint64 {
	return s.consumer.LastProcessedIndex()
}

// Head returns the highest job index available.
func (s *Syncer) Head() (uint64, error) {
	return s.consumer.Head()
}

// Size returns the number of in-memory jobs that the consumer is processing.
func (s *Syncer) Size() uint {
	return s.consumer.Size()
}
