package ingestion2

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/queue"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/rs/zerolog"
)

const (
	forestManagerCheckInterval = 10 * time.Second
)

// BlockStatus represents the state of a block in the system
type BlockStatus int

const (
	// BlockStatusSealed indicates the block has been sealed
	BlockStatusSealed BlockStatus = iota
	// BlockStatusFinalized indicates the block is finalized
	BlockStatusFinalized
	// BlockStatusCertified indicates the block is certified
	BlockStatusCertified
)

// String returns the string representation of the block state
func (bs BlockStatus) String() string {
	switch bs {
	case BlockStatusSealed:
		return "sealed"
	case BlockStatusFinalized:
		return "finalized"
	case BlockStatusCertified:
		return "certified"
	default:
		return "unknown"
	}
}

// ForestManagerConfig contains configuration for processing execution result containers
type ForestManagerConfig struct {
	// RequiredAgreeingExecutors is the minimum number of executors that must agree on the result
	RequiredAgreeingExecutors uint
	// RequiredExecutors is the set of executor IDs that must have provided receipts
	// At least one of these executors must have produced a receipt for the result to be processed
	RequiredExecutors map[flow.Identifier]struct{}
	// RequiredBlockStatus is the minimum block status required for processing
	RequiredBlockStatus BlockStatus
}

// DefaultForestManagerConfig returns the default configuration for the forest manager.
//
// Returns:
//   - ForestManagerConfig: the default configuration
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

// TODO: how to opimize this to avoid doing traversals?
//
// Reasons a result may not be processable:
// - does not have enough agreeing executors
// - does not have receipts from required executors
// - result's block is not in the required status
// - queue is full
//
// naive approach:
// any time a new result is added, a new block is finalized, or space becomes available in the queue,
// we traverse the forest and add processable results to the queue.
//
// better approach:
// * when a receipt is added, check if the result was not processable before, and is now processable
// * if so, enqueue the result and check its descendants
// * when a block is finalized, check if any results are now processable
// * when the queue was full and now has space, check if any results are now processable
// * we can merge the two full checks into one by adding a periodic check

// ForestManager iterates the ResultsForest starting from the latest persisted sealed result
// to find processable ancestors that are not running yet. It uses a breadth-first traversal
// approach to ensure ancestors are processed before descendants.
// The forest manager is intended to be run in a single goroutine as a component.ComponentWorker
// via the WorkerLoop method.
//
// Concurrency-safety:
//   - Not safe for concurrent access. Must be run in a single goroutine.
type ForestManager struct {
	log           zerolog.Logger
	resultsForest *ResultsForest
	workQueue     *queue.PriorityMessageQueue[*ExecutionResultContainer]
	config        ForestManagerConfig
	notifier      engine.Notifier

	// maxQueueSize is the maximum size of the work queue
	maxQueueSize uint

	// queueFillThreshold is the threshold below which new results are added to the work queue
	// this is used to avoid performing tree traversals when there is only a small amount of
	// space left in the queue.
	queueFillThreshold uint

	// sealedHeight is the latest sealed block height passed to OnBlockStatusUpdated
	sealedHeight uint64

	// finalizedHeight is the latest finalized block height passed to OnBlockStatusUpdated
	finalizedHeight uint64

	mu sync.RWMutex
}

// NewForestManager creates a new instance of ForestManager.
//
// Parameters:
//   - log: logger instance
//   - resultsForest: the results forest to manage
//   - workQueue: the priority queue for work items
//   - config: configuration for processing requirements
//   - notifier: notifier for notifications
//   - queueFillThreshold: the threshold at which the work queue is considered full
//   - maxQueueSize: the maximum size of the work queue
//
// Returns:
//   - *ForestManager: the newly created forest manager
func NewForestManager(
	log zerolog.Logger,
	resultsForest *ResultsForest,
	workQueue *queue.PriorityMessageQueue[*ExecutionResultContainer],
	config ForestManagerConfig,
	notifier engine.Notifier,
	queueFillThreshold uint,
	maxQueueSize uint,
) (*ForestManager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid forest manager config: %w", err)
	}

	return &ForestManager{
		log:                log.With().Str("component", "forest_manager").Logger(),
		resultsForest:      resultsForest,
		workQueue:          workQueue,
		config:             config,
		notifier:           notifier,
		queueFillThreshold: queueFillThreshold,
		maxQueueSize:       maxQueueSize,
	}, nil
}

