package pipeline

import (
	"context"
	"fmt"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

// Pipeline2 represents a pipelined state machine for a processing single ExecutionResult.
//
// The state machine is initialized in the Pending state.
//
//     Trigger condition: (1)▷┬┄┄┄┄┄┐                           ┌┄┄┐
//             parent is      ┊    ╭∇─────────────────────────╮ ┊ ╭∇──────────────────────────╮ ┌?▻● Trigger condition (3):
//           Downloading      ┊    │Task Downloading (once):  │ ┊ │Task Indexing (once):      │ ┊  ┊ sealed & parent completed
//           or Indexing      ┊    │1. download               │ ┊ │1. build index from downl. │ ┊ ╭∇───────────────────────────╮
//     or WaitingPersist      ┊    │3. add Task Indexing (2)▷┄│┄┘ │3. check trigger (3)  ▷?┄┄┄│┄┘ │Task Persisting (once):     │
//           or Complete      ┊    │2. CAS state transition   │   │2. CAS state transition    │   │- database write            │
//                            ┊    │   downloading → indexing │   │   indexing →WaitingPersist│   │- CAS state transition      │
//                            ┊    │               ▷┄┄┄┄┄┄┐   │   │            ▷┄┄┄┄┐         │   │  WaitingPersist → Complete │
//     ┏━━━━━━━━━━━━━━━━━━━┓  ┊  ┏━━━━━━━━━━━━━━━━┓       ┊  ┏━━━━━━━━━━━━━━━━━━━┓  ┊   ┏━━━━━━━━━━━━━━━━━━┓   ┏━━━━━━━━━━━━━━━━┓
//     ┃       state       ┃  ┊  ┃     state      ┃       ┊  ┃      state        ┃  ┊   ┃       state      ┃   ┃      state     ┃
//     ┃      Pending      ┃──●─►┃  Downloading   ┃───────●─►┃     Indexing      ┃──●──►┃  WaitingPersist  ┃──►┃     Complete   ┃
//     ┗━━━━━━━━━━━━┯━━━━━━┛     ┗━━━━━━━┯━━━━━━━━┛          ┗━━━━━━━━━━━━┯━━━━━━┛      ┗━━━━━━━┯━━━━━━━━━━┛   ┗━━━━━━━━━━━━━━━━┛
//                  ┃              ╰─────┃────────────────────╯   ╰───────┃───────────────────╯ ┃ ╰────────────────────────────╯
//                  ┃                    ┃                                ┃                     ┃
//                  ┃                    ┃         ┏━━━━━━━━━━━━━━━┓      ┃                     ┃
//                  ┗━━━━━━━━━━━━━━━━━━━━┻━━━━━━━━━▷   Abandoned   ◁━━━━━━┻━━━━━━━━━━━━━━━━━━━━━┛
//                                                 ┗━━━━━━━━━━━━━━━┛
//
// Trigger condition (1):
// • parent must be Downloading or Indexing or WaitingPersist or Complete
//
// This can be implement as listening to the `OnParentStateUpdated` event:
// (assuming no state change was missed, when pipeline was added to forest
//
// Trigger condition (2):
// • The download is completed.
// As we have a single routine executing the downloading task, this routine should always be able to perform the state transition,
// barring the processing being abandoned.
//
// Trigger condition (3):
// • pipeline must represent sealed result
// • and parent's state must be completed
// • This can be implement as listening to the following events:
//    - OnParentStateUpdated
//    - SetSealed
//   each must have been called once, the latter (atomically) can trigger the task `Persisting`
//
// CAUTION:
//  • All tasks are executed at most once.
//  • Trigger conditions being satisfied must not be missed.
//
// We must enforce that _all_ parent updated notifications are delivered this required a "and-and-subscribe + catchup trigger"

// The state machine is designed to be run in a single goroutine. The Run method must only be called once.
type Pipeline2 struct {
	log                  zerolog.Logger
	stateConsumer        optimistic_sync.PipelineStateConsumer
	stateChangedNotifier engine.Notifier
	core                 optimistic_sync.Core

	downloadingWorkerPool *workerpool.WorkerPool
	indexingWorkerPool    *workerpool.WorkerPool
	persistingWorkerPool  *workerpool.WorkerPool

	worker *worker

	// The following fields are accessed externally. they are stored using atomics to avoid
	// blocking the caller.
	state            *atomic.Int32
	parentStateCache *atomic.Int32
	isSealed         *atomic.Bool
	isAbandoned      *atomic.Bool
	isIndexed        *atomic.Bool
}

var _ optimistic_sync.Pipeline = (*Pipeline2)(nil)

// NewPipeline2 creates a new processing pipeline.
// The pipeline is initialized in the Pending state.
func NewPipeline2(
	log zerolog.Logger,
	downloadingWorkerPool *workerpool.WorkerPool,
	indexingWorkerPool *workerpool.WorkerPool,
	persistingWorkerPool *workerpool.WorkerPool,
	executionResult *flow.ExecutionResult,
	stateReceiver optimistic_sync.PipelineStateConsumer,
) *Pipeline2 {
	log = log.With().
		Str("component", "pipeline").
		Str("execution_result_id", executionResult.ExecutionDataID.String()).
		Str("block_id", executionResult.BlockID.String()).
		Logger()

	return &Pipeline2{
		log:                   log,
		stateConsumer:         stateReceiver,
		worker:                newWorker(),
		state:                 atomic.NewInt32(int32(optimistic_sync.StatePending)),
		parentStateCache:      atomic.NewInt32(int32(optimistic_sync.StatePending)),
		isSealed:              atomic.NewBool(false),
		isAbandoned:           atomic.NewBool(false),
		isIndexed:             atomic.NewBool(false),
		stateChangedNotifier:  engine.NewNotifier(),
		downloadingWorkerPool: downloadingWorkerPool,
		indexingWorkerPool:    indexingWorkerPool,
		persistingWorkerPool:  persistingWorkerPool,
	}
}

// Run starts the pipeline processing and blocks until completion or context cancellation.
// CAUTION: not concurrency safe! Run must only be called once.
//
// Expected error returns during normal operations:
//   - context.Canceled: when the context is canceled
func (p *Pipeline2) Run(ctx context.Context, core optimistic_sync.Core, parentState optimistic_sync.State) error {
	panic("deprecated to be removed")
}

// loop implements the main event loop for state machine. It reacts on different events and performs operations upon
// entering or leaving some state.
// loop will perform a blocking operation until one of next things happens, whatever happens first:
// 1. parent context signals that it is no longer valid.
// 2. the worker thread has received an error. It's not safe to continue execution anymore, so this error needs to be propagated
// to the caller.
//  3. Pipeline2 has successfully entered terminal state.
//     Pipeline2 won't and shouldn't perform any state transitions after returning from this function.
//
// Expected error returns during normal operations:
//   - context.Canceled: when the context is canceled
func (p *Pipeline2) loop(ctx context.Context) error {
	// try to start processing in case we are able to.
	p.stateChangedNotifier.Notify()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-p.worker.ErrChan():
			return err
		case <-p.stateChangedNotifier.Channel():
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// if parent got abandoned no point to continue, and we just go to the abandoned state and perform cleanup logic.
			if p.checkAbandoned() {
				if err := p.transitionTo(optimistic_sync.StateAbandoned); err != nil {
					return fmt.Errorf("could not transition to abandoned state: %w", err)
				}
			}

			currentState := p.GetState()
			switch currentState {
			case optimistic_sync.StatePending:
				if err := p.onStartProcessing(); err != nil {
					return fmt.Errorf("could not process pending state: %w", err)
				}
			case optimistic_sync.StateProcessing:
				if err := p.onProcessing(); err != nil {
					return fmt.Errorf("could not process processing state: %w", err)
				}
			case optimistic_sync.StateWaitingPersist:
				if err := p.onPersistChanges(); err != nil {
					return fmt.Errorf("could not process waiting persist state: %w", err)
				}
			case optimistic_sync.StateAbandoned:
				if err := p.core.Abandon(); err != nil {
					return fmt.Errorf("could not process abandonded state: %w", err)
				}
				return nil
			case optimistic_sync.StateComplete:
				return nil // terminate
			default:
				return fmt.Errorf("invalid pipeline state: %s", currentState)
			}
		}
	}
}

