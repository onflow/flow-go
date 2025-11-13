package fetcher

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/storage"
)

// JobProcessor implements the job lifecycle for collection indexing.
// It orchestrates the flow: request → receive → index → complete.
// TODO: rename to fetch_job_processor
type JobProcessor struct {
	log         zerolog.Logger
	mcq         collection_sync.MissingCollectionQueue
	indexer     collection_sync.BlockCollectionIndexer
	requester   collection_sync.CollectionRequester
	blocks      storage.Blocks
	collections storage.CollectionsReader
}

var _ collection_sync.JobProcessor = (*JobProcessor)(nil)

// NewJobProcessor creates a new JobProcessor.
//
// Parameters:
//   - log: Logger for the component
//   - mcq: MissingCollectionQueue for tracking missing collections and callbacks
//   - indexer: BlockCollectionIndexer for storing and indexing collections
//   - requester: CollectionRequester for requesting collections from the network
//   - blocks: Blocks storage for retrieving block data
//   - collections: Collections storage reader for checking if collections already exist
//     Set to a very large value to effectively disable fetching and rely only on EDI.
//
// No error returns are expected during normal operation.
func NewJobProcessor(
	log zerolog.Logger,
	mcq collection_sync.MissingCollectionQueue,
	indexer collection_sync.BlockCollectionIndexer,
	requester collection_sync.CollectionRequester,
	blocks storage.Blocks,
	collections storage.CollectionsReader,
) *JobProcessor {
	return &JobProcessor{
		log:         log.With().Str("component", "coll_fetcher").Logger(),
		mcq:         mcq,
		indexer:     indexer,
		requester:   requester,
		blocks:      blocks,
		collections: collections,
	}
}

// ProcessJob processes a job for a finalized block.
// It checks if the block is already indexed, and if not, enqueues missing collections
// and optionally requests them based on EDI lag.
//
// No error returns are expected during normal operation.
func (jp *JobProcessor) ProcessJobConcurrently(
	ctx irrecoverable.SignalerContext,
	job module.Job,
	done func(),
) error {
	// Convert job to block
	block, err := jobqueue.JobToBlock(job)
	if err != nil {
		return fmt.Errorf("could not convert job to block: %w", err)
	}

	blockHeight := block.Height
	jp.log.Debug().Uint64("block_height", blockHeight).
		Msg("processing collection fetching job for finalized block")

	// Get missing collections for this block
	missingGuarantees, err := jp.getMissingCollections(blockHeight)
	if err != nil {
		return fmt.Errorf("failed to get missing collections for block height %d: %w", blockHeight, err)
	}

	// If no missing collections, block is complete
	if len(missingGuarantees) == 0 {
		done()
		return nil
	}

	// Extract collection IDs
	collectionIDs := make([]flow.Identifier, len(missingGuarantees))
	for i, guarantee := range missingGuarantees {
		collectionIDs[i] = guarantee.CollectionID
	}

	// Enqueue missing collections with callback
	// When all collections are received and indexed, mark the job as done
	callback := done

	err = jp.mcq.EnqueueMissingCollections(blockHeight, collectionIDs, callback)
	if err != nil {
		return fmt.Errorf("failed to enqueue missing collections for block height %d: %w", blockHeight, err)
	}

	// Request collections from collection nodes
	err = jp.requester.RequestCollections(collectionIDs)
	if err != nil {
		return fmt.Errorf("failed to request collections for block height %d: %w", blockHeight, err)
	}

	return nil
}

// OnReceiveCollection is called when a collection is received from the requester.
// It passes the collection to MCQ, and if it completes a block, indexes it and marks it as done.
//
// No error returns are expected during normal operation.
func (jp *JobProcessor) OnReceiveCollection(originID flow.Identifier, collection *flow.Collection) error {
	collectionID := collection.ID()

	// Pass collection to MCQ
	collections, height, complete := jp.mcq.OnReceivedCollection(collection)

	// Log collection receipt and whether it completes a block
	if complete {
		jp.log.Info().
			Hex("collection_id", collectionID[:]).
			Hex("origin_id", originID[:]).
			Uint64("block_height", height).
			Int("collections_count", len(collections)).
			Msg("received collection completing block to be indexed")
	} else {
		jp.log.Debug().
			Hex("collection_id", collectionID[:]).
			Hex("origin_id", originID[:]).
			Msg("received collection (block not yet complete)")
	}

	if !complete {
		// Block is not complete yet, nothing more to do
		return nil
	}

	// Block became complete, index it
	err := jp.indexer.IndexCollectionsForBlock(height, collections)
	if err != nil {
		return fmt.Errorf("failed to index collections for block height %d: %w", height, err)
	}

	// Mark the block as indexed (this invokes the callback)
	jp.mcq.OnIndexedForBlock(height)

	return nil
}

// getMissingCollections retrieves the block and returns collection guarantees that are missing.
// Only collections that are not already in storage are returned.
func (jp *JobProcessor) getMissingCollections(blockHeight uint64) ([]*flow.CollectionGuarantee, error) {
	block, err := jp.blocks.ByHeight(blockHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve block at height %d: %w", blockHeight, err)
	}

	var missingGuarantees []*flow.CollectionGuarantee
	for _, guarantee := range block.Payload.Guarantees {
		// Check if collection already exists in storage
		_, err := jp.collections.LightByID(guarantee.CollectionID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				// Collection is missing
				missingGuarantees = append(missingGuarantees, guarantee)
			} else {
				// Unexpected error
				return nil, fmt.Errorf("failed to check if collection %v exists: %w", guarantee.CollectionID, err)
			}
		}
		// If collection exists, skip it
	}

	return missingGuarantees, nil
}
