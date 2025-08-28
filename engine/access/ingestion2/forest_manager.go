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
	// forestManagerCheckInterval is the interval at which the forest manager checks for new
	// processable results. This is used to ensure that the forest manager makes progress even
	// if block finalization halts.
	//
	// This value should be greater than the block production rate.
	forestManagerCheckInterval = 10 * time.Second
)

// BlockStatus represents the state of a block in the forest.
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
type ForestManager struct {
	log           zerolog.Logger
	resultsForest *ResultsForest
	workQueue     *queue.PriorityMessageQueue[*ExecutionResultContainer]
	config        ForestManagerConfig
	notifier      engine.Notifier

	// maxQueueSize is the maximum size of the work queue
	maxQueueSize uint

	// sealedHeight is the latest sealed block height passed to OnBlockStatusUpdated
	sealedHeight uint64

	// finalizedHeight is the latest finalized block height passed to OnBlockStatusUpdated
	finalizedHeight uint64

	mu sync.RWMutex
}

// NewForestManager creates a new instance of ForestManager.
func NewForestManager(
	log zerolog.Logger,
	resultsForest *ResultsForest,
	workQueue *queue.PriorityMessageQueue[*ExecutionResultContainer],
	config ForestManagerConfig,
	notifier engine.Notifier,
	maxQueueSize uint,
) (*ForestManager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid forest manager config: %w", err)
	}

	return &ForestManager{
		log:           log.With().Str("component", "forest_manager").Logger(),
		resultsForest: resultsForest,
		workQueue:     workQueue,
		config:        config,
		notifier:      notifier,
		maxQueueSize:  maxQueueSize,
	}, nil
}

// WorkerLoop is a component.ComponentWorker that adds processable containers to the work queue.
//
// This is the main loop that runs in a single goroutine. It periodically checks for processable
// results and adds them to the work queue. It also listens for notifications from the OnBlockStatusUpdated
// and OnReceiptAdded handlers.
//
// CAUTION: not concurrency safe! Only one instance of this should be run at a time.
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
			// skip if we already checked recently
			// Note: this check will always be skipped on the happy path since block finalization
			// should happen more frequently than the check interval.
			if time.Since(lastChecked) < forestManagerCheckInterval {
				continue
			}
		}
		lastChecked = time.Now()

		// TODO: we may want to only run the full check after dropping below some threshold to reduce
		// the overhead of a full traversal when only a few results can be added to the queue.
		if uint(fm.workQueue.Len()) >= fm.maxQueueSize {
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
// This is used to notify the manager that a result and its descendants may be processable.
func (fm *ForestManager) OnReceiptAdded(container *ExecutionResultContainer) {
	if container.IsEnqueued() {
		return // already enqueued, adding this receipt will not change processability
	}

	if uint(fm.workQueue.Len()) >= fm.maxQueueSize {
		return
	}

	parentID, _ := container.Parent()
	parent, found := fm.resultsForest.GetContainer(parentID)
	if !found {
		return // parent does not exist in the forest yet, this result is not processable
	}

	// result previously was not enqueued and is now processable. this means that either this receipt
	// caused the result to become processable, or there is more room in the queue.
	// enqueue the result and check its descendants
	if fm.checkAndEnqueueContainer(container, parent) {
		fm.traverseDescendants(container)
	}
}

// OnBlockStatusUpdated is called when a new block is finalized.
// This is used to notify the forest that some results may now be processable.
func (fm *ForestManager) OnBlockStatusUpdated(finalized, sealed *flow.Header) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.sealedHeight = sealed.Height
	fm.finalizedHeight = finalized.Height

	fm.notifier.Notify()
}

// getLatestPersistedSealedResultContainer returns the latest persisted sealed result's container.
//
// No errors are expected during normal operation.
func (fm *ForestManager) getLatestPersistedSealedResultContainer() (*ExecutionResultContainer, error) {
	latestPersistedSealedResultID, _ := fm.resultsForest.latestPersistedSealedResult.Latest()
	latestPersistedSealedResultContainer, found := fm.resultsForest.GetContainer(latestPersistedSealedResultID)
	if !found {
		// the latest persisted sealed result must be in the forest, otherwise the forest is in an
		// inconsistent state.
		return nil, fmt.Errorf("latest persisted sealed result not found in forest")
	}
	return latestPersistedSealedResultContainer, nil
}