// onStartProcessing performs the initial state transitions depending on the parent state:
// - Pending -> Processing
// - Pending -> Abandoned
// No errors are expected during normal operations.
func (p *Pipeline2) onStartProcessing() error {
	switch p.parentState() {
	case optimistic_sync.StateProcessing, optimistic_sync.StateWaitingPersist, optimistic_sync.StateComplete:
		err := p.transitionTo(optimistic_sync.StateProcessing)
		if err != nil {
			return err
		}
		p.worker.Submit(p.performDownload)
	case optimistic_sync.StatePending:
		return nil
	case optimistic_sync.StateAbandoned:
		return p.transitionTo(optimistic_sync.StateAbandoned)
	default:
		// it's unexpected for the parent to be in any other state. this most likely indicates there's a bug
		return fmt.Errorf("unexpected parent state: %s", p.parentState())
	}
	return nil
}

// onProcessing performs the state transitions when the pipeline is in the Processing state.
// When data has been successfully indexed, we can transition to StateWaitingPersist.
// No errors are expected during normal operations.
func (p *Pipeline2) onProcessing() error {
	if p.isIndexed.Load() {
		return p.transitionTo(optimistic_sync.StateWaitingPersist)
	}
	return nil
}

// onPersistChanges performs the state transitions when the pipeline is in the WaitingPersist state.
// When the execution result has been sealed and the parent has already transitioned to StateComplete then
// we can persist the data and transition to StateComplete.
// No errors are expected during normal operations.
func (p *Pipeline2) onPersistChanges() error {
	if p.isSealed.Load() && p.parentState() == optimistic_sync.StateComplete {
		if err := p.core.Persist(); err != nil {
			return fmt.Errorf("could not persist pending changes: %w", err)
		}
		return p.transitionTo(optimistic_sync.StateComplete)
	} else {
		return nil
	}
}

