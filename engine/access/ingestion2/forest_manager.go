package ingestion2

import (
	"container/list"
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/queue"
	"github.com/onflow/flow-go/utils/logging"
)

// ForestManagerConfig contains configuration for processing execution result containers
type ForestManagerConfig struct {
	// RequiredAgreeingExecutors is the minimum number of executors that must agree on the result
	// Must be 1 or greater.
	RequiredAgreeingExecutors uint
	// RequiredExecutors is the set of executor IDs that must have provided receipts
	// At least one of these executors must have produced a receipt for the result to be processed
	RequiredExecutors map[flow.Identifier]struct{}
	// RequiredBlockStatus is the minimum block status required for processing
	RequiredBlockStatus BlockStatus
}

// DefaultForestManagerConfig returns the default configuration for the forest manager.
func DefaultForestManagerConfig() ForestManagerConfig {
	return ForestManagerConfig{
		RequiredAgreeingExecutors: 2,
		RequiredExecutors:         make(map[flow.Identifier]struct{}),
		RequiredBlockStatus:       BlockStatusSealed,
	}
}

func (c ForestManagerConfig) Validate() error {
	switch c.RequiredBlockStatus {
	case BlockStatusSealed, BlockStatusFinalized, BlockStatusCertified:
	default:
		return fmt.Errorf("invalid block status: %s", c.RequiredBlockStatus)
	}

	if c.RequiredAgreeingExecutors == 0 {
		return fmt.Errorf("required agreeing executors must be greater than 0")
	}

	return nil
}

// ForestManager iterates the ResultsForest starting from the latest persisted sealed result
// to find processable ancestors that are not running yet. It uses a breadth-first traversal
// approach to ensure ancestors are processed before descendants.
// The forest manager is intended to be run in a single goroutine as a component.ComponentWorker
// via the WorkerLoop method.
//
// Note: the forest manager adds tasks to the work queue without an explicit limit. The total number
// of tasks is naturally bounded by the size of the results forest. Since each task only stores a
// function pointer, the memory impact of the work queue is minimal compared to the results forest.
type ForestManager struct {
	log           zerolog.Logger
	resultsForest *ResultsForest
	workQueue     *queue.ConcurrentPriorityQueue[PipelineTask]
	config        ForestManagerConfig
	coreFactory   optimistic_sync.CoreFactory
}

// NewForestManager creates a new instance of ForestManager.
func NewForestManager(
	log zerolog.Logger,
	resultsForest *ResultsForest,
	workQueue *queue.ConcurrentPriorityQueue[PipelineTask],
	config ForestManagerConfig,
	coreFactory optimistic_sync.CoreFactory,
) (*ForestManager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid forest manager config: %w", err)
	}

	return &ForestManager{
		log:           log.With().Str("component", "forest_manager").Logger(),
		resultsForest: resultsForest,
		workQueue:     workQueue,
		config:        config,
		coreFactory:   coreFactory,
	}, nil
}

// OnResultUpdated notifies the manager that a result was updated.
func (fm *ForestManager) OnResultUpdated(container *ExecutionResultContainer) {
	if container.IsEnqueued() {
		// already enqueued, adding this receipt will not change processability of any of its
		// descendants
		return
	}

	fm.traverseDescendants(container)
}

