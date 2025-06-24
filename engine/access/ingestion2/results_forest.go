package ingestion2

import (
	"fmt"
	"sort"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/forest"
	"github.com/onflow/flow-go/storage"
)

var (
	// ErrMaxSizeExceeded is returned when adding a new container would exceed the maximum size
	ErrMaxSizeExceeded = fmt.Errorf("adding new result would exceed maximum size")
)

// ResultsForest is a mempool holding execution results and receipts, which is aware of the tree
// structure formed by the results. The mempool supports pruning by view (of the executed block):
// only results descending from the latest sealed and finalized result are relevant.
// By convention, the ResultsForest always contains the latest sealed result. Thereby, the
// ResultsForest is able to determine whether results for a block still need to be processed or
// can be orphaned (processing abandoned). Hence, we prune all results for blocks _below_ the
// latest block with a finalized seal.
// All results for views at or above the pruning threshold are retained, explicitly including results
// from execution forks or orphaned blocks even if they conflict with the finalized seal. However, such
// orphaned forks will eventually stop growing, because either (i) a conflicting fork of blocks is
// finalized, which means that the orphaned forks can no longer be extended by new blocks or (ii) a
// rogue execution node pursuing its own execution fork will eventually be slashed and can no longer
// submit new results.
// Nevertheless, to utilize resources efficiently, the ResultsForest tried to avoid processing execution
// forks that conflict with the finalized seal.
//
// Safe for concurrent access. Internally, the mempool utilizes the LevelledForrest.
type ResultsForest struct {
	log                         zerolog.Logger
	forest                      forest.LevelledForest
	maxSize                     uint64
	lastSealedResultID          flow.Identifier
	latestPersistedSealedResult storage.LatestPersistedSealedResult
	mu                          sync.RWMutex
}

// NewResultsForest creates a new instance of ResultsForest.
//
// Parameters:
//   - log: logger instance
//   - maxSize: the maximum number of containers allowed in the forest
//   - latestPersistedSealedResult: the latest persisted sealed result
//
// Returns:
//   - *ResultsForest: the newly created forest
//
// Concurrency safety:
//   - Safe for concurrent access
func NewResultsForest(
	log zerolog.Logger,
	maxSize uint64,
	latestPersistedSealedResult storage.LatestPersistedSealedResult,
) *ResultsForest {
	resultID, _ := latestPersistedSealedResult.Latest()
	return &ResultsForest{
		log:                         log.With().Str("component", "results_forest").Logger(),
		forest:                      *forest.NewLevelledForest(0),
		maxSize:                     maxSize,
		lastSealedResultID:          resultID,
		latestPersistedSealedResult: latestPersistedSealedResult,
		mu:                          sync.RWMutex{},
	}
}

// AddResult adds an Execution Result to the Result Forest (without any receipts), in
// case the result is not already stored in the tree.
// This is useful for crash recovery:
// After recovering from a crash, the mempools are wiped and the sealed results will not
// be stored in the Execution Tree anymore. Adding the result to the tree allows to create
// a vertex in the tree without attaching any Execution Receipts to it.
//
// Parameters:
//   - result: the execution result to add
//   - header: the block header associated with the result
//   - pipeline: the pipeline to process the result
//
// Returns:
//   - error: any error that occurred during the operation
//
// Expected Errors:
//   - ErrMaxSizeExceeded: when adding a new container would exceed the maximum size
//   - All other errors are unexpected and potential indicators of corrupted internal state
//
// Concurrency safety:
//   - Safe for concurrent access
//
// TODO: during normal operations we should never add a result without a receipt, this is only used for crash recovery
func (rf *ResultsForest) AddResult(result *flow.ExecutionResult, header *flow.Header, pipeline optimistic_sync.Pipeline) error {
	// front-load sanity checks before locking: result must be for block
	if header.ID() != result.BlockID {
		return fmt.Errorf("receipt is for different block")
	}

	rf.mu.Lock()
	defer rf.mu.Unlock()

	// drop receipts for block views lower than the lowest view.
	if header.View < rf.forest.LowestLevel {
		return nil
	}

	_, err := rf.getOrCreateExecutionResultContainer(result, header, pipeline)
	if err != nil {
		return fmt.Errorf("failed to get container for result (%s): %w", result.ID(), err)
	}
	return nil
}

