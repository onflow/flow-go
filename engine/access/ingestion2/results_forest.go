package ingestion2

import (
	"fmt"
	"sort"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/forest"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

var (
	// ErrMaxViewDeltaExceeded is returned when attempting to add a results who's block view is
	// more than maxViewDelta views ahead of the last sealed view.
	ErrMaxViewDeltaExceeded = fmt.Errorf("results block view exceeds accepted range")
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
	headers                     storage.Headers
	maxViewDelta                uint64
	lastSealedResultID          flow.Identifier
	lastSealedView              uint64
	latestPersistedSealedResult storage.LatestPersistedSealedResult
	pipelineFactory             optimistic_sync.PipelineFactory
	mu                          sync.RWMutex
}

// NewResultsForest creates a new instance of ResultsForest.
func NewResultsForest(
	log zerolog.Logger,
	headers storage.Headers,
	latestPersistedSealedResult storage.LatestPersistedSealedResult,
	maxViewDelta uint64,
) (*ResultsForest, error) {
	resultID, sealedHeight := latestPersistedSealedResult.Latest()

	sealedHeader, err := headers.ByHeight(sealedHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to get block header for latest persisted sealed result (height: %d): %w", sealedHeight, err)
	}

	rf := &ResultsForest{
		log:                         log.With().Str("component", "results_forest").Logger(),
		forest:                      *forest.NewLevelledForest(sealedHeader.View),
		headers:                     headers,
		maxViewDelta:                maxViewDelta,
		lastSealedResultID:          resultID,
		latestPersistedSealedResult: latestPersistedSealedResult,
	}

	return rf, nil
}

// AddSealedResult adds a sealed Execution Result to the Result Forest (without any receipts), in
// case the result is not already stored in the tree.
// This is useful for crash recovery:
// After recovering from a crash, the mempools are wiped and the sealed results will not
// be stored in the Execution Tree anymore. Adding the result to the tree allows to create
// a vertex in the tree without attaching any Execution Receipts to it.
//
// Expected errors during normal operations:
//   - ErrMaxViewDeltaExceeded: if the result's block view is more than maxViewDelta views ahead of the last sealed view
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
//
// TODO: during normal operations we should never add a result without a receipt, this is only used for crash recovery
func (rf *ResultsForest) AddSealedResult(result *flow.ExecutionResult) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	_, err := rf.getOrCreateExecutionResultContainer(result, true)
	if err != nil {
		return fmt.Errorf("failed to get container for result (%s): %w", result.ID(), err)
	}
	return nil
}

// AddReceipt adds the given execution receipt to the forest.
//
// Expected errors during normal operations:
//   - ErrMaxViewDeltaExceeded: if the result's block view is more than maxViewDelta views ahead of the last sealed view
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (rf *ResultsForest) AddReceipt(receipt *flow.ExecutionReceipt) (bool, error) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	container, err := rf.getOrCreateExecutionResultContainer(&receipt.ExecutionResult, false)
	if err != nil {
		return false, fmt.Errorf("failed to get container for result (%s): %w", receipt.ExecutionResult.ID(), err)
	}

	if container == nil {
		// noop if the result's block view is lower than the lowest view.
		return false, nil
	}

	added, err := container.AddReceipt(receipt)
	if err != nil {
		return false, fmt.Errorf("failed to add receipt to its container: %w", err)
	}

	return added > 0, nil
}

