package collections

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine/access/ingestion2"
	"github.com/onflow/flow-go/model/flow"
)

// blockJobState tracks the state of a job for a specific block height.
type blockJobState struct {
	// missingCollections is a set of collection IDs that are still missing for this block height.
	missingCollections map[flow.Identifier]struct{}
	// receivedCollections stores the collections that have been received so far, keyed by collection ID.
	// This allows us to return all collections when the block becomes complete.
	receivedCollections map[flow.Identifier]*flow.Collection
	// callback is invoked when all collections for this block height have been received and indexed.
	callback func()
}

// MissingCollectionQueue helps the job processor to keep track of the jobs and their callbacks.
// Note, it DOES NOT index collections directly, instead, it only keeps track of which collections are missing
// for each block height, and when all collections for a block height have been received, it returns the
// collections to the caller for processing (storing and indexing). And let the caller to notify the completion
// of the processing, so that it can mark the job as done by calling the callback.
// This allows the MissingCollectionQueue to be decoupled from the actual processing of the collections, keep
// all states in memory and allow the different callers to hold the lock less time and reduce contention.
//
// The caller is responsible for checking if collections are already in storage before enqueueing them.
// Only collections that are actually missing should be passed to EnqueueMissingCollections.
//
// MissingCollectionQueue is safe for concurrent use.
type MissingCollectionQueue struct {
	// mu protects all the maps below.
	mu sync.RWMutex

	// blockJobs maps block height to the job state for that height.
	blockJobs map[uint64]*blockJobState

	// collectionToHeight maps collection ID to the single block height waiting for that collection.
	// This enforces a 1:1 relationship: each collection belongs to exactly one block.
	// This allows efficient lookup when a collection is received.
	collectionToHeight map[flow.Identifier]uint64
}

var _ ingestion2.MissingCollectionQueue = (*MissingCollectionQueue)(nil)

// NewMissingCollectionQueue creates a new MissingCollectionQueue.
//
// No error returns are expected during normal operation.
func NewMissingCollectionQueue() *MissingCollectionQueue {
	return &MissingCollectionQueue{
		blockJobs:          make(map[uint64]*blockJobState),
		collectionToHeight: make(map[flow.Identifier]uint64),
	}
}

// EnqueueMissingCollections registers missing collections for a block height along with a callback
// that will be invoked when all collections for that height have been received and indexed.
//
// The caller is responsible for checking if collections are already in storage before calling this method.
// Only collections that are actually missing should be passed in collectionIDs.
//
// If the same block height is enqueued multiple times, the previous callback is replaced.
//
// No error returns are expected during normal operation.
func (mcq *MissingCollectionQueue) EnqueueMissingCollections(
	blockHeight uint64,
	collectionIDs []flow.Identifier,
	callback func(),
) error {
	mcq.mu.Lock()
	defer mcq.mu.Unlock()

	// Create the job state with all collections marked as missing.
	// The caller has already verified these collections are not in storage.
	missingSet := make(map[flow.Identifier]struct{}, len(collectionIDs))
	for _, id := range collectionIDs {
		missingSet[id] = struct{}{}
	}

	jobState := &blockJobState{
		missingCollections:  missingSet,
		receivedCollections: make(map[flow.Identifier]*flow.Collection),
		callback:            callback,
	}

	mcq.blockJobs[blockHeight] = jobState

	// Update the collection-to-height mapping, enforcing 1:1 relationship.
	for _, collectionID := range collectionIDs {
		existingHeight, exists := mcq.collectionToHeight[collectionID]
		if exists && existingHeight != blockHeight {
			// Collection is already assigned to a different block - this violates the 1:1 constraint.
			return fmt.Errorf(
				"collection %v is already assigned to block height %d, cannot assign to height %d",
				collectionID, existingHeight, blockHeight,
			)
		}
		mcq.collectionToHeight[collectionID] = blockHeight
	}

	return nil
}

// OnReceivedCollection notifies the queue that a collection has been received.
// It checks if the block height is now complete and returns the collections and height.
//
// The collection parameter should be the actual collection object received from the requester.
//
// Returns:
//   - (collections, height, true) if the block height became complete
//   - (nil, 0, false) if no block height became complete
//
// No error returns are expected during normal operation.
func (mcq *MissingCollectionQueue) OnReceivedCollection(
	collection *flow.Collection,
) ([]*flow.Collection, uint64, bool) {
	collectionID := collection.ID()

	mcq.mu.Lock()
	defer mcq.mu.Unlock()

	// Find the block height waiting for this collection (1:1 relationship).
	height, ok := mcq.collectionToHeight[collectionID]
	if !ok {
		// No block is waiting for this collection.
		return nil, 0, false
	}

	jobState, exists := mcq.blockJobs[height]
	if !exists {
		// Job was already completed/removed, clean up the mapping.
		delete(mcq.collectionToHeight, collectionID)
		return nil, 0, false
	}

	// Check if this collection was still missing for this block.
	if _, wasMissing := jobState.missingCollections[collectionID]; !wasMissing {
		// Collection was already received or wasn't part of this block's missing set.
		// Clean up the mapping since we've already processed this collection.
		delete(mcq.collectionToHeight, collectionID)
		return nil, 0, false
	}

	// Remove from missing set and add to received collections.
	delete(jobState.missingCollections, collectionID)
	// Store the collection so it can be returned when the block is complete.
	jobState.receivedCollections[collectionID] = collection

	// Remove this collection from the collection-to-height mapping since we've processed it.
	delete(mcq.collectionToHeight, collectionID)

	// Check if the block is now complete (all collections received).
	if len(jobState.missingCollections) == 0 {
		// Return all received collections for this block.
		collections := make([]*flow.Collection, 0, len(jobState.receivedCollections))
		for _, col := range jobState.receivedCollections {
			collections = append(collections, col)
		}
		return collections, height, true
	}

	return nil, 0, false
}

// OnIndexedForBlock notifies the queue that a block height has been indexed.
// This invokes the callback for that block height and removes it from tracking.
//
// No error returns are expected during normal operation.
func (mcq *MissingCollectionQueue) OnIndexedForBlock(blockHeight uint64) {
	mcq.mu.Lock()

	jobState, exists := mcq.blockJobs[blockHeight]
	if !exists {
		// Block was not tracked or already completed (callback already called and job removed).
		mcq.mu.Unlock()
		return
	}

	// Get the callback before removing the job.
	callback := jobState.callback

	// Clean up all collection-to-height mappings for collections belonging to this block.
	// Clean up from missing collections.
	for collectionID := range jobState.missingCollections {
		if height, ok := mcq.collectionToHeight[collectionID]; ok && height == blockHeight {
			delete(mcq.collectionToHeight, collectionID)
		}
	}
	// Clean up from received collections.
	for collectionID := range jobState.receivedCollections {
		if height, ok := mcq.collectionToHeight[collectionID]; ok && height == blockHeight {
			delete(mcq.collectionToHeight, collectionID)
		}
	}

	// Remove the job state from the queue before calling the callback.
	// This ensures the height is removed from tracking once the callback is invoked.
	delete(mcq.blockJobs, blockHeight)

	// Release the lock before calling the callback to avoid holding it during callback execution.
	mcq.mu.Unlock()
	callback()
	// Note: We manually unlocked above, so we don't use defer here to avoid double-unlock.
}
