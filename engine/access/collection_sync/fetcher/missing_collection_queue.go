package fetcher

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/model/flow"
)

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

var _ collection_sync.MissingCollectionQueue = (*MissingCollectionQueue)(nil)

// NewMissingCollectionQueue creates a new MissingCollectionQueue.
//
// No error returns are expected during normal operation.
func NewMissingCollectionQueue() *MissingCollectionQueue {
	return &MissingCollectionQueue{
		blockJobs:          make(map[uint64]*blockJobState),
		collectionToHeight: make(map[flow.Identifier]uint64),
	}
}

// pickOne returns an arbitrary key from the given map.
// The map must be non-empty. The selection is non-deterministic due to Go's map iteration order.
func pickOne(m map[flow.Identifier]struct{}) flow.Identifier {
	for id := range m {
		return id
	}
	return flow.ZeroID
}

// EnqueueMissingCollections registers missing collections for a block height along with a callback
// that will be invoked when all collections for that height have been received and indexed.
//
// The caller is responsible for checking if collections are already in storage before calling this method.
// Only collections that are actually missing should be passed in collectionIDs.
//
// Returns an error if the block height is already enqueued to prevent overwriting existing jobs.
func (mcq *MissingCollectionQueue) EnqueueMissingCollections(
	blockHeight uint64,
	collectionIDs []flow.Identifier,
	callback func(),
) error {
	mcq.mu.Lock()
	defer mcq.mu.Unlock()

	// Check if block height is already enqueued to prevent overwriting.
	if _, exists := mcq.blockJobs[blockHeight]; exists {
		return fmt.Errorf(
			"block height %d is already enqueued, cannot overwrite existing job",
			blockHeight,
		)
	}

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
				"fatal: collection %v is already assigned to block height %d, cannot assign to height %d",
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
//   - (collections, height, missingCollectionID, true) if the block height became complete
//   - (nil, 0, missingCollectionID, false) if no block height became complete
//     missingCollectionID is an arbitrary ID from the remaining missing collections, or ZeroID if none.
func (mcq *MissingCollectionQueue) OnReceivedCollection(
	collection *flow.Collection,
) ([]*flow.Collection, uint64, flow.Identifier, bool) {
	collectionID := collection.ID()

	mcq.mu.Lock()
	defer mcq.mu.Unlock()

	// Find the block height waiting for this collection (1:1 relationship).
	height, ok := mcq.collectionToHeight[collectionID]
	if !ok {
		// No block is waiting for this collection.
		return nil, 0, flow.ZeroID, false
	}

	jobState, exists := mcq.blockJobs[height]
	if !exists {
		// Job was already completed/removed.
		// Don't delete from collectionToHeight - cleanup happens in OnIndexedForBlock.
		return nil, 0, flow.ZeroID, false
	}

	// Check if this collection was still missing for this block.
	if _, wasMissing := jobState.missingCollections[collectionID]; !wasMissing {
		// Collection was already received or wasn't part of this block's missing set.
		// Don't delete from collectionToHeight - cleanup happens in OnIndexedForBlock.
		return nil, 0, pickOne(jobState.missingCollections), false
	}

	// Remove from missing set and add to received collections.
	delete(jobState.missingCollections, collectionID)
	// Store the collection so it can be returned when the block is complete.
	jobState.receivedCollections[collectionID] = collection

	// Don't delete from collectionToHeight - the mapping is kept until OnIndexedForBlock cleans it up.

	// Check if the block is now complete (all collections received).
	if len(jobState.missingCollections) > 0 {
		// pick a random missing collection to return
		// useful for logging/debugging purposes
		// in case fetching is stuck, it's useful to know which collections are still missing
		// we don't need to return all missing collections, just one is enough
		return nil, 0, pickOne(jobState.missingCollections), false
	}

	// Return all received collections for this block.
	collections := make([]*flow.Collection, 0, len(jobState.receivedCollections))
	for _, col := range jobState.receivedCollections {
		collections = append(collections, col)
	}
	return collections, height, flow.ZeroID, true
}

// IsHeightQueued returns true if the given height has queued collections
// Returns false if the height is not tracked
func (mcq *MissingCollectionQueue) IsHeightQueued(height uint64) bool {
	mcq.mu.RLock()
	defer mcq.mu.RUnlock()

	_, exists := mcq.blockJobs[height]
	return exists
}

// OnIndexedForBlock notifies the queue that a block height has been indexed,
// removes that block height from tracking, and return the callback for caller to
// invoke.
//
// Returns:
// (callback, true) if the height existed and was processed;
// (nil, false) if the height was not tracked.
//
// Note, caller should invoke the returned callback if not nil.
//
// Behavior: OnIndexedForBlock can return the callback even before the block is
// complete (i.e., before all collections have been received). This allows the caller
// to index a block with partial collections if needed. After indexing:
//   - The block is removed from tracking (IsHeightQueued returns false)
//   - All collection-to-height mappings for this block are cleaned up
//   - Any remaining missing collections are removed from tracking
//   - Subsequent OnReceivedCollection calls for collections belonging to this block
//     will return (nil, 0, false) because the block has been removed
func (mcq *MissingCollectionQueue) OnIndexedForBlock(blockHeight uint64) (func(), bool) {
	mcq.mu.Lock()
	defer mcq.mu.Unlock()

	// Clean up all collection-to-height mappings and remove the block job.
	// This ensures the height is removed from tracking once the callback is invoked.
	jobState, exists := mcq.cleanupCollectionMappingsForHeight(blockHeight)
	if !exists {
		// Block was not tracked or already completed
		return nil, false
	}

	// Get the callback from the job state.
	return jobState.callback, true
}

// cleanupCollectionMappingsForHeight removes all collection-to-height mappings for collections
// belonging to the specified block height and removes the block job from tracking.
// This includes both missing and received collections.
//
// Returns the job state and a bool indicating if the block height existed.
//
// This method must be called while holding the write lock (mcq.mu.Lock()).
func (mcq *MissingCollectionQueue) cleanupCollectionMappingsForHeight(
	blockHeight uint64,
) (*blockJobState, bool) {
	jobState, exists := mcq.blockJobs[blockHeight]
	if !exists {
		return nil, false
	}

	// Clean up from missing collections.
	for collectionID := range jobState.missingCollections {
		delete(mcq.collectionToHeight, collectionID)
	}
	// Clean up from received collections.
	for collectionID := range jobState.receivedCollections {
		delete(mcq.collectionToHeight, collectionID)
	}

	delete(mcq.blockJobs, blockHeight)

	return jobState, true
}

// Size returns the number of incomplete jobs currently in the queue.
func (mcq *MissingCollectionQueue) Size() uint {
	mcq.mu.RLock()
	defer mcq.mu.RUnlock()

	return uint(len(mcq.blockJobs))
}

// GetMissingCollections returns all collection IDs that are currently missing across all block heights.
//
// Returns a slice of collection identifiers that are still missing. The order is non-deterministic
// due to map iteration order. Returns an empty slice if there are no missing collections.
func (mcq *MissingCollectionQueue) GetMissingCollections() []flow.Identifier {
	mcq.mu.RLock()
	defer mcq.mu.RUnlock()

	// Count total missing collections to pre-allocate slice
	totalMissing := 0
	for _, jobState := range mcq.blockJobs {
		totalMissing += len(jobState.missingCollections)
	}

	// Pre-allocate slice with capacity
	missingCollections := make([]flow.Identifier, 0, totalMissing)

	// Collect all missing collection IDs from all block jobs
	for _, jobState := range mcq.blockJobs {
		for collectionID := range jobState.missingCollections {
			missingCollections = append(missingCollections, collectionID)
		}
	}

	return missingCollections
}

// GetMissingCollectionsByHeight returns a map of block height to collection IDs that are missing for that height.
//
// Returns a map where keys are block heights and values are slices of collection identifiers
// that are still missing for that height. Returns an empty map if there are no missing collections.
func (mcq *MissingCollectionQueue) GetMissingCollectionsByHeight() map[uint64][]flow.Identifier {
	mcq.mu.RLock()
	defer mcq.mu.RUnlock()

	// Build map of height -> collection IDs
	missingByHeight := make(map[uint64][]flow.Identifier)

	for height, jobState := range mcq.blockJobs {
		if len(jobState.missingCollections) == 0 {
			continue
		}

		collectionIDs := make([]flow.Identifier, 0, len(jobState.missingCollections))
		for collectionID := range jobState.missingCollections {
			collectionIDs = append(collectionIDs, collectionID)
		}
		missingByHeight[height] = collectionIDs
	}

	return missingByHeight
}