// getOrCreateExecutionResultContainer retrieves or creates the container for the given result.
// CAUTION: not concurrency safe! Caller must hold a lock.
//
// Expected errors during normal operations:
//   - ErrMaxViewDeltaExceeded: if the result's block view is more than maxViewDelta views ahead of the last sealed view
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (rf *ResultsForest) getOrCreateExecutionResultContainer(result *flow.ExecutionResult, isSealed bool) (*ExecutionResultContainer, error) {
	// First try to get existing container
	resultID := result.ID()
	container, found := rf.getContainer(resultID)
	if found {
		return container, nil
	}

	executedBlock, err := rf.headers.ByBlockID(result.BlockID)
	if err != nil {
		// this is an exception since only results for certified blocks should be added to the forest
		return nil, fmt.Errorf("failed to get block header for result (%s): %w", resultID, err)
	}

	// drop receipts for block views lower than the lowest view.
	if executedBlock.View < rf.forest.LowestLevel {
		return nil, nil
	}

	// make sure the result's block view is within the accepted range
	if executedBlock.View > rf.forest.LowestLevel+rf.maxViewDelta {
		return nil, ErrMaxViewDeltaExceeded
	}

	pipeline := rf.pipelineFactory.NewPipeline(result, isSealed)
	container, err = NewExecutionResultContainer(result, executedBlock, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to create container for result (%s): %w", resultID, err)
	}

	// Verify and add to forest
	err = rf.forest.VerifyVertex(container)
	if err != nil {
		return nil, fmt.Errorf("failed to store receipt's container: %w", err)
	}
	rf.forest.AddVertex(container)

	// mark the container as abandoned if it does not descend from the latest sealed result.
	//
	// consider the following case:
	// X is the result that was just added
	// A was previously sealed, and B was sealed before X was added
	//
	//   ↙ X
	// A ← B ← C
	//
	// in this case, we know that X conflicts with B and will never be sealed. We should abandon X
	// immediately.
	//
	// consider another case:
	// Y is the result that was just added
	// X is its parent, but does not exist in the forest yet
	//
	//   ↙ [X] ← Y
	// A ← B ← C
	//
	// in this case, we do not know if Y will eventually be sealed since we don't know which result
	// X will descend from. We must wait until we eventually receive X to determine if X and Y should
	// be abandoned.
	if descends, connected := rf.descendsFromLatestSealedResult(container); connected && !descends {
		rf.abandonFork(container)
	}

	return container, nil
}

// HasReceipt checks if a receipt exists in the forest.
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

// Size returns the number of results stored in the forest.
func (rf *ResultsForest) Size() uint {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return uint(rf.forest.GetSize())
}

// LowestView returns the lowest view where results are still stored in the mempool.
func (rf *ResultsForest) LowestView() uint64 {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.forest.LowestLevel
}

// GetContainer retrieves the ExecutionResultContainer for the given result ID.
func (rf *ResultsForest) GetContainer(resultID flow.Identifier) (*ExecutionResultContainer, bool) {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.getContainer(resultID)
}

// getContainer retrieves the ExecutionResultContainer for the given result ID without locking.
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) getContainer(resultID flow.Identifier) (*ExecutionResultContainer, bool) {
	container, ok := rf.forest.GetVertex(resultID)
	if !ok {
		return nil, false
	}
	return container.(*ExecutionResultContainer), true
}

// IterateChildren iterates over all children of the given result ID and calls the provided function on each child.
// Callback function should return false to stop iteration
func (rf *ResultsForest) IterateChildren(resultID flow.Identifier, fn func(*ExecutionResultContainer) bool) {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	rf.iterateChildren(resultID, fn)
}

// iterateChildren iterates over all children of the given result ID and calls the provided function on each child.
// Callback function should return false to stop iteration
// CAUTION: not concurrency safe! Caller must hold a lock.
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
// No errors are expected during normal operation.
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

	// collect all unsealed containers in the path from the sealed result to the last sealed result.
	// this ensures:
	// 1. the newly sealed result descends from the last sealed result (state is consistent)
	// 2. any sealing notifications that were missed due to undiscovered results are handled
	// 3. sealing notifications are processed in sealing order
	unsealedContainers, err := rf.findUnsealedAncestors(sealedResult)
	if err != nil {
		return err
	}

	// if this notification is for an already sealed result, we can ignore it.
	if len(unsealedContainers) == 0 {
		return nil
	}

	for _, container := range unsealedContainers {
		rf.markResultSealed(container)
	}

	rf.lastSealedResultID = resultID
	rf.lastSealedView = sealedResult.blockHeader.View

	return nil
}

// findUnsealedAncestors returns all unsealed containers between the last sealed container (excluded)
// and the provided `head` of an execution fork. The returned containers are ordered by ascending views
// of the executed blocks.
// CAUTION: not concurrency safe!
//
// findUnsealedAncestors expects that `head` is a descendant of the last sealed result and that all
// intermediate results are also in the forest. Otherwise, an exception is returned.
//
// No errors are expected during normal operation.
func (rf *ResultsForest) findUnsealedAncestors(head *ExecutionResultContainer) ([]*ExecutionResultContainer, error) {
	unsealedContainers := make([]*ExecutionResultContainer, 0)

	if rf.lastSealedView >= head.blockHeader.View {
		return unsealedContainers, nil
	}
	unsealedContainers = append(unsealedContainers, head)

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
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) markResultSealed(container *ExecutionResultContainer) {
	container.Pipeline().SetSealed()

	// abandon all conflicting forks
	parentID, _ := container.Parent()
	rf.iterateChildren(parentID, func(sibling *ExecutionResultContainer) bool {
		if sibling.resultID != container.resultID {
			rf.abandonFork(sibling)
		}
		return true
	})
}

