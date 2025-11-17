package finalized_indexer

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
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
	component.Component

	consumer *jobqueue.ComponentConsumer
	blocks   storage.Blocks

	collectionExecutedMetric module.CollectionExecutedMetric
}

// NewFinalizedBlockProcessor creates and initializes a new FinalizedBlockProcessor,
// setting up job consumer infrastructure to handle finalized block processing.
//
// No errors are expected during normal operations.
func NewFinalizedBlockProcessor(
	log zerolog.Logger,
	state protocol.State,
	blocks storage.Blocks,
	finalizedProcessedHeight storage.ConsumerProgressInitializer,
	distributor hotstuff.Distributor,
	collectionExecutedMetric module.CollectionExecutedMetric,
) (*FinalizedBlockProcessor, error) {
	reader := jobqueue.NewFinalizedBlockReader(state, blocks)
	finalizedBlock, err := state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized block header: %w", err)
	}

	blockFinalizedNotifier := engine.NewNotifier()
	processor := &FinalizedBlockProcessor{
		log:                      log,
		blocks:                   blocks,
		collectionExecutedMetric: collectionExecutedMetric,
	}

	// TODO (leo): no need to use job queue consumer, instead, we could use a simple
	// for loop to go through from next unprocessed height to the latest height.
	processor.consumer, err = jobqueue.NewComponentConsumer(
		log.With().Str("module", "ingestion_block_consumer").Logger(),
		blockFinalizedNotifier.Channel(),
		finalizedProcessedHeight,
		reader,
		finalizedBlock.Height,
		processor.processFinalizedBlockJobCallback,
		finalizedBlockProcessorWorkerCount,
		searchAhead,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating finalized block jobqueue: %w", err)
	}

	distributor.AddOnBlockFinalizedConsumer(func(_ *model.Block) {
		blockFinalizedNotifier.Notify()
	})

	// Build component manager with worker loop
	cm := component.NewComponentManagerBuilder().
		AddWorker(processor.workerLoop).
		Build()

	processor.Component = cm

	return processor, nil
}

// StartWorkerLoop begins processing of finalized blocks and signals readiness when initialization is complete.
func (p *FinalizedBlockProcessor) workerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
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
	finalizedBlock, err := jobqueue.JobToBlock(job)
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to convert job to finalizedBlock: %w", err))
		return
	}

	err = p.indexForFinalizedBlock(finalizedBlock)
	if err != nil {
		p.log.Error().Err(err).
			Str("job_id", string(job.ID())).
			Msg("unexpected error during finalized block processing job")
		ctx.Throw(fmt.Errorf("failed to index finalized block: %w", err))
		return
	}

	done()
}

// indexForFinalizedBlock indexes the given finalized block’s collection guarantees
//
// No errors are expected during normal operations.
func (p *FinalizedBlockProcessor) indexForFinalizedBlock(block *flow.Block) error {
	err := p.blocks.IndexBlockContainingCollectionGuarantees(block.ID(), flow.GetIDs(block.Payload.Guarantees))
	if err != nil {
		return fmt.Errorf("could not index block for collections: %w", err)
	}

	p.collectionExecutedMetric.BlockFinalized(block)

	return nil
}