// TODO: remove me and/or format this into a better comment
//
// General rationale for this approach:
// * we will most likely NOT be able to start processing results as soon as they are added to the forest
//   since we require a certain number of receipts and the block status may not be set to certified
// * I want to avoid maintaining a second queue for results that are not yet processable

// WorkerLoop is a component.ComponentWorker that adds processable containers to the work queue.
//
// This is the main loop that runs in a single goroutine. It periodically checks for processable
// results and adds them to the work queue. It also listens for notifications from the OnBlockStatusUpdated
// handler.
//
// Parameters:
//   - ctx: the context for the operation
//   - ready: a function to call when the goroutine is ready
//
// Concurrency-safety:
//   - Not safe for concurrent access. Must be run in a single goroutine.
func (fm *ForestManager) WorkerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	// periodic check to ensure we continue processing results even if no blocks are finalized
	periodicCheck := time.NewTicker(forestManagerCheckInterval)
	defer periodicCheck.Stop()

	var lastChecked time.Time
	notifierChan := fm.notifier.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-notifierChan:
		case <-periodicCheck.C:
			if time.Since(lastChecked) < forestManagerCheckInterval {
				continue // skip if we already checked recently
			}
		}
		lastChecked = time.Now()

		if uint(fm.workQueue.Len()) > fm.queueFillThreshold {
			continue
		}

		latestPersistedSealedResultContainer, err := fm.getLatestPersistedSealedResultContainer()
		if err != nil {
			ctx.Throw(err)
			return
		}

		fm.traverseDescendants(latestPersistedSealedResultContainer)
	}
}

// OnReceiptAdded is called when a new receipt is added to the forest.
// This is used to notify the forest that a result and its descendants may be processable.
//
// Parameters:
//   - container: the container that the receipt was added to
//
// Returns:
//   - error: any error that occurred during the operation
//
// No errors are expected during normal operation.
func (fm *ForestManager) OnReceiptAdded(container *ExecutionResultContainer) error {
	if container.IsEnqueued() {
		return nil // already enqueued, adding this receipt will not change processability
	}

	if uint(fm.workQueue.Len()) > fm.queueFillThreshold {
		return nil
	}

	parent, found := fm.resultsForest.GetContainer(container.result.PreviousResultID)
	if !found {
		return fmt.Errorf("parent %s for result %s not found in forest", container.result.PreviousResultID, container.resultID)
	}

	processable := fm.canProcessContainer(container, parent)
	if !processable {
		return nil
	}

	// result previously was not enqueued and is now processable. this means that either this receipt
	// caused the result to become processable, or there is more room in the queue.
	// enqueue the result and check its descendants
	fm.workQueue.Push(container, container.blockHeader.View)
	_ = container.SetEnqueued()

	fm.traverseDescendants(container)
	return nil
}

// OnBlockStatusUpdated is called when a new block is finalized.
// This is used to notify the forest that some results may now be processable.
//
// Parameters:
//   - finalized: the finalized block header
//   - sealed: the sealed block header
//
// Concurrency-safety:
//   - Safe for concurrent access.
func (fm *ForestManager) OnBlockStatusUpdated(finalized, sealed *flow.Header) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.sealedHeight = sealed.Height
	fm.finalizedHeight = finalized.Height

	fm.notifier.Notify()
}

// getLatestPersistedSealedResultContainer returns the latest persisted sealed result's container.
//
// Returns:
//   - *ExecutionResultContainer: the latest persisted sealed result container
//   - error: any error that occurred during the operation
//
// No errors are expected during normal operation.
func (fm *ForestManager) getLatestPersistedSealedResultContainer() (*ExecutionResultContainer, error) {
	latestPersistedSealedResultID, _ := fm.resultsForest.latestPersistedSealedResult.Latest()
	latestPersistedSealedResultContainer, found := fm.resultsForest.GetContainer(latestPersistedSealedResultID)
	if !found {
		return nil, fmt.Errorf("latest persisted sealed result not found in forest")
	}
	return latestPersistedSealedResultContainer, nil
}

// traverseDescendants performs a breadth-first traversal starting from the startContainer and adds
// processable containers to the work queue.
//
// Parameters:
//   - startContainer: the container to start traversal from
func (fm *ForestManager) traverseDescendants(startContainer *ExecutionResultContainer) {
	// queue used for BFS traversal
	// Note: we intentionally do not check or enqueue the startContainer
	// when traversing from the latest persisted sealed result, we must skip it since it is by
	// definition completed and no further processing is needed.
	queue := list.New()
	queue.PushBack(startContainer)

	for queue.Len() > 0 && uint(fm.workQueue.Len()) < fm.maxQueueSize {
		element := queue.Front()
		queue.Remove(element)
		parent := element.Value.(*ExecutionResultContainer)
		fm.resultsForest.IterateChildren(parent.resultID, func(child *ExecutionResultContainer) bool {
			processable := fm.checkAndEnqueueContainer(child, parent)

			// Enqueue processable children for further iteration
			if processable {
				queue.PushBack(child)
			}

			return true
		})
	}
}