// OnBlockFinalized signals that the given block is finalized.
// It finds all vertices for results of blocks that conflict with the finalized block and abort them.
func (rf *ResultsForest) OnBlockFinalized(finalizedBlockID flow.Identifier, parentBlockResultIDs []flow.Identifier) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// 1. Get all ExecutionResults for the finalized block's parent (done by caller)
	// 2. For each of these results, get all child vertices
	// 3. For each child vertex, cancel if it does not reference the finalized block

	for _, parentResultID := range parentBlockResultIDs {
		rf.iterateChildren(parentResultID, func(child *ExecutionResultContainer) bool {
			if child.blockHeader.ID() != finalizedBlockID {
				rf.abandonFork(child)
			}
			return true
		})
	}
}

// abandonFork recursively abandons a container and all its descendants.
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) abandonFork(container *ExecutionResultContainer) {
	container.Pipeline().Abandon()
	rf.iterateChildren(container.resultID, func(child *ExecutionResultContainer) bool {
		rf.abandonFork(child)
		return true
	})
}

// OnStateUpdated is called by pipeline state machines when their state changes, and propagates the
// state update to all children of the result.
func (rf *ResultsForest) OnStateUpdated(resultID flow.Identifier, newState optimistic_sync.State) {
	// abandoned status is propagated to all descendants synchronously, so no need to traverse again here.
	if newState == optimistic_sync.StateAbandoned {
		return
	}

	// send state update to all children.
	rf.IterateChildren(resultID, func(child *ExecutionResultContainer) bool {
		child.Pipeline().OnParentStateUpdated(newState)
		return true
	})

	// process completed pipelines
	if newState == optimistic_sync.StateComplete {
		rf.mu.Lock()
		defer rf.mu.Unlock()

		if err := rf.processCompleted(resultID); err != nil {
			// TODO: handle with a irrecoverable error
			rf.log.Fatal().Err(err).Msg("irrecoverable exception: failed to process completed pipeline")
		}
	}
}

// processCompleted processes a completed pipeline and prunes the forest.
// CAUTION: not concurrency safe! Caller must hold a lock.
//
// No errors are expected during normal operation.
func (rf *ResultsForest) processCompleted(resultID flow.Identifier) error {
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
	err := rf.forest.PruneUpToLevel(latestPersistedView)
	if err != nil {
		return fmt.Errorf("failed to prune results forest (view: %d): %w", latestPersistedView, err)
	}

	return nil
}

// descendsFromLatestSealedResult checks if a container's result is a descendant of the latest sealed result.
// CAUTION: not concurrency safe! Caller must hold a lock.
//
// Returns:
//   - descends: true if the container's result descends from the latest sealed result, otherwise false
//   - connected: true if the container's result descends from a sealed or abandoned result, otherwise false
func (rf *ResultsForest) descendsFromLatestSealedResult(container *ExecutionResultContainer) (descends bool, connected bool) {
	current := container
	for {
		parentID, parentView := current.Parent()
		if parentID == rf.lastSealedResultID {
			return true, true
		}

		// sealed views are strictly increasing, so if we find a parent view that is lower than the
		// last sealed view, and we have not yet found the latest sealed result, then we are guaranteed
		// to never find it.
		if parentView < rf.lastSealedView {
			return false, true
		}

		// if the parent is not found, that means either the parent has not been added to the forest yet,
		// or it has already been pruned. either way, we can't confirm that the container descends
		// from the latest sealed result.
		parent, found := rf.getContainer(parentID)
		if !found {
			return false, false
		}

		// if the parent is abandoned, then the container does not descend from the latest sealed
		// result, and we can guarantee that the container will never be started.
		if parent.Pipeline().GetState() == optimistic_sync.StateAbandoned {
			return false, true
		}

		current = parent
	}
}