// OnBlockFinalized notifies the manager that a new block was finalized.
func (fm *ForestManager) OnBlockFinalized(finalized *flow.Block) {
	// find all containers that are now processable given the new finalized block.
	// We optimize the search by inspecting only the containers that are affected by the finalized
	// block status update.

	switch fm.config.RequiredBlockStatus {
	case BlockStatusSealed:
		// per protocol specification, seals can only be included in a block if the result's parent is
		// already sealed. This means that for any given block, if there are multiple seals, they form
		// a continuous chain extending from the previous latest sealed result. We can then optimize
		// the search by picking any seal, and finding its oldest pending ancestor.
		if len(finalized.Payload.Seals) > 0 {
			// the protocol does not guarantee that seals are included in view order. Hence, we pick
			// the first seal and find its oldest pending ancestor. In the worst case, this is the same
			// as looking up the containers for each seal to find the lowest view.
			seal := finalized.Payload.Seals[0]
			if container, found := fm.resultsForest.GetContainer(seal.ResultID); found {
				if pendingAncestor, found := fm.oldestPendingAncestor(container); found {
					fm.traverseDescendants(pendingAncestor)
				}
			}
		}
		return

	case BlockStatusFinalized:
		containers := make([]*ExecutionResultContainer, 0)

		for container := range fm.resultsForest.IterateView(finalized.View) {
			if pendingAncestor, found := fm.oldestPendingAncestor(container); found {
				containers = append(containers, pendingAncestor)
			}
		}

		fm.traverseDescendants(containers...)
		return

	default:
		// the results forest only contains results for certified blocks. If the required block status
		// is certified, then finalization does not affect their processability.
		return
	}
}

// oldestPendingAncestor returns the oldest (lowest view) ancestor that is not yet enqueued
// This is used to get all potentially processable results for the fork containing the given container.
func (fm *ForestManager) oldestPendingAncestor(container *ExecutionResultContainer) (*ExecutionResultContainer, bool) {
	for {
		parentID, _ := container.Parent()
		parent, found := fm.resultsForest.GetContainer(parentID)
		if !found {
			return nil, false
		}

		if parent.IsEnqueued() {
			return container, true
		}

		container = parent
	}
}

// traverseDescendants performs a breadth-first traversal of descendants of `heads`, and adds
// processable containers to the work queue.
func (fm *ForestManager) traverseDescendants(heads ...*ExecutionResultContainer) {
	queue := list.New()
	for _, head := range heads {
		parentID, _ := head.Parent()
		parent, found := fm.resultsForest.GetContainer(parentID)
		if found && fm.enqueueIfProcessable(head, parent) {
			queue.PushBack(head)
		}
	}

	for queue.Len() > 0 {
		element := queue.Front()
		queue.Remove(element)
		parent := element.Value.(*ExecutionResultContainer)
		for child := range fm.resultsForest.IterateChildren(parent.resultID) {
			if fm.enqueueIfProcessable(child, parent) {
				queue.PushBack(child)
			}
		}
	}
}

// enqueueIfProcessable enqueues a container if it is processable, and returns true if it was enqueued.
func (fm *ForestManager) enqueueIfProcessable(container, parentContainer *ExecutionResultContainer) bool {
	if container.IsEnqueued() {
		return true // already enqueued, no need to check again
	}

	lg := fm.log.With().
		Hex("result_id", logging.ID(container.ResultID())).
		Uint64("view", container.BlockView()).
		Logger()

	if !fm.canProcessResult(container, parentContainer) {
		lg.Debug().Msg("container not ready for processing, skipping fork")
		return false
	}

	// Add task to work queue with priority based on view (lower view processed first)
	if container.SetEnqueued() {
		task := fm.ExecutePipelineTask(container)
		fm.workQueue.Push(task, container.BlockView())

		lg.Debug().Msg("added container to work queue")
	}

	return true
}

