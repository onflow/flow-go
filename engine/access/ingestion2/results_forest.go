package ingestion2

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	pipeline "github.com/onflow/flow-go/module/executiondatasync/optimistic_syncing"
	"github.com/onflow/flow-go/module/forest"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/state_synchronization/requester"
)

const (
	// DefaultMaxProcessingSize is the default maximum number of pipelines that can be run in parallel
	// CAUTION: each pipeline may hold hundreds of MB of data in memory, so update with care.
	DefaultMaxProcessingSize = 20
)

var (
	// ErrResultNotFound is returned when a result is not found in the forest.
	ErrResultNotFound = fmt.Errorf("result not found")
)

// ResultsForest is a mempool holding processing pipelines for ingesting execution data
// for a particular ExecutionResult. The mempool is aware of the tree structure
// formed by the results. The mempool supports pruning by view: only results
// descending from the latest persisted sealed result are relevant. Hence, we
// can prune all results for blocks _below_ the latest persisted sealed result.
// Results of sufficient views for forks that conflict with the finalized fork are
// retained. However, such orphaned forks do not grow anymore and their results
// will be progressively flushed out with increasing sealed view.
//
// Safe for concurrent access. Internally, the mempool utilizes the LevelledForrest.
// For an in-depth discussion of the core algorithm, see ./Fork-Aware_Mempools.md
type ResultsForest struct {
	component.Component

	log zerolog.Logger
	cm  *component.ComponentManager

	forest forest.LevelledForest

	// maxRunningCount is the maximum number of pipelines that can be run in parallel.
	maxRunningCount uint

	// latestSealedResultID is the ID of vertex that was most recently marked sealed.
	// This may be the same as the latest persisted sealed result ID, or a descendent.
	// This is used by the sealing logic to backfill any results that were sealed during loading.
	latestSealedResultID flow.Identifier

	// latestPersistedSealedResult holds the resultID of the latest sealed result that is fully indexed
	// and persisted to the database.
	latestPersistedSealedResult *LatestPersistedSealedResult

	// loadingComplete is set to true when all results have been loaded from the database into the forest.
	loadingComplete bool

	// notifier is used to signal to the pipelineManager>Loop that a new vertex is available, or one has completed
	notifier engine.Notifier

	mu sync.RWMutex
}