// AddReceipt adds the given execution receipt to the forest.
//
// Parameters:
//   - receipt: the execution receipt to add
//   - header: the block header associated with the receipt
//   - pipeline: the pipeline to process the receipt
//
// Returns:
//   - bool: true if the receipt was added, false if it already existed
//   - error: any error that occurred during the operation
//
// Expected Errors:
//   - ErrMaxSizeExceeded: when adding a new container would exceed the maximum size
//   - All other errors are unexpected and potential indicators of corrupted internal state
//
// Concurrency safety:
//   - Safe for concurrent access
func (rf *ResultsForest) AddReceipt(receipt *flow.ExecutionReceipt, header *flow.Header, pipeline optimistic_sync.Pipeline) (bool, error) {
	// front-load sanity checks before locking: result must be for block
	if header.ID() != receipt.ExecutionResult.BlockID {
		return false, fmt.Errorf("receipt is for different block")
	}

	rf.mu.Lock()
	defer rf.mu.Unlock()

	// drop receipts for block views lower than the lowest view.
	if header.View < rf.forest.LowestLevel {
		return false, nil
	}

	container, err := rf.getOrCreateExecutionResultContainer(&receipt.ExecutionResult, header, pipeline)
	if err != nil {
		return false, fmt.Errorf("failed to get container for result (%s): %w", receipt.ExecutionResult.ID(), err)
	}

	added, err := container.AddReceipt(receipt)
	if err != nil {
		return false, fmt.Errorf("failed to add receipt to its container: %w", err)
	}

	return added > 0, nil
}

// getOrCreateExecutionResultContainer retrieves or creates the container for the given result.
//
// Parameters:
//   - result: the execution result
//   - block: the block header associated with the result
//   - pipeline: the pipeline to process the result
//
// Returns:
//   - *ExecutionResultContainer: the container for the result
//   - error: any error that occurred during the operation
//
// Expected Errors:
//   - ErrMaxSizeExceeded: when adding a new container would exceed the maximum size
//   - All other errors are unexpected and potential indicators of corrupted internal state
//
// Concurrency safety:
//   - Not safe for concurrent access, caller must hold lock
func (rf *ResultsForest) getOrCreateExecutionResultContainer(result *flow.ExecutionResult, block *flow.Header, pipeline optimistic_sync.Pipeline) (*ExecutionResultContainer, error) {
	// First try to get existing container
	container, found := rf.getContainer(result.ID())
	if found {
		return container, nil
	}

	// Check if adding new container would exceed max size
	if rf.forest.GetSize() >= rf.maxSize {
		return nil, ErrMaxSizeExceeded
	}

	container, err := NewExecutionResultContainer(result, block, pipeline)
	if err != nil {
		return nil, fmt.Errorf("constructing container for receipt failed: %w", err)
	}

	// Verify and add to forest
	err = rf.forest.VerifyVertex(container)
	if err != nil {
		return nil, fmt.Errorf("failed to store receipt's container: %w", err)
	}
	rf.forest.AddVertex(container)

	return container, nil
}

// HasReceipt checks if a receipt exists in the forest.
//
// Parameters:
//   - receipt: the execution receipt to check
//
// Returns:
//   - bool: true if the receipt exists, false otherwise
//
// Concurrency safety:
//   - Safe for concurrent access
func (rf *ResultsForest) HasReceipt(receipt *flow.ExecutionReceipt) bool {
	resultID := receipt.ExecutionResult.ID()
	receiptID := receipt.ID()

	rf.mu.RLock()
	defer rf.mu.RUnlock()

	container, found := rf.getContainer(resultID)
	if !found {
		return false
	}
	return container.Has(receiptID)
}

// pruneUpToView prunes all results for all blocks with view up to but not including the given limit.
//
// Parameters:
//   - limit: the view up to which to prune (exclusive)
//
// Returns:
//   - error: any error that occurred during the operation
//
// No errors are expected during normal operation.
//
// Concurrency safety:
//   - Not safe for concurrent access, caller must hold lock
func (rf *ResultsForest) pruneUpToView(level uint64) error {
	err := rf.forest.PruneUpToLevel(level)
	if err != nil {
		return fmt.Errorf("pruning Levelled Forest up to view (aka level) %d failed: %w", level, err)
	}

	return nil
}