// canProcessResult determines if a result can be processed.
func (fm *ForestManager) canProcessResult(container, parentContainer *ExecutionResultContainer) bool {
	lg := fm.log.With().
		Hex("result_id", logging.ID(container.ResultID())).
		Logger()

	// 1. Container must be pending
	// Any other state means the container is either already running or abandoned.
	if container.Pipeline().GetState() != optimistic_sync.StatePending {
		lg.Debug().
			Str("state", container.Pipeline().GetState().String()).
			Msg("container not in pending state. skipping")
		return false
	}

	// 2. Parent must already be enqueued and not abandoned
	parentState := parentContainer.Pipeline().GetState()
	if parentContainer.IsEnqueued() && parentState != optimistic_sync.StateAbandoned {
		lg.Debug().
			Hex("parent_id", logging.ID(parentContainer.resultID)).
			Str("parent_state", parentState.String()).
			Msg("parent not ready for processing")
		return false
	}

	// 3. The result's block must have the required status

	// Execution Results that have passed verification and were sealed are guaranteed to be correct.
	// Hence, we can process them without further checks.
	blockStatus := container.BlockStatus()
	if blockStatus == BlockStatusSealed {
		return true
	}

	// We need to be careful with results _before_ they are sealed though, as they could still turn out to
	// be incorrect (or orphaned). For the interim period between a result being published and a result being
	// sealed (or slashed), we fall back on human-specified trust assumptions evaluated below.

	if !fm.hasRequiredBlockStatus(container) {
		lg.Debug().
			Uint64("block_height", container.BlockHeader().Height).
			Str("block_status", blockStatus.String()).
			Msg("block is not processable yet")
		return false
	}

	// TODO: can utilize logic from the API's criteria matching here

	// 4. A minimal number of Execution Nodes must have committed to the result:
	if uint(container.Size()) < fm.config.RequiredAgreeingExecutors {
		lg.Debug().
			Uint("receipt_count", container.Size()).
			Uint("required_count", fm.config.RequiredAgreeingExecutors).
			Msg("insufficient agreeing executors")
		return false
	}

	// 5. At least one of the configured (trusted) executors must have committed to the result
	if len(fm.config.RequiredExecutors) > 0 {
		hasRequiredExecutor := false
		for _, receipt := range container.Receipts() {
			if _, ok := fm.config.RequiredExecutors[receipt.ExecutorID]; ok {
				hasRequiredExecutor = true
				break // do not return here, so further rules can be added at the bottom
			}
		}
		if !hasRequiredExecutor {
			lg.Debug().Msg("container missing receipts from required executors")
			return false
		}
	}

	return true
}

// hasRequiredBlockStatus returns true if the container is processable based on the required block status.
func (fm *ForestManager) hasRequiredBlockStatus(container *ExecutionResultContainer) bool {
	switch fm.config.RequiredBlockStatus {
	case BlockStatusSealed:
		return container.BlockStatus() == BlockStatusSealed
	case BlockStatusFinalized:
		return container.BlockStatus() == BlockStatusSealed || container.BlockStatus() == BlockStatusFinalized
	default:
		// By design, the consensus follower only incorporates blocks once they are certified. All
		// execution results we work with in the context of the ResultsForest pertain to known blocks,
		// i.e. certified blocks which the consensus follower running inside this node incorporated
		// into the node's state. Hence, the executed block status of any result `ExecutionResultContainer`
		// here is at minimum `BlockStatusCertified`
		return true
	}
}

// ExecutePipelineTask returns a function that executes the pipeline for the given container.
func (fm *ForestManager) ExecutePipelineTask(container *ExecutionResultContainer) PipelineTask {
	return func(ctx context.Context) error {
		parentID, _ := container.Parent()
		parent, found := fm.resultsForest.getContainer(parentID)
		if !found {
			return fmt.Errorf("parent %s not found in forest for result %s", parentID, container.ResultID())
		}

		core := fm.coreFactory.NewCore(container.Result(), container.BlockHeader())
		err := container.Pipeline().Run(ctx, core, parent.Pipeline().GetState())
		if err != nil {
			return fmt.Errorf("failed to execute pipeline for result %s: %w", container.ResultID(), err)
		}

		// Note: the pipeline guarantees the latest persisted sealed result is updated sequentially
		// by requiring the following before persisting:
		// 1. the pipeline's result is sealed
		// 2. the pipeline's parent is completed (persisted)
		// Only after persisting is the latest persisted sealed result updated.

		// TODO: there is a missing liveness check. It is possible for state to be corrupted such
		// that the last sealed result does not descend from the latest persisted sealed result.
		// In this case, the pipeline's result may never be marked sealed, and the forest manager
		// will never progress.

		if err := fm.resultsForest.processCompleted(container.ResultID()); err != nil {
			return fmt.Errorf("failed to process completed pipeline for result %s: %w", container.ResultID(), err)
		}

		return nil
	}
}