// NewResultsForest instantiates a ResultsForest
func NewResultsForest(
	log zerolog.Logger,
	latestPersistedSealedResult *LatestPersistedSealedResult,
) *ResultsForest {
	f := &ResultsForest{
		log:                         log.With().Str("component", "results_forest").Logger(),
		forest:                      *forest.NewLevelledForest(0),
		maxRunningCount:             DefaultMaxProcessingSize,
		latestSealedResultID:        latestPersistedSealedResult.ResultID(),
		latestPersistedSealedResult: latestPersistedSealedResult,
		notifier:                    engine.NewNotifier(),
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(f.pipelineManagerLoop).
		Build()

	f.Component = cm
	f.cm = cm

	return f
}

func (f *ResultsForest) pipelineManagerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	wg := sync.WaitGroup{}
	ch := f.notifier.Channel()
	for {
		select {
		case <-ctx.Done():
			// wait for all pipelines to gracefully shutdown
			wg.Wait()
			return

		case <-ch:
			runningCount := uint(0)

			// Inspect all vertices in the tree forming from the latest persisted sealed result.
			// Count all running pipelines, and start new ones up to the maxRunningCount.
			// This will iterate over at most 2*maxRunningCount vertices.
			f.visitAllDescendantsBFS(f.latestPersistedSealedResult.ResultID(), func(container *ExecutionResultContainer) bool {
				state := container.pipeline.GetState()

				switch state {
				case pipeline.StateAbandoned:
					// TODO: free the pipeline's resources (not necessarily here)
					return true

				case pipeline.StateComplete:
					if err := f.processCompleted(container.resultID); err != nil {
						ctx.Throw(err)
					}
					return true

				case pipeline.StatePending:
					wg.Add(1)
					go func() {
						defer wg.Done()

						// TODO: This core stubbed with nil storages, but needs to be initialized with real data
						core, err := pipeline.NewCoreImpl(
							f.log,
							container.result,
							container.blockHeader,
							nil,
							requester.OneshotExecutionDataConfig{
								FetchTimeout:    requester.DefaultFetchTimeout,
								MaxFetchTimeout: requester.DefaultMaxFetchTimeout,
								RetryDelay:      requester.DefaultRetryDelay,
								MaxRetryDelay:   requester.DefaultMaxRetryDelay,
							},
							nil,
							nil,
							nil,
							nil,
							nil,
							nil,
							nil,
							nil,
						)
						if err != nil {
							ctx.Throw(fmt.Errorf("core creation failed (result: %s): %w", container.resultID, err)
						}

						err = container.pipeline.Run(ctx, core)
						if err != nil && !errors.Is(err, context.Canceled) {
							ctx.Throw(fmt.Errorf("pipeline execution failed (result: %s): %w", container.resultID, err))
						}
					}()
				}

				runningCount++
				return runningCount < f.maxRunningCount
			})
		}
	}
}

// visitAllDescendantsBFS visits all ancestors of the given result ID in a breadth-first manner, and
// calls the provided function on each ancestor.
// If the function returns false, the traversal is stopped.
func (f *ResultsForest) visitAllDescendantsBFS(resultID flow.Identifier, fn func(*ExecutionResultContainer) bool) {
	queue := []flow.Identifier{resultID}
	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		f.iterateChildren(currentID, func(child *ExecutionResultContainer) bool {
			if !fn(child) {
				return false
			}
			queue = append(queue, child.resultID)
			return true
		})
	}
}

// processCompleted processes a completed pipeline and prunes the forest.
// No errors are expected during normal operation.
func (f *ResultsForest) processCompleted(resultID flow.Identifier) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// first, ensure that the result ID is in the forest, otherwise the forest is in an inconsistent state
	container, ok := f.getContainer(resultID)
	if !ok {
		return fmt.Errorf("state update from unknown result vertex %s", resultID)
	}

	// next, ensure that this result descends from the latest persisted sealed result, otherwise
	// the forest is in an inconsistent state since persisting must be done sequentially
	if container.resultID != f.latestPersistedSealedResult.ResultID() {
		return fmt.Errorf("result %s does not match the latest persisted sealed result %s", container.resultID, f.latestPersistedSealedResult.ResultID())
	}

	// finally, prune the forest up to the latest persisted result's block view
	latestPersistedView := container.blockHeader.View
	err := f.pruneUpToView(latestPersistedView)
	if err != nil {
		return fmt.Errorf("failed to prune results forest (view: %d): %w", latestPersistedView, err)
	}

	return nil
}

// AddResult adds an Execution Result to the Results Forest, in case the result is not already
// stored in the tree.
//
// No errors are expected during normal operation.
func (f *ResultsForest) AddResult(result *flow.ExecutionResult, block *flow.Header, isSealed bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// drop results for block views lower than the lowest view.
	if block.View < f.forest.LowestLevel {
		return nil
	}

	// sanity check: initial result should be for block
	if block.ID() != result.BlockID {
		return fmt.Errorf("result is for different block")
	}

	_, err := f.getExecutionResultContainer(result, block, isSealed)
	if err != nil {
		return fmt.Errorf("failed to get container for result (%x): %w", result.ID(), err)
	}

	f.notifier.Notify()

	return nil
}