// Size returns the number of receipts stored in the forest.
//
// Returns:
//   - uint: the number of receipts stored
//
// Concurrency safety:
//   - Safe for concurrent access
func (rf *ResultsForest) Size() uint {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return uint(rf.forest.GetSize())
}

// LowestView returns the lowest view where results are still stored in the mempool.
//
// Returns:
//   - uint64: the lowest view
//
// Concurrency safety:
//   - Safe for concurrent access
func (rf *ResultsForest) LowestView() uint64 {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.forest.LowestLevel
}

// GetContainer retrieves the ExecutionResultContainer for the given result ID.
//
// Parameters:
//   - resultID: the ID of the result to retrieve
//
// Returns:
//   - *ExecutionResultContainer: the container for the result, or nil if not found
//   - bool: true if the container was found, false otherwise
//
// Concurrency safety:
//   - Safe for concurrent access
func (rf *ResultsForest) GetContainer(resultID flow.Identifier) (*ExecutionResultContainer, bool) {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.getContainer(resultID)
}

// getContainer retrieves the ExecutionResultContainer for the given result ID.
//
// Parameters:
//   - resultID: the ID of the result to retrieve
//
// Returns:
//   - *ExecutionResultContainer: the container for the result, or nil if not found
//   - bool: true if the container was found, false otherwise
//
// Concurrency safety:
//   - Not safe for concurrent access, caller must hold read lock
func (rf *ResultsForest) getContainer(resultID flow.Identifier) (*ExecutionResultContainer, bool) {
	container, ok := rf.forest.GetVertex(resultID)
	if !ok {
		return nil, false
	}
	return container.(*ExecutionResultContainer), true
}

// iterateChildren iterates over all children of the given result ID and calls the provided function on each child.
//
// Parameters:
//   - resultID: the ID of the result whose children to iterate
//   - fn: function to call on each child, return false to stop iteration
//
// Concurrency safety:
//   - Not safe for concurrent access, caller must hold read lock
func (rf *ResultsForest) iterateChildren(resultID flow.Identifier, fn func(*ExecutionResultContainer) bool) {
	siblings := rf.forest.GetChildren(resultID)
	for siblings.HasNext() {
		sibling := siblings.NextVertex().(*ExecutionResultContainer)
		if !fn(sibling) {
			return
		}
	}
}

// OnResultSealed marks the execution result as sealed and updates the state of related pipelines.
//
// Parameters:
//   - resultID: the ID of the result that was sealed
//
// Returns:
//   - error: any error that occurred during the operation
//
// No errors are expected during normal operation.
//
// Concurrency safety:
//   - Safe for concurrent access
func (rf *ResultsForest) OnResultSealed(resultID flow.Identifier) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// Get the container for the newly sealed result
	sealedResult, found := rf.getContainer(resultID)
	if !found {
		// the sealed result may not be loaded yet. we can ignore this notification for now and
		// handle it during a future call.
		return nil
	}

	unsealedContainers, err := rf.findUnsealedAncestors(sealedResult)
	if err != nil {
		return err
	}

	for _, container := range unsealedContainers {
		rf.markResultSealed(container)
	}

	rf.lastSealedResultID = resultID

	return nil
}

// findUnsealedAncestors returns all unsealed containers between the last sealed container (excluded)
// and the provided `head` of an execution fork. The returned containers are ordered by ascending views
// of the executed blocks.
// CAUTION: not concurrency safe!
//
// findUnsealedAncestors expects that `head` is a descendant of the last sealed result and that all
// intermediate results are also in the forest. Otherwise, an exception is returned.
func (rf *ResultsForest) findUnsealedAncestors(head *ExecutionResultContainer) ([]*ExecutionResultContainer, error) {
	unsealedContainers := []*ExecutionResultContainer{head}

	for {
		parentID, _ := head.Parent()
		if parentID == rf.lastSealedResultID {
			break
		}

		parent, found := rf.getContainer(parentID)
		if !found {
			return nil, fmt.Errorf("ancestor result %s of %s not found in forest", parentID, head.resultID)
		}

		unsealedContainers = append(unsealedContainers, parent)
		head = parent
	}

	// Sort containers by view in ascending order
	sort.Slice(unsealedContainers, func(i, j int) bool {
		return unsealedContainers[i].blockHeader.View < unsealedContainers[j].blockHeader.View
	})

	return unsealedContainers, nil
}