// checkAbandoned returns true if the pipeline or its parent are abandoned.
func (p *Pipeline2) checkAbandoned() bool {
	if p.isAbandoned.Load() {
		return true
	}
	if p.parentState() == optimistic_sync.StateAbandoned {
		return true
	}
	return p.GetState() == optimistic_sync.StateAbandoned
}

// GetState returns the current state of the pipeline.
func (p *Pipeline2) GetState() optimistic_sync.State {
	return optimistic_sync.State(p.state.Load())
}

// parentState returns the last cached parent state of the pipeline.
func (p *Pipeline2) parentState() optimistic_sync.State {
	return optimistic_sync.State(p.parentStateCache.Load())
}

// SetSealed marks the execution result as sealed.
// This will cause the pipeline to eventually transition to the StateComplete state when the parent finishes processing.
func (p *Pipeline2) SetSealed() {
	// Note: do not use a mutex here to avoid blocking the results forest.
	if p.isSealed.CompareAndSwap(false, true) {
		p.stateChangedNotifier.Notify()
	}
}

// OnParentStateUpdated updates the pipeline's state based on the provided parent state.
// If the parent state has changed, it will notify the state consumer and trigger a state change notification.
func (p *Pipeline2) OnParentStateUpdated(parentState optimistic_sync.State) {
	oldState := p.parentStateCache.Load()
	if p.parentStateCache.CompareAndSwap(oldState, int32(parentState)) {
		p.stateChangedNotifier.Notify()
	}
}

