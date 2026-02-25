package ingestion2

import (
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/ingestion/collections"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

const (
	// finalizedBlockProcessorWorkerCount defines the number of workers that
	// concurrently process finalized blocks in the job queue.
	// MUST be 1 to ensure sequential processing
	finalizedBlockProcessorWorkerCount = 1

	// searchAhead is a number of blocks that should be processed ahead by jobqueue
	// MUST be 1 to ensure sequential processing
	searchAhead = 1
)

// FinalizedBlockProcessor handles processing of finalized blocks,
// including indexing and syncing of related collections and execution results.
//
// FinalizedBlockProcessor is designed to handle the ingestion of finalized Flow blocks
// in a scalable and decoupled manner. It uses a jobqueue.ComponentConsumer to consume
// and process finalized block jobs asynchronously. This design enables the processor
// to handle high-throughput block finalization events without blocking other parts
// of the system.
//
// The processor relies on a notifier (engine.Notifier) to signal when a new finalized
// block is available, which triggers the job consumer to process it. The actual
// processing involves indexing block-to-collection and block-to-execution-result
// mappings, as well as requesting the associated collections.
type FinalizedBlockProcessor struct {
	log zerolog.Logger

	consumer         *jobqueue.ComponentConsumer
	consumerNotifier engine.Notifier
	lockManager      storage.LockManager
	db               storage.DB

	blocks           storage.Blocks
	executionResults storage.ExecutionResults

	collectionSyncer         *collections.Syncer
	collectionExecutedMetric module.CollectionExecutedMetric
}

// NewFinalizedBlockProcessor creates and initializes a new FinalizedBlockProcessor,
// setting up job consumer infrastructure to handle finalized block processing.
//
// No errors are expected during normal operations.
func NewFinalizedBlockProcessor(
	log zerolog.Logger,
	state protocol.State,
	lockManager storage.LockManager,
	db storage.DB,
	blocks storage.Blocks,
	executionResults storage.ExecutionResults,
	finalizedProcessedHeight storage.ConsumerProgress,
	syncer *collections.Syncer,
	collectionExecutedMetric module.CollectionExecutedMetric,
) (*FinalizedBlockProcessor, error) {
	reader := jobqueue.NewFinalizedBlockReader(state, blocks)

	consumerNotifier := engine.NewNotifier()
	processor := &FinalizedBlockProcessor{
		log:                      log,
		db:                       db,
		lockManager:              lockManager,
		blocks:                   blocks,
		executionResults:         executionResults,
		consumerNotifier:         consumerNotifier,
		collectionSyncer:         syncer,
		collectionExecutedMetric: collectionExecutedMetric,
	}

	var err error
	processor.consumer, err = jobqueue.NewComponentConsumer(
		log.With().Str("module", "ingestion_block_consumer").Logger(),
		consumerNotifier.Channel(),
		finalizedProcessedHeight,
		reader,
		processor.processFinalizedBlockJobCallback,
		finalizedBlockProcessorWorkerCount,
		searchAhead,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating finalized block jobqueue: %w", err)
	}

	return processor, nil
}

// Notify notifies the processor that a new finalized block is available for processing.
func (p *FinalizedBlockProcessor) Notify() {
	p.consumerNotifier.Notify()
}

// StartWorkerLoop begins processing of finalized blocks and signals readiness when initialization is complete.
func (p *FinalizedBlockProcessor) StartWorkerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	p.consumer.Start(ctx)

	err := util.WaitClosed(ctx, p.consumer.Ready())
	if err == nil {
		ready()
	}

	<-p.consumer.Done()
}

// processFinalizedBlockJobCallback is a jobqueue callback that processes a finalized block job.
func (p *FinalizedBlockProcessor) processFinalizedBlockJobCallback(
	ctx irrecoverable.SignalerContext,
	job module.Job,
	done func(),
) {
	block, err := jobqueue.JobToBlock(job)
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to convert job to block: %w", err))
		return
	}

	err = p.indexFinalizedBlock(block)
	if err != nil {
		p.log.Error().Err(err).
			Str("job_id", string(job.ID())).
			Msg("unexpected error during finalized block processing job")
		ctx.Throw(fmt.Errorf("failed to index finalized block: %w", err))
		return
	}

	done()
}

// indexFinalizedBlock indexes the given finalized block’s collection guarantees and execution results,
// and requests related collections from the syncer.
//
// No errors are expected during normal operations.
func (p *FinalizedBlockProcessor) indexFinalizedBlock(block *flow.Block) error {
	err := storage.WithLocks(p.lockManager, storage.LockGroupAccessFinalizingBlock,
		func(lctx lockctx.Context) error {
			return p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				// require storage.LockIndexBlockByPayloadGuarantees
				err := p.blocks.BatchIndexBlockContainingCollectionGuarantees(lctx, rw, block.ID(), flow.GetIDs(block.Payload.Guarantees))
				if err != nil {
					return fmt.Errorf("could not index block for collections: %w", err)
				}

				// loop through seals and index ID -> result ID
				for _, seal := range block.Payload.Seals {
					// require storage.LockIndexExecutionResult
					err := p.executionResults.BatchIndex(lctx, rw, seal.BlockID, seal.ResultID)
					if err != nil {
						return fmt.Errorf("could not index block for execution result: %w", err)
					}
				}
				return nil
			})
		})
	if err != nil {
		return fmt.Errorf("could not index execution results: %w", err)
	}

	err = p.collectionSyncer.RequestCollectionsForBlock(block.Height, block.Payload.Guarantees)
	if err != nil {
		return fmt.Errorf("could not request collections for block: %w", err)
	}

	p.collectionExecutedMetric.BlockFinalized(block)

	return nil
}
