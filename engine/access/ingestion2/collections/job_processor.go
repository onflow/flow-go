package collections

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/engine/access/ingestion2"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/storage"
)

// EDIHeightProvider provides the latest height for which execution data indexer has collections.
type EDIHeightProvider interface {
	// HighestIndexedHeight returns the highest block height for which EDI has indexed collections.
	HighestIndexedHeight() (uint64, error)
}

// JobProcessor implements the job lifecycle for collection indexing.
// It orchestrates the flow: request → receive → index → complete.
type JobProcessor struct {
	mcq               ingestion2.MissingCollectionQueue
	indexer           ingestion2.BlockCollectionIndexer
	requester         ingestion2.CollectionRequester
	blocks            storage.Blocks
	collections       storage.CollectionsReader
	ediHeightProvider EDIHeightProvider // when set as nil, EDI is considered disabled
	ediLagThreshold   uint64
}

var _ ingestion2.JobProcessor = (*JobProcessor)(nil)

// NewJobProcessor creates a new JobProcessor.
//
// Parameters:
//   - mcq: MissingCollectionQueue for tracking missing collections and callbacks
//   - indexer: BlockCollectionIndexer for storing and indexing collections
//   - requester: CollectionRequester for requesting collections from the network
//   - blocks: Blocks storage for retrieving block data
//   - collections: Collections storage reader for checking if collections already exist
//   - ediHeightProvider: Provider for EDI's highest indexed height (can be nil if EDI is disabled)
//   - ediLagThreshold: Threshold in blocks. If (blockHeight - ediHeight) > threshold, fetch collections.
//     Set to a very large value to effectively disable fetching and rely only on EDI.
//
// No error returns are expected during normal operation.
func NewJobProcessor(
	mcq ingestion2.MissingCollectionQueue,
	indexer ingestion2.BlockCollectionIndexer,
	requester ingestion2.CollectionRequester,
	blocks storage.Blocks,
	collections storage.CollectionsReader,
	ediHeightProvider EDIHeightProvider,
	ediLagThreshold uint64,
) *JobProcessor {
	return &JobProcessor{
		mcq:               mcq,
		indexer:           indexer,
		requester:         requester,
		blocks:            blocks,
		collections:       collections,
		ediHeightProvider: ediHeightProvider,
		ediLagThreshold:   ediLagThreshold,
	}
}

// ProcessJob processes a job for a finalized block.
// It checks if the block is already indexed, and if not, enqueues missing collections
// and optionally requests them based on EDI lag.
//
// No error returns are expected during normal operation.
func (jp *JobProcessor) ProcessJob(
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
	callback := func() {
		// When all collections are received and indexed, mark the job as done
		done()
	}

	err = jp.mcq.EnqueueMissingCollections(blockHeight, collectionIDs, callback)
	if err != nil {
		return fmt.Errorf("failed to enqueue missing collections for block height %d: %w", blockHeight, err)
	}

	// Check EDI lag to decide whether to fetch collections
	shouldFetch, err := jp.shouldFetchCollections(blockHeight)
	if err != nil {
		return fmt.Errorf("failed to check EDI lag for block height %d: %w", blockHeight, err)
	}

	if shouldFetch {
		// Request collections from collection nodes
		err = jp.requester.RequestCollections(collectionIDs)
		if err != nil {
			return fmt.Errorf("failed to request collections for block height %d: %w", blockHeight, err)
		}
	}

	return nil
}

// OnReceivedCollectionsForBlock is called by the execution data indexer when collections are received.
// It forwards collections to MCQ and handles block completion.
//
// The blockHeight parameter is provided by EDI to indicate which block these collections belong to.
// Collections are forwarded individually to MCQ, which tracks completion per block.
//
// No error returns are expected during normal operation.
func (jp *JobProcessor) OnReceivedCollectionsForBlock(
	blockHeight uint64,
	collections []*flow.Collection,
) error {
	// Forward each collection to MCQ
	// MCQ will track which block each collection belongs to and detect when a block becomes complete
	for _, collection := range collections {
		receivedCols, height, complete := jp.mcq.OnReceivedCollection(collection)
		if complete {
			// Block became complete, index it
			err := jp.indexer.OnReceivedCollectionsForBlock(height, receivedCols)
			if err != nil {
				return fmt.Errorf("failed to index collections for block height %d: %w", height, err)
			}

			// Notify MCQ that the block has been indexed (this invokes the callback)
			jp.mcq.OnIndexedForBlock(height)
		}
	}

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

// shouldFetchCollections determines whether to fetch collections based on EDI lag.
// Returns true if collections should be fetched, false if we should wait for EDI.
func (jp *JobProcessor) shouldFetchCollections(blockHeight uint64) (bool, error) {
	// If EDI is not available, always fetch
	if jp.ediHeightProvider == nil {
		return true, nil
	}

	ediHeight, err := jp.ediHeightProvider.HighestIndexedHeight()
	if err != nil {
		// If we can't get EDI height, err on the side of fetching to avoid blocking
		return true, nil
	}

	// Calculate lag
	lag := blockHeight - ediHeight

	// If lag exceeds threshold, fetch collections
	return lag > jp.ediLagThreshold, nil
}
