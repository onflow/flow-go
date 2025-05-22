package ingestion2

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	pipeline "github.com/onflow/flow-go/module/executiondatasync/optimistic_syncing"
	"github.com/onflow/flow-go/module/forest"
	"github.com/onflow/flow-go/module/irrecoverable"
)

const (
	// DefaultMaxProcessingSize is the default maximum number of pipelines that can be run in parallel
	DefaultMaxProcessingSize = 20
)

type Pipeline interface {
	Run(context.Context, pipeline.Core) error
	GetState() pipeline.State
	SetSealed()
	OnParentStateUpdated(pipeline.State)
}

// ResultsForest is a mempool holding receipts, which is aware of the tree structure
// formed by the results. The mempool supports pruning by height: only results
// descending from the latest sealed and finalized result are relevant. Hence, we
// can prune all results for blocks _below_ the latest block with a finalized seal.
// Results of sufficient height for forks that conflict with the finalized fork are
// retained. However, such orphaned forks do not grow anymore and their results
// will be progressively flushed out with increasing sealed-finalized height.
//
// Safe for concurrent access. Internally, the mempool utilizes the LevelledForrest.
// For an in-depth discussion of the core algorithm, see ./Fork-Aware_Mempools.md
type ResultsForest struct {
	component.Component

	log zerolog.Logger
	cm  *component.ComponentManager

	forest forest.LevelledForest

	maxRunningCount uint

	latestPersistedSealedResultID flow.Identifier

	// completed is used to pass state machine update notifications from the pipelines to the handler
	// Note: updates are pushed onto the completed channel even after the component begins shutdown.
	// This allows the forest to handle all final completed state machine updates. It relies on the
	// pipelines to stop gracefully when the context is done.
	// the completed channel must only be closed after all pipelines have stopped.
	completed chan flow.Identifier

	// notifier is used to signal to the pipelineManager>Loop that a new vertex is available, or one has completed
	notifier engine.Notifier

	mu sync.RWMutex
}