// getExecutionResultContainer retrieves the vertex container for the given result or creates a new
// one and stores it into the levelled forest
//
// No errors are expected during normal operation.
func (f *ResultsForest) getExecutionResultContainer(result *flow.ExecutionResult, block *flow.Header, isSealed bool) (*ExecutionResultContainer, error) {
	container, found := f.getContainer(result.ID())
	if found {
		return container, nil
	}

	p := pipeline.NewPipeline(f.log, isSealed, result, f.OnStateUpdated)

	container, err := NewExecutionResultContainer(result, block, p)
	if err != nil {
		return nil, fmt.Errorf("constructing execution result container failed: %w", err)
	}

	if err = f.forest.VerifyVertex(container); err != nil {
		return nil, fmt.Errorf("failed to store execution result container: %w", err)
	}

	f.forest.AddVertex(container)
	return container, nil
}

// OnResultSealed signals that the result with the given ID is sealed.
// This is used to communicate to the processing pipeline that its result is sealed, and to all
// results on abandoned forks that they should stop processing.
//
// No errors are expected during normal operation.
func (f *ResultsForest) OnResultSealed(resultID flow.Identifier) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	hasUpdates, err := f.sealAllAncestors(resultID)
	if err != nil {
		if f.loadingComplete || !errors.Is(err, ErrResultNotFound) {
			return fmt.Errorf("failed to mark ancestors as sealed for result %s: %w", resultID, err)
		}

		// if we're still loading, then we can ignore this notification for now and handle it during
		// a future notification.
		f.log.Debug().
			Str("result_id", resultID.String()).
			Msg("sealed result is not in the tree, ignoring")
		return nil
	}

	if hasUpdates {
		f.notifier.Notify()
	}

	return nil
}

// sealAllAncestors marks all ancestors of the given result ID as sealed.
//
// Expected errors:
//   - ErrResultNotFound: if the result ID or any of its unsealed ancestors are not found in the forest.
func (f *ResultsForest) sealAllAncestors(resultID flow.Identifier) (hasUpdates bool, err error) {
	// Find all unsealed ancestors of the result ID ending on the latest sealed result.
	ancestorsAndSelf, err := f.findUnsealedAncestors(resultID)
	if err != nil {
		return false, err
	}

	// Mark all ancestors as sealed in sealing order, and abort their siblings
	for i := len(ancestorsAndSelf) - 1; i >= 0; i-- {
		sealedContainer := ancestorsAndSelf[i]
		if f.markContainerAsSealed(sealedContainer) {
			hasUpdates = true
		}
		f.latestSealedResultID = sealedContainer.resultID
	}

	return hasUpdates, nil
}

// findUnsealedAncestors finds all ancestor vertices of the given result ID that are not marked sealed.
// The returned slice includes the result itself.
//
// Expected errors:
//   - ErrResultNotFound: if the result ID or any of its unsealed ancestors up to the latest sealed
//     result are not found in the forest.
func (f *ResultsForest) findUnsealedAncestors(resultID flow.Identifier) ([]*ExecutionResultContainer, error) {
	ancestorsAndSelf := make([]*ExecutionResultContainer, 0)

	// Find all unsealed ancestors of the result ID ending on the latest persisted sealed result.
	// abort and return an error if we fail to find any ancestor.
	sealedResultID := resultID
	for {
		sealedContainer, ok := f.getContainer(sealedResultID)
		if !ok {
			// Note: if resultID does not descend from the latest persisted sealed result, iteration
			// will eventually stop when it reaches the pruned level.
			return nil, fmt.Errorf("sealed result %s not found in the tree: %w", sealedResultID, ErrResultNotFound)
		}

		// stop searching once we find the most recently sealed result.
		if sealedContainer.resultID == f.latestSealedResultID {
			break
		}

		ancestorsAndSelf = append(ancestorsAndSelf, sealedContainer)
		sealedResultID, _ = sealedContainer.Parent()
	}

	return ancestorsAndSelf, nil
}