// checkAndEnqueueContainer checks if a container can be processed and enqueues it if it can.
//
// Parameters:
//   - container: the container to check
//
// Returns:
//   - bool: true if the container is processable, false otherwise
func (fm *ForestManager) checkAndEnqueueContainer(container, parentContainer *ExecutionResultContainer) bool {
	if container.IsEnqueued() {
		return true // already enqueued, no need to check again
	}

	lg := fm.log.With().
		Hex("result_id", logging.ID(container.resultID)).
		Uint64("view", container.blockHeader.View).
		Logger()

	processable := fm.canProcessContainer(container, parentContainer)
	if !processable {
		lg.Debug().Msg("container not ready for processing, skipping fork")
		return false
	}

	// Add to work queue with priority based on level (lower level = higher priority)
	fm.workQueue.Push(container, container.blockHeader.View)
	_ = container.SetEnqueued()

	lg.Debug().Msg("added container to work queue")

	return true
}

// canProcessContainer determines if a container can be processed.
//
// Parameters:
//   - container: the container to check
//   - parentContainer: the parent container of the container to check
//
// Returns:
//   - bool: true if the container can be processed, false otherwise
func (fm *ForestManager) canProcessContainer(container, parentContainer *ExecutionResultContainer) bool {
	lg := fm.log.With().
		Hex("result_id", logging.ID(container.resultID)).
		Logger()

	// 1. Container must be pending
	if container.Pipeline().GetState() != optimistic_sync.StatePending {
		lg.Debug().
			Str("state", container.Pipeline().GetState().String()).
			Msg("container not in pending state. skipping")
		return false
	}

	// 2. Parent must not be in pending or abandoned state
	parentState := parentContainer.Pipeline().GetState()
	if parentState == optimistic_sync.StatePending || parentState == optimistic_sync.StateAbandoned {
		lg.Debug().
			Hex("parent_id", logging.ID(parentContainer.resultID)).
			Str("parent_state", parentState.String()).
			Msg("parent not ready for processing")
		return false
	}

	// 3. The result's block must have the required status
	blockStatus, processable := fm.checkBlockStatus(container.blockHeader.Height)
	if !processable {
		lg.Debug().
			Uint64("block_height", container.blockHeader.Height).
			Str("block_status", blockStatus.String()).
			Msg("block is not processable yet")
		return false
	}

	// we do not need to check executors for sealed blocks
	if blockStatus == BlockStatusSealed {
		return true
	}

	// 4. There must be enough agreeing executors
	if uint(container.Size()) < fm.config.RequiredAgreeingExecutors {
		lg.Debug().
			Uint("receipt_count", container.Size()).
			Uint("required_count", fm.config.RequiredAgreeingExecutors).
			Msg("insufficient agreeing executors")
		return false
	}

	// 5. The result must have at least one of the configured required executors
	if len(fm.config.RequiredExecutors) > 0 {
		hasRequiredExecutor := false
		for _, receipt := range container.receipts {
			if _, ok := fm.config.RequiredExecutors[receipt.ExecutorID]; ok {
				hasRequiredExecutor = true
				break
			}
		}
		if !hasRequiredExecutor {
			lg.Debug().Msg("container missing receipts from required executors")
			return false
		}
	}

	return true
}

// checkBlockStatus checks if a block is processable based on the block status and the required block status.
//
// Parameters:
//   - height: the height to check
//
// Returns:
//   - BlockStatus: the block status
//   - bool: true if the block meets the required block status, false otherwise
func (fm *ForestManager) checkBlockStatus(height uint64) (BlockStatus, bool) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if height > fm.finalizedHeight {
		return BlockStatusCertified, fm.config.RequiredBlockStatus == BlockStatusCertified
	}

	if height > fm.sealedHeight {
		return BlockStatusFinalized, fm.config.RequiredBlockStatus == BlockStatusCertified || fm.config.RequiredBlockStatus == BlockStatusFinalized
	}

	return BlockStatusSealed, true
}
