package finalized_indexer

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// FinalizedBlockProcessor handles processing of finalized blocks,
// including indexing and syncing of related collections and execution results.
//
// FinalizedBlockProcessor processes finalized blocks sequentially using a simple loop
// that iterates from the last processed height to the latest finalized block height.
// When notified of a new finalized block, it processes all blocks up to the current
// finalized head height.
type FinalizedBlockProcessor struct {
	log zerolog.Logger

	state           protocol.State
	blocks          storage.Blocks
	processedHeight *counters.PersistentStrictMonotonicCounter

	blockFinalizedNotifier   chan struct{}
	collectionExecutedMetric module.CollectionExecutedMetric
}

// NewFinalizedBlockProcessor creates and initializes a new FinalizedBlockProcessor.
//
// No errors are expected during normal operations.
func NewFinalizedBlockProcessor(
	log zerolog.Logger,
	state protocol.State,
	blocks storage.Blocks,
	finalizedProcessedHeight storage.ConsumerProgressInitializer,
	collectionExecutedMetric module.CollectionExecutedMetric,
) (*FinalizedBlockProcessor, error) {
	finalizedBlock, err := state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized block header: %w", err)
	}

	processedHeightProgress, err := finalizedProcessedHeight.Initialize(finalizedBlock.Height)
	if err != nil {
		return nil, fmt.Errorf("could not initialize processed height: %w", err)
	}

	processedHeightCounter, err := counters.NewPersistentStrictMonotonicCounter(processedHeightProgress)
	if err != nil {
		return nil, fmt.Errorf("failed to create persistent strict monotonic counter: %w", err)
	}

	processor := &FinalizedBlockProcessor{
		log:                      log,
		state:                    state,
		blocks:                   blocks,
		processedHeight:          processedHeightCounter,
		blockFinalizedNotifier:   make(chan struct{}, 1),
		collectionExecutedMetric: collectionExecutedMetric,
	}

	return processor, nil
}

// OnBlockFinalized notifies the processor that a new finalized block is available for processing.
func (p *FinalizedBlockProcessor) OnBlockFinalized() {
	select {
	case p.blockFinalizedNotifier <- struct{}{}:
	default:
		// if the channel is full, no need to block, just return.
		// once the worker loop processes the buffered signal, it will
		// process the next height all the way to the highest finalized height.
	}
}

// StartWorkerLoop begins processing of finalized blocks and signals readiness when initialization is complete.
// It uses a single-threaded loop to process each finalized block height sequentially.
func (p *FinalizedBlockProcessor) StartWorkerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.blockFinalizedNotifier:
			finalizedHead, err := p.state.Final().Head()
			if err != nil {
				ctx.Throw(fmt.Errorf("failed to get finalized head: %w", err))
				return
			}

			highestFinalizedHeight := finalizedHead.Height
			lowestMissing := p.processedHeight.Value() + 1

			for height := lowestMissing; height <= highestFinalizedHeight; height++ {
				block, err := p.blocks.ByHeight(height)
				if err != nil {
					ctx.Throw(fmt.Errorf("failed to get block at height %d: %w", height, err))
					return
				}

				err = p.indexForFinalizedBlock(block)
				if err != nil {
					ctx.Throw(fmt.Errorf("failed to index finalized block at height %d: %w", height, err))
					return
				}

				// Update processed height after successful indexing
				err = p.processedHeight.Set(height)
				if err != nil {
					ctx.Throw(fmt.Errorf("failed to update processed height to %d: %w", height, err))
					return
				}
			}
		}
	}
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
