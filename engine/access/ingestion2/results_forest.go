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

type PipelineFactory interface {
	NewPipeline(result *flow.ExecutionResult, isSealed bool) optimistic_sync.Pipeline
}

// ResultsForest is a mempool holding execution results and receipts, which is aware of the tree structure
// formed by the results. The mempool supports pruning by view: only results
// descending from the latest sealed and finalized result are relevant. Hence, we
// can prune all results for blocks _below_ the latest block with a finalized seal.
// Results of sufficient view for forks that conflict with the finalized fork are
// retained. However, such orphaned forks do not grow anymore and their results
// will be progressively flushed out with increasing sealed-finalized view.
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
	pipelineFactory             PipelineFactory
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

// AddResult adds an execution result to the forest without any receipts.
//
// Expected errors during normal operations:
//   - ErrMaxViewDeltaExceeded: if the result's block view is more than maxViewDelta views ahead of the last sealed view
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (rf *ResultsForest) AddResult(result *flow.ExecutionResult) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	_, err := rf.getOrCreateExecutionResultContainer(result)
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

	container, err := rf.getOrCreateExecutionResultContainer(&receipt.ExecutionResult)
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
//
// Expected errors during normal operations:
//   - ErrMaxViewDeltaExceeded: if the result's block view is more than maxViewDelta views ahead of the last sealed view
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
//
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) getOrCreateExecutionResultContainer(result *flow.ExecutionResult) (*ExecutionResultContainer, error) {
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

	// TODO: determine if the result is sealed
	// implement this when adding the loader functionality
	var isSealed bool
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

	vertex, found := rf.forest.GetVertex(resultID)
	if !found {
		return false
	}
	return vertex.(*ExecutionResultContainer).Has(receiptID)
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
	unsealedContainers, err := rf.findUnsealedContainersInPath(sealedResult)
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

// findUnsealedContainersInPath finds all unsealed containers in the path from the current container
// to the last sealed container (exclusive). The returned containers are sorted by block header view
// in ascending order. Returns an error if any ancestor is not found in the forest. Returns an empty
// slice if the current container is already sealed.
//
// No errors are expected during normal operation.
//
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) findUnsealedContainersInPath(current *ExecutionResultContainer) ([]*ExecutionResultContainer, error) {
	unsealedContainers := make([]*ExecutionResultContainer, 0)

	if rf.lastSealedView >= current.blockHeader.View {
		return unsealedContainers, nil
	}
	unsealedContainers = append(unsealedContainers, current)

	for {
		parentID, _ := current.Parent()
		if parentID == rf.lastSealedResultID {
			break
		}

		parent, found := rf.getContainer(parentID)
		if !found {
			return nil, fmt.Errorf("ancestor result %s of %s not found in forest", parentID, current.resultID)
		}

		unsealedContainers = append(unsealedContainers, parent)
		current = parent
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
//
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
//
// No errors are expected during normal operation.
//
// CAUTION: not concurrency safe! Caller must hold a lock.
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
//
// Returns:
//   - descends: true if the container's result descends from the latest sealed result, otherwise false
//   - connected: true if the container's result descends from a sealed or abandoned result, otherwise false
//
// CAUTION: not concurrency safe! Caller must hold a lock.
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