// traverseDescendants performs a breadth-first traversal of descendants of `head`, and adds
// processable containers to the work queue.
func (fm *ForestManager) traverseDescendants(head *ExecutionResultContainer) {
	queue := list.New()
	queue.PushBack(head)

	// Note: only descendents of head are added to the queue, not head itself.
	for queue.Len() > 0 && uint(fm.workQueue.Len()) < fm.maxQueueSize {
		element := queue.Front()
		queue.Remove(element)
		parent := element.Value.(*ExecutionResultContainer)
		fm.resultsForest.IterateChildren(parent.resultID, func(child *ExecutionResultContainer) bool {
			// Enqueue processable children for further iteration
			if fm.checkAndEnqueueContainer(child, parent) {
				queue.PushBack(child)
			}
			return uint(fm.workQueue.Len()) < fm.maxQueueSize
		})
	}
}

// checkAndEnqueueContainer checks if a container can be processed and enqueues it if it can.
// Returns true if the container was enqueued, false if it was not.
func (fm *ForestManager) checkAndEnqueueContainer(container, parentContainer *ExecutionResultContainer) bool {
	if container.IsEnqueued() {
		return true // already enqueued, no need to check again
	}

	lg := fm.log.With().
		Hex("result_id", logging.ID(container.resultID)).
		Uint64("view", container.blockHeader.View).
		Logger()

	if !fm.canProcessResult(container, parentContainer) {
		lg.Debug().Msg("container not ready for processing, skipping fork")
		return false
	}

	// Add to work queue with priority based on view (lower view processed first)
	fm.workQueue.Push(container, container.blockHeader.View)
	_ = container.SetEnqueued()

	lg.Debug().Msg("added container to work queue")

	return true
}

// canProcessResult determines if a result can be processed.
func (fm *ForestManager) canProcessResult(container, parentContainer *ExecutionResultContainer) bool {
	lg := fm.log.With().
		Hex("result_id", logging.ID(container.resultID)).
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
	blockStatus, processable := fm.checkBlockStatus(container.blockHeader.Height)
	if !processable {
		lg.Debug().
			Uint64("block_height", container.blockHeader.Height).
			Str("block_status", blockStatus.String()).
			Msg("block is not processable yet")
		return false
	}

	// Execution Results that have passed verification and were sealed are guaranteed to be correct.
	// Hence, we can process them without further checks.
	if blockStatus == BlockStatusSealed {
		return true
	}
	// We need to be careful with results _before_ they are sealed though, as they could still turn out to
	// be incorrect (or orphaned). For the interim period between a result being published and a result being
	// sealed (or slashed), we fall back on human-specified trust assumptions evaluated below.

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
		for _, receipt := range container.receipts {
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

// checkBlockStatus checks if a block is processable based on the required block status.
func (fm *ForestManager) checkBlockStatus(height uint64) (BlockStatus, bool) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if height > fm.finalizedHeight {
		return BlockStatusCertified, fm.config.RequiredBlockStatus == BlockStatusCertified
	}

	// Note: it is possible that there are results in the forest for unfinalized blocks that conflict
	// with finalized blocks. it is OK to accept them as processable here because
	// 1. finalized/sealed blocks are provided by the results forest, so the manager and forest are in sync.
	// 2. the results forest will abandon any results that conflict with finalized or sealed results.
	// 3. the pipeline logic will detect if the result is abandoned and exit with minimal overhead.
	// We are trading off a small amount of work by the workers for lower code complexity.

	if height > fm.sealedHeight {
		return BlockStatusFinalized, fm.config.RequiredBlockStatus == BlockStatusCertified || fm.config.RequiredBlockStatus == BlockStatusFinalized
	}

	return BlockStatusSealed, true
}