// markContainerAsSealed marks the given container as sealed and aborts all its siblings.
// Returns true if any siblings were aborted.
func (f *ResultsForest) markContainerAsSealed(sealed *ExecutionResultContainer) (hasUpdates bool) {
	sealed.pipeline.SetSealed()
	parentID, _ := sealed.Parent()

	f.iterateChildren(parentID, func(child *ExecutionResultContainer) bool {
		if child.resultID != sealed.resultID {
			child.pipeline.OnParentStateUpdated(pipeline.StateAbandoned)
			hasUpdates = true
		}
		return true
	})

	return hasUpdates
}

// OnBlockFinalized signals that the given block is finalized.
// All pipelines for siblings of the finalized block should be aborted.
func (f *ResultsForest) OnBlockFinalized(blockID flow.Identifier, parentResultIDs ...flow.Identifier) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Find all vertices for results of blocks that conflict with the finalized block and abort them
	// To accomplish this
	// 1. Get all ExecutionResults for the finalized block's parent
	// 2. For each of these results, get all child vertices
	// 3. For each child vertex, cancel if it does not reference the finalized block

	hasUpdates := false
	for _, parentID := range parentResultIDs {
		f.iterateChildren(parentID, func(child *ExecutionResultContainer) bool {
			if child.result.BlockID != blockID {
				child.pipeline.OnParentStateUpdated(pipeline.StateAbandoned)
				hasUpdates = true
			}
			return true
		})
	}

	if hasUpdates {
		f.notifier.Notify()
	}
}

// OnStateUpdated is called by pipeline state machines when their state changes, and propagates the
// state update to all children of the result.
func (f *ResultsForest) OnStateUpdated(resultID flow.Identifier, newState pipeline.State) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Note on concurrency: This is designed to keep the propagation of state updates within the caller's
	// goroutine. This ensures that the state updates are propagated to all descendents before continuing
	// and avoids blocking the forest's loop.

	// propagate the state update to all children of the result
	f.iterateChildren(resultID, func(child *ExecutionResultContainer) bool {
		child.pipeline.OnParentStateUpdated(newState)
		return true
	})

	// notify the pipeline manager loop that a pipeline has completed
	if newState.IsTerminal() {
		f.notifier.Notify()
	}
}

// pruneUpToView prunes all results for all blocks with view up to but
// NOT INCLUDING `level`. Errors if level is lower than
// the previous value (as we cannot recover previously pruned results).
//
// The caller must hold a write lock on the forest's mutex.
//
// No errors are expected during normal operation.
func (f *ResultsForest) pruneUpToView(level uint64) error {
	err := f.forest.PruneUpToLevel(level)
	if err != nil {
		return fmt.Errorf("pruning Levelled Forest up to view (aka level) %d failed: %w", level, err)
	}

	return nil
}

// Size returns the number of receipts stored in the mempool
func (f *ResultsForest) Size() uint {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return uint(f.forest.GetSize())
}

// LowestView returns the lowest view, where results are still stored in the mempool.
func (f *ResultsForest) LowestView() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.forest.LowestLevel
}

// getContainer retrieves the ExecutionResultContainer for the given result ID.
// Returns the container and a boolean indicating whether it was found.
//
// The caller must hold a read lock on the forest's mutex.
func (f *ResultsForest) getContainer(resultID flow.Identifier) (*ExecutionResultContainer, bool) {
	container, ok := f.forest.GetVertex(resultID)
	if !ok {
		return nil, false
	}
	return container.(*ExecutionResultContainer), true
}

// iterateChildren iterates over all children of the given result ID and calls the provided function on each child.
// If the function returns false, the iteration is stopped.
//
// The caller must hold a read lock on the forest's mutex.
func (f *ResultsForest) iterateChildren(resultID flow.Identifier, fn func(*ExecutionResultContainer) bool) {
	siblings := f.forest.GetChildren(resultID)
	for siblings.HasNext() {
		sibling := siblings.NextVertex().(*ExecutionResultContainer)
		if !fn(sibling) {
			return
		}
	}
}
