package fetcher

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// BlockProcessor implements the job lifecycle for collection indexing.
// It orchestrates the flow: request → receive → index → complete.
type BlockProcessor struct {
	log       zerolog.Logger
	mcq       collection_sync.MissingCollectionQueue
	indexer   collection_sync.BlockCollectionIndexer
	requester collection_sync.CollectionRequester
}

var _ collection_sync.BlockProcessor = (*BlockProcessor)(nil)

// NewBlockProcessor creates a new BlockProcessor.
//
// Parameters:
//   - log: Logger for the component
//   - mcq: MissingCollectionQueue for tracking missing collections and callbacks
//   - indexer: BlockCollectionIndexer for storing and indexing collections
//   - requester: CollectionRequester for requesting collections from the network
//
// No error returns are expected during normal operation.
func NewBlockProcessor(
	log zerolog.Logger,
	mcq collection_sync.MissingCollectionQueue,
	indexer collection_sync.BlockCollectionIndexer,
	requester collection_sync.CollectionRequester,
) *BlockProcessor {
	return &BlockProcessor{
		log:       log.With().Str("component", "coll_fetcher").Logger(),
		mcq:       mcq,
		indexer:   indexer,
		requester: requester,
	}
}

// FetchCollections processes a block for collection fetching.
// It checks if the block is already indexed, and if not, enqueues missing collections
// and optionally requests them based on EDI lag.
//
// No error returns are expected during normal operation.
func (bp *BlockProcessor) FetchCollections(
	ctx irrecoverable.SignalerContext,
	block *flow.Block,
	done func(),
) error {
	blockHeight := block.Height
	bp.log.Debug().Uint64("block_height", blockHeight).
		Msg("processing collection fetching job for finalized block")

	// Get missing collections for this block
	missingGuarantees, err := bp.indexer.GetMissingCollections(block)
	if err != nil {
		return fmt.Errorf("failed to get missing collections for block height %d: %w", blockHeight, err)
	}

	// If there are no missing collections, this block is considered complete.
	// Caution: This relies on the assumption that:
	// whenever a collection exists in storage, all of its transactions must have already been indexed.
	// This assumption currently holds because transaction indexing by collection is always performed
	// in the same batch that stores the collection (via collections.BatchStoreAndIndexByTransaction).
	// Note: when we receives a collection, we need it in memory, and don't index until all collections
	// of the block are received.
	if len(missingGuarantees) == 0 {
		done()
		return nil
	}

	// Extract collection IDs
	collectionIDs := make([]flow.Identifier, len(missingGuarantees))
	for i, guarantee := range missingGuarantees {
		collectionIDs[i] = guarantee.CollectionID
	}

	// Enqueue missing collections with notifyJobCompletion
	// When all collections are received and indexed, mark the job as done
	notifyJobCompletion := done

	// the notifyJobCompletion callback will be returned in the OnReceiveCollection method
	err = bp.mcq.EnqueueMissingCollections(blockHeight, collectionIDs, notifyJobCompletion)
	if err != nil {
		return fmt.Errorf("failed to enqueue missing collections for block height %d: %w", blockHeight, err)
	}

	// Request collections from collection nodes
	err = bp.requester.RequestCollections(collectionIDs)
	if err != nil {
		return fmt.Errorf("failed to request collections for block height %d: %w", blockHeight, err)
	}

	return nil
}

// OnReceiveCollection is called when a collection is received from the requester.
// It passes the collection to MCQ, and if it completes a block, indexes it and marks it as done.
//
// No error returns are expected during normal operation.
func (bp *BlockProcessor) OnReceiveCollection(originID flow.Identifier, collection *flow.Collection) error {
	collectionID := collection.ID()

	// Pass collection to MCQ
	collections, height, complete := bp.mcq.OnReceivedCollection(collection)

	// Log collection receipt and whether it completes a block
	if complete {
		bp.log.Info().
			Hex("collection_id", collectionID[:]).
			Hex("origin_id", originID[:]).
			Uint64("block_height", height).
			Int("collections_count", len(collections)).
			Msg("received collection completing block to be indexed")
	} else {
		bp.log.Debug().
			Hex("collection_id", collectionID[:]).
			Hex("origin_id", originID[:]).
			Msg("received collection (block not yet complete)")
	}

	if !complete {
		// Block is not complete yet, nothing more to do
		return nil
	}

	// Block became complete, index it
	err := bp.indexer.IndexCollectionsForBlock(height, collections)
	if err != nil {
		return fmt.Errorf("failed to index collections for block height %d: %w", height, err)
	}

	// Mark the block as indexed
	notifyJobCompletion, ok := bp.mcq.OnIndexedForBlock(height)
	if ok {
		notifyJobCompletion()
	}

	return nil
}

// MissingCollectionQueueSize returns the number of missing collections currently in the queue.
func (bp *BlockProcessor) MissingCollectionQueueSize() uint {
	return bp.mcq.Size()
}

// PruneUpToHeight removes all tracked heights up to and including the given height.
func (bp *BlockProcessor) PruneUpToHeight(height uint64) {
	callbacks := bp.mcq.PruneUpToHeight(height)
	// notify job completion
	for _, cb := range callbacks {
		cb()
	}
}
