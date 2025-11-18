package finalized_indexer

import (
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// FinalizedBlockProcessor handles processing of finalized blocks,
// including indexing and syncing of related collections and execution results.
//
// FinalizedBlockProcessor is designed to handle the ingestion of finalized Flow blocks
// in a scalable and decoupled manner. It uses a worker loop with a for loop to iterate
// through heights sequentially, processing each finalized block. This design enables
// the processor to handle high-throughput block finalization events without blocking
// other parts of the system.
//
// The processor relies on the distributor to signal when a new finalized
// block is available, which triggers the worker loop to process any unprocessed blocks.
// The actual processing involves indexing block-to-collection and block-to-execution-result
// mappings, as well as requesting the associated collections.
type FinalizedBlockProcessor struct {
	log zerolog.Logger
	component.Component

	newBlockFinalized chan struct{}
	state             protocol.State
	blocks            storage.Blocks
	db                storage.DB
	lockManager       storage.LockManager
	processedProgress storage.ConsumerProgress

	collectionExecutedMetric module.CollectionExecutedMetric
}

// NewFinalizedBlockProcessor creates and initializes a new FinalizedBlockProcessor,
// setting up worker loop infrastructure to handle finalized block processing.
//
// No errors are expected during normal operations.
func NewFinalizedBlockProcessor(
	log zerolog.Logger,
	state protocol.State,
	lockManager storage.LockManager,
	db storage.DB,
	blocks storage.Blocks,
	finalizedProcessedHeight storage.ConsumerProgressInitializer,
	distributor hotstuff.Distributor,
	collectionExecutedMetric module.CollectionExecutedMetric,
) (*FinalizedBlockProcessor, error) {
	finalizedBlock, err := state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized block header: %w", err)
	}

	// Initialize the progress tracker
	processedProgress, err := finalizedProcessedHeight.Initialize(finalizedBlock.Height)
	if err != nil {
		return nil, fmt.Errorf("could not initialize processed height: %w", err)
	}

	processor := &FinalizedBlockProcessor{
		log:                      log.With().Str("component", "finalized_block_processor").Logger(),
		newBlockFinalized:        make(chan struct{}, 1),
		state:                    state,
		db:                       db,
		lockManager:              lockManager,
		blocks:                   blocks,
		processedProgress:        processedProgress,
		collectionExecutedMetric: collectionExecutedMetric,
	}

	// Initialize the channel so that even if no new blocks are finalized,
	// the worker loop can still be triggered to process any existing blocks.
	processor.newBlockFinalized <- struct{}{}

	distributor.AddOnBlockFinalizedConsumer(func(_ *model.Block) {
		select {
		case processor.newBlockFinalized <- struct{}{}:
		default:
			// if the channel is full, no need to block, just return.
			// once the worker loop processes the buffered signal, it will
			// process the next height all the way to the highest available height.
		}
	})

	// Build component manager with worker loop
	cm := component.NewComponentManagerBuilder().
		AddWorker(processor.workerLoop).
		Build()

	processor.Component = cm

	return processor, nil
}

// workerLoop processes finalized blocks sequentially using a for loop to iterate through heights.
func (p *FinalizedBlockProcessor) workerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	// using a single threaded loop to process each finalized block by height
	// since indexing collections is blocking anyway, and reading the blocks
	// is quick.
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.newBlockFinalized:
			finalizedHeader, err := p.state.Final().Head()
			if err != nil {
				ctx.Throw(fmt.Errorf("failed to get finalized block header: %w", err))
				return
			}
			highestAvailableHeight := finalizedHeader.Height

			processedHeight, err := p.processedProgress.ProcessedIndex()
			if err != nil {
				ctx.Throw(fmt.Errorf("failed to get processed height: %w", err))
				return
			}
			lowestMissing := processedHeight + 1

			for height := lowestMissing; height <= highestAvailableHeight; height++ {
				block, err := p.blocks.ByHeight(height)
				if err != nil {
					ctx.Throw(fmt.Errorf("failed to get block by height %d: %w", height, err))
					return
				}

				err = p.indexForFinalizedBlock(block)
				if err != nil {
					ctx.Throw(fmt.Errorf("failed to index finalized block at height %d: %w", height, err))
					return
				}

				// Update processed height after successful indexing
				err = p.processedProgress.SetProcessedIndex(height)
				if err != nil {
					ctx.Throw(fmt.Errorf("failed to update processed height to %d: %w", height, err))
					return
				}

				// Log progress for each height with all relevant information
				p.log.Info().
					Uint64("indexed", height).
					Uint64("lowest_missing", lowestMissing).
					Uint64("highest_available", highestAvailableHeight).
					Uint64("processed_count", height-lowestMissing+1).
					Uint64("remaining_count", highestAvailableHeight-height).
					Uint64("total_to_process", highestAvailableHeight-lowestMissing+1).
					Msg("indexed finalized block progress")
			}
		}
	}
}

// indexForFinalizedBlock indexes the given finalized block’s collection guarantees
//
// No errors are expected during normal operations.
func (p *FinalizedBlockProcessor) indexForFinalizedBlock(block *flow.Block) error {
	err := storage.WithLock(p.lockManager, storage.LockIndexBlockByPayloadGuarantees,
		func(lctx lockctx.Context) error {
			return p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				// require storage.LockIndexBlockByPayloadGuarantees
				err := p.blocks.BatchIndexBlockContainingCollectionGuarantees(lctx, rw, block.ID(), flow.GetIDs(block.Payload.Guarantees))
				if err != nil {
					return fmt.Errorf("could not index block for collections: %w", err)
				}

				return nil
			})
		})
	if err != nil {
		return fmt.Errorf("could not index execution results: %w", err)
	}

	p.collectionExecutedMetric.BlockFinalized(block)

	return nil
}