// Abandon marks the pipeline as abandoned
// This will cause the pipeline to eventually transition to the Abandoned state and halt processing
func (p *Pipeline2) Abandon() {
	if p.isAbandoned.CompareAndSwap(false, true) {
		p.stateChangedNotifier.Notify()
	}
}

// performDownload performs the processing step of the pipeline by downloading and indexing data.
// It uses an atomic flag to indicate whether the operation has been completed successfully which
// informs the state machine that eventually it can transition to the next state.
// Expected error returns during normal operations:
//   - context.Canceled: when the context is canceled
func (p *Pipeline2) performDownload(ctx context.Context) error {
	if err := p.core.Download(ctx); err != nil {
		return fmt.Errorf("could not perform download: %w", err)
	}
	if err := p.core.Index(); err != nil {
		return fmt.Errorf("could not perform indexing: %w", err)
	}
	if p.isIndexed.CompareAndSwap(false, true) {
		p.stateChangedNotifier.Notify()
	}
	return nil
}

// transitionTo transitions the pipeline to the given state and broadcasts
// the state change to children pipelines.
//
// Expected error returns during normal operations:
//   - ErrInvalidTransition: when the transition is invalid
func (p *Pipeline2) transitionTo(newState optimistic_sync.State) error {
	hasChange, err := p.setState(newState)
	if err != nil {
		return err
	}

	if hasChange {
		// send notification for all state changes. we require that implementations of [PipelineStateConsumer]
		// are non-blocking and consume the state updates without noteworthy delay.
		p.stateConsumer.OnStateUpdated(newState)
		p.stateChangedNotifier.Notify()
	}

	return nil
}

// setState sets the state of the pipeline and logs the transition.
// Returns true if the state was changed, false otherwise.
//
// Expected error returns during normal operations:
//   - ErrInvalidTransition: when the state transition is invalid
func (p *Pipeline2) setState(newState optimistic_sync.State) (bool, error) {
	currentState := p.GetState()

	// transitioning to the same state is a no-op
	if currentState == newState {
		return false, nil
	}

	if err := p.validateTransition(currentState, newState); err != nil {
		return false, fmt.Errorf("failed to transition from %s to %s: %w", currentState, newState, err)
	}

	if !p.state.CompareAndSwap(int32(currentState), int32(newState)) {
		// Note: this should never happen since state is only updated within the Run goroutine.
		return false, fmt.Errorf("failed to transition from %s to %s", currentState, newState)
	}

	p.log.Debug().
		Str("old_state", currentState.String()).
		Str("new_state", newState.String()).
		Msg("pipeline state transition")

	return true, nil
}

// validateTransition validates the transition from the current state to the new state.
//
// Expected error returns during normal operations:
//   - ErrInvalidTransition: when the transition is invalid
func (p *Pipeline2) validateTransition(currentState optimistic_sync.State, newState optimistic_sync.State) error {
	switch newState {
	case optimistic_sync.StateProcessing:
		if currentState == optimistic_sync.StatePending {
			return nil
		}
	case optimistic_sync.StateWaitingPersist:
		if currentState == optimistic_sync.StateProcessing {
			return nil
		}
	case optimistic_sync.StateComplete:
		if currentState == optimistic_sync.StateWaitingPersist {
			return nil
		}
	case optimistic_sync.StateAbandoned:
		// Note: it does not make sense to transition to abandoned from persisting or completed since to be in either state:
		// 1. the parent must be completed
		// 2. the pipeline's result must be sealed
		// At that point, there are no conditions that would cause the pipeline be abandoned
		switch currentState {
		case optimistic_sync.StatePending, optimistic_sync.StateProcessing, optimistic_sync.StateWaitingPersist:
			return nil
		}

	default:
		return fmt.Errorf("invalid transition to state: %s", newState)
	}

	return ErrInvalidTransition
}