// NewResultsForest instantiates a ResultsForest
func NewResultsForest(
	log zerolog.Logger,
) *ResultsForest {
	f := &ResultsForest{
		log:             log.With().Str("component", "results_forest").Logger(),
		forest:          *forest.NewLevelledForest(0),
		completed:       make(chan flow.Identifier, 1),
		notifier:        engine.NewNotifier(),
		maxRunningCount: DefaultMaxProcessingSize,
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(f.pipelineManagerLoop).
		AddWorker(f.processCompletedLoop).
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

			// it is safe to close this channel since it is only written to by updates passed
			// from pipelines, and all pipelines are stopped.
			// this signals to the processCompletedLoop that it should shut down
			close(f.completed)
			return

		case <-ch:
			runningCount := uint(0)

			// Inspect all vertices in the tree forming from the latest persisted sealed result.
			// Count all running pipelines, and start new ones up to the maxRunningCount.
			// This will iterate over at most maxRunningCount vertices.
			f.visitAllAncestorsBFS(f.latestPersistedSealedResultID, func(container *ExecutionResultContainer) bool {
				state := container.pipeline.GetState()
				if state == pipeline.StateComplete || state == pipeline.StateCanceled {
					return true
				}

				if state == pipeline.StateUninitialized {
					wg.Add(1)
					go func() {
						defer wg.Done()

						core := pipeline.NewCore()
						if err := container.pipeline.Run(ctx, core); err != nil {
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

// visitAllAncestorsBFS visits all ancestors of the given result ID in a breadth-first manner, and
// calls the provided function on each ancestor.
// If the function returns false, the traversal is stopped.
func (f *ResultsForest) visitAllAncestorsBFS(resultID flow.Identifier, fn func(*ExecutionResultContainer) bool) {
	queue := []flow.Identifier{resultID}

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		children := f.forest.GetChildren(currentID)
		for children.HasNext() {
			child := children.NextVertex().(*ExecutionResultContainer)
			if !fn(child) {
				return
			}
			queue = append(queue, child.resultID)
		}
	}
}

// processCompletedLoop processes completed messages from the pipelines, and prunes the forest as pipelines complete.
func (f *ResultsForest) processCompletedLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	for {
		resultID, ok := <-f.completed
		if !ok {
			// exit after fully draining the channel
			// note: since this loop only handles fully completed pipelines and persisting is done
			// sequentially, there will only ever be zero or one pipelines waiting with an update.
			return
		}

		// first ensure that the result ID is in the forest, otherwise the forest is in an inconsistent state
		updater, ok := f.forest.GetVertex(resultID)
		if !ok {
			ctx.Throw(fmt.Errorf("state update from unknown result vertex %s", resultID))
		}

		latestPersistedHeight := updater.(*ExecutionResultContainer).blockHeader.Height

		err := f.pruneUpToHeight(latestPersistedHeight)
		if err != nil {
			ctx.Throw(fmt.Errorf("failed to prune results forest (height: %d): %w", latestPersistedHeight, err))
		}
	}
}

// AddResult adds an Execution Result to the Results Forest, in case the result is not already
// stored in the tree.
//
// No errors are expected during normal operation.
func (f *ResultsForest) AddResult(result *flow.ExecutionResult, block *flow.Header, isSealed bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// drop results for block heights lower than the lowest height.
	if block.Height < f.forest.LowestLevel {
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
	vertex, found := f.forest.GetVertex(result.ID())
	if found {
		return vertex.(*ExecutionResultContainer), nil
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
	// TODO: handle the case where we are still loading results into the tree
	// 	- possible solution: ignore missing vertices, then seal all ancestors when a disconnected
	// 		result is sealed (last sealed result != previousResultID)
	sealedVertex, ok := f.forest.GetVertex(resultID)
	if !ok {
		return fmt.Errorf("failed to mark result sealed: vertex not found")
	}

	// Mark the vertex as sealed
	sealedContainer := sealedVertex.(*ExecutionResultContainer)
	sealedContainer.pipeline.SetSealed()
	parentID, _ := sealedContainer.Parent()

	// Update all siblings that they no longer descend from the latest sealed result
	hasUpdates := false
	siblings := f.forest.GetChildren(parentID)
	for siblings.HasNext() {
		sibling := siblings.NextVertex().(*ExecutionResultContainer)
		if sibling.resultID != resultID {
			sibling.pipeline.OnParentStateUpdated(pipeline.StateCanceled)
			hasUpdates = true
		}
	}

	if hasUpdates {
		f.notifier.Notify()
	}

	return nil
}

// OnBlockFinalized signals that the given block is finalized.
// All pipelines for siblings of the finalized block should be aborted.
func (f *ResultsForest) OnBlockFinalized(blockID flow.Identifier, parentResultIDs ...flow.Identifier) {
	// Find all vertices for results of blocks that conflict with the finalized block and cancel them
	// To accomplish this
	// 1. Get all ExecutionResults for the finalized block's parent
	// 2. For each of these results, get all child vertices
	// 3. For each child vertex, cancel if it does not reference the finalized block

	hasUpdates := false
	for _, parentID := range parentResultIDs {
		siblings := f.forest.GetChildren(parentID)
		for siblings.HasNext() {
			sibling := siblings.NextVertex().(*ExecutionResultContainer)
			if sibling.result.BlockID != blockID {
				sibling.pipeline.OnParentStateUpdated(pipeline.StateCanceled)
				hasUpdates = true
			}
		}
	}

	if hasUpdates {
		f.notifier.Notify()
	}
}

// OnStateUpdated is called by pipeline state machines when their state changes, and propagates the
// state update to all children of the result.
func (f *ResultsForest) OnStateUpdated(resultID flow.Identifier, newState pipeline.State) {
	// Note on concurrency: This is designed to keep the propagation of state updates within the caller's
	// goroutine. This ensures that the state updates are propagated to all descendents before continuing
	// and avoids blocking the forest's loop.
	// Only completed pipeline notifications are handled by the forest's pruning loop to ensure that
	// updates are handled in order

	// propagate the state update to all children of the result
	children := f.forest.GetChildren(resultID)
	for children.HasNext() {
		child := children.NextVertex().(*ExecutionResultContainer)
		child.pipeline.OnParentStateUpdated(newState)
	}

	// notify the pipeline manager loop that a pipeline has completed
	if newState == pipeline.StateCanceled || newState == pipeline.StateComplete {
		f.notifier.Notify()
	}

	// pass the completed resultID to the pruning loop
	if newState == pipeline.StateComplete {
		f.completed <- resultID
	}
}

// pruneUpToHeight prunes all results for all blocks with height up to but
// NOT INCLUDING `level`. Errors if level is lower than
// the previous value (as we cannot recover previously pruned results).
//
// No errors are expected during normal operation.
func (f *ResultsForest) pruneUpToHeight(level uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	err := f.forest.PruneUpToLevel(level)
	if err != nil {
		return fmt.Errorf("pruning Levelled Forest up to height (aka level) %d failed: %w", level, err)
	}

	return nil
}

// Size returns the number of receipts stored in the mempool
func (f *ResultsForest) Size() uint {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return uint(f.forest.GetSize())
}

// LowestHeight returns the lowest height, where results are still stored in the mempool.
func (f *ResultsForest) LowestHeight() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.forest.LowestLevel
}