// markResultSealed marks a result as sealed and updates its siblings' pipelines to abandoned.
//
// Parameters:
//   - container: the container to mark as sealed
//
// Concurrency safety:
//   - Not safe for concurrent access, caller must hold read lock
func (rf *ResultsForest) markResultSealed(container *ExecutionResultContainer) {
	container.Pipeline().SetSealed()

	// Mark siblings' pipelines as abandoned
	parentID, _ := container.Parent()
	rf.iterateChildren(parentID, func(sibling *ExecutionResultContainer) bool {
		if sibling.resultID != container.resultID {
			sibling.Pipeline().OnParentStateUpdated(optimistic_sync.StateAbandoned)
		}
		return true
	})
}

// OnBlockFinalized signals that the given block is finalized.
// It finds all vertices for results of blocks that conflict with the finalized block and abort them.
//
// Parameters:
//   - finalizedBlockID: the ID of the block that was finalized
//   - parentBlockResultIDs: list of IDs for results for the finalized block's parent
//
// Concurrency safety:
//   - Safe for concurrent access
func (rf *ResultsForest) OnBlockFinalized(finalizedBlockID flow.Identifier, parentBlockResultIDs []flow.Identifier) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// 1. Get all ExecutionResults for the finalized block's parent (done by caller)
	// 2. For each of these results, get all child vertices
	// 3. For each child vertex, cancel if it does not reference the finalized block

	for _, parentResultID := range parentBlockResultIDs {
		rf.iterateChildren(parentResultID, func(child *ExecutionResultContainer) bool {
			if child.blockHeader.ID() != finalizedBlockID {
				child.Pipeline().OnParentStateUpdated(optimistic_sync.StateAbandoned)
			}
			return true
		})
	}
}

// OnStateUpdated is called by pipeline state machines when their state changes, and propagates the
// state update to all children of the result.
//
// Parameters:
//   - resultID: the ID of the result whose state was updated
//   - newState: the new state of the pipeline
//
// Concurrency safety:
//   - Safe for concurrent access
func (rf *ResultsForest) OnStateUpdated(resultID flow.Identifier, newState optimistic_sync.State) {
	// send state update to all children.
	rf.mu.RLock()
	rf.iterateChildren(resultID, func(child *ExecutionResultContainer) bool {
		child.Pipeline().OnParentStateUpdated(newState)
		return true
	})
	rf.mu.RUnlock()

	// process completed pipelines
	if newState == optimistic_sync.StateComplete {
		if err := rf.processCompleted(resultID); err != nil {
			// TODO: handle with a irrecoverable error
			rf.log.Fatal().Err(err).Msg("irrecoverable exception: failed to process completed pipeline")
		}
	}
}

// processCompleted processes a completed pipeline and prunes the forest.
//
// Parameters:
//   - resultID: the ID of the result whose pipeline completed
//
// Returns:
//   - error: any error that occurred during the operation
//
// No errors are expected during normal operation. An error indicates the forest's internal state is
// corrupted.
//
// Concurrency safety:
//   - Safe for concurrent access
func (rf *ResultsForest) processCompleted(resultID flow.Identifier) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// first, ensure that the result ID is in the forest, otherwise the forest is in an inconsistent state
	container, found := rf.getContainer(resultID)
	if !found {
		return fmt.Errorf("result %s not found in forest", resultID)
	}

	// next, ensure that this result matches the latest persisted sealed result, otherwise
	// the forest is in an inconsistent state since persisting must be done sequentially
	latestResultID, _ := rf.latestPersistedSealedResult.Latest()
	if container.resultID != latestResultID {
		return fmt.Errorf("completed result %s does not match latest persisted sealed result %s",
			container.resultID, latestResultID)
	}

	// finally, prune the forest up to the latest persisted result's block view
	latestPersistedView := container.blockHeader.View
	err := rf.pruneUpToView(latestPersistedView)
	if err != nil {
		return fmt.Errorf("failed to prune results forest (view: %d): %w", latestPersistedView, err)
	}

	return nil
}
