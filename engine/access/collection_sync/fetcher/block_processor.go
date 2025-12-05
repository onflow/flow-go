package fetcher

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

// BlockProcessor implements the job lifecycle for collection indexing.
// It orchestrates the flow: request → receive → index → complete.
type BlockProcessor struct {
	log       zerolog.Logger
	mcq       collection_sync.MissingCollectionQueue
	indexer   collection_sync.BlockCollectionIndexer
	requester collection_sync.CollectionRequester
	blocks    storage.Blocks
}

var _ collection_sync.BlockProcessor = (*BlockProcessor)(nil)

// NewBlockProcessor creates a new BlockProcessor.
//
// Parameters:
//   - log: Logger for the component
//   - mcq: MissingCollectionQueue for tracking missing collections and callbacks
//   - indexer: BlockCollectionIndexer for storing and indexing collections
//   - requester: CollectionRequester for requesting collections from the network
//   - blocks: Blocks storage for retrieving blocks to extract guarantees
//
// No error returns are expected during normal operation.
func NewBlockProcessor(
	log zerolog.Logger,
	mcq collection_sync.MissingCollectionQueue,
	indexer collection_sync.BlockCollectionIndexer,
	requester collection_sync.CollectionRequester,
	blocks storage.Blocks,
) *BlockProcessor {
	return &BlockProcessor{
		log:       log.With().Str("coll_sync", "fetcher_processor").Logger(),
		mcq:       mcq,
		indexer:   indexer,
		requester: requester,
		blocks:    blocks,
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

	// Extract collection IDs for enqueueing
	collectionIDs := make([]flow.Identifier, len(missingGuarantees))
	for i, guarantee := range missingGuarantees {
		collectionIDs[i] = guarantee.CollectionID
	}

	// Enqueue missing collections with notifyJobCompletion
	// When all collections are received and indexed, the notifyJobCompletion callback
	// will be returned in the OnIndexedForBlock method, calling it will notify the job
	// queue that the job is complete.
	notifyJobCompletion := done

	err = bp.mcq.EnqueueMissingCollections(blockHeight, collectionIDs, notifyJobCompletion)
	if err != nil {
		return fmt.Errorf("failed to enqueue missing collections for block height %d: %w", blockHeight, err)
	}

	// Request collections from collection nodes
	// Note: requester does not guarantee it will keep trying to fetch the collections,
	// in fact, if a collection node is down, the request may fail permanently,
	// so we need to have a retry mechanism in place (see RetryFetchingMissingCollections),
	// which allows requester to retry with a different collection node.
	err = bp.requester.RequestCollectionsByGuarantees(missingGuarantees)
	if err != nil {
		return fmt.Errorf("failed to request collections for block height %d: %w", blockHeight, err)
	}

	bp.log.Debug().Uint64("block_height", blockHeight).
		Int("missing_collections_count", len(missingGuarantees)).
		Msg("fetched collections for finalized block")

	return nil
}

// OnReceiveCollection is called when a collection is received from the requester.
// It passes the collection to MCQ, and if it completes a block, indexes it and marks it as done.
//
// No error returns are expected during normal operation.
func (bp *BlockProcessor) OnReceiveCollection(originID flow.Identifier, collection *flow.Collection) error {
	collectionID := collection.ID()

	// Pass collection to MCQ
	collections, height, missingCol, complete := bp.mcq.OnReceivedCollection(collection)

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
			// there might be multiple missing collections, just log one of them
			Hex("missing_col", missingCol[:]).
			Uint64("block_height", height).
			Msg("received collection (block not yet complete)")
	}

	// OnReceiveCollection might be called multiple times for the same collection
	// but complete will only be true once per block height.
	if !complete {
		// Block is not complete yet, nothing more to do
		// or block is complete, but another thread is indexing it
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

// RetryFetchingMissingCollections retries fetching all missing collections currently in the queue.
// It retrieves blocks for the heights with missing collections and extracts the corresponding guarantees.
// Note, the caller is responsible to ensure this method is called periodically, and not concurrently.
//
// No error returns are expected during normal operation.
func (bp *BlockProcessor) RetryFetchingMissingCollections() error {
	// Get missing collections grouped by height
	missingByHeight := bp.mcq.GetMissingCollectionsByHeight()
	if len(missingByHeight) == 0 {
		return nil
	}

	// Collect guarantees for missing collections by retrieving blocks
	// Double-check that collections are actually missing in storage (they may have been
	// indexed by execution data indexer or received from another source)
	var guarantees []*flow.CollectionGuarantee

	for height := range missingByHeight {
		// Get block for this height
		block, err := bp.blocks.ByHeight(height)
		if err != nil {
			bp.log.Fatal().
				Uint64("block_height", height).
				Err(err).
				Msg("failed to retrieve block for retrying missing collections, skipping")
			continue
		}

		// Use indexer's GetMissingCollections to filter out collections that already exist
		// This handles the case where execution data indexer may have already indexed them
		actuallyMissingGuarantees, err := bp.indexer.GetMissingCollections(block)
		if err != nil {
			bp.log.Warn().
				Uint64("block_height", height).
				Err(err).
				Msg("failed to check missing collections for block, skipping")
			continue
		}

		// If no collections are missing, all collections for this block are now indexed
		if len(actuallyMissingGuarantees) == 0 {
			// All collections for this block are now indexed, mark it as done
			notifyJobCompletion, ok := bp.mcq.OnIndexedForBlock(height)
			if ok {
				notifyJobCompletion()
				bp.log.Info().
					Uint64("block_height", height).
					Msg("all collections for block are now indexed, marked block as done")
			}
			continue
		}

		// Add guarantees for collections that are still missing
		guarantees = append(guarantees, actuallyMissingGuarantees...)
	}

	if len(guarantees) == 0 {
		bp.log.Debug().Msg("no guarantees found for missing collections")
		return nil
	}

	// Request collections using the guarantees
	err := bp.requester.RequestCollectionsByGuarantees(guarantees)
	if err != nil {
		return fmt.Errorf("failed to retry requesting collections: %w", err)
	}

	bp.log.Info().
		Int("guarantees_count", len(guarantees)).
		Int("heights_count", len(missingByHeight)).
		Msg("retried fetching missing collections")

	return nil
}

// MissingCollectionQueueSize returns the number of missing collections currently in the queue.
func (bp *BlockProcessor) MissingCollectionQueueSize() uint {
	return bp.mcq.Size()
}
