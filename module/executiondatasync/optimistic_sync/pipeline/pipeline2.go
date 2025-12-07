package pipeline

import (
	"context"
	"fmt"
	"sync"

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
//	Trigger condition: (1)▷┬┄┄┄┄┄┐                           ┌┄┄┐
//	        parent is      ┊    ╭∇─────────────────────────╮ ┊ ╭∇──────────────────────────╮ ┌?▻● Trigger condition (3):
//	      Downloading      ┊    │Task Downloading (once):  │ ┊ │Task Indexing (once):      │ ┊  ┊ sealed & parent completed
//	      or Indexing      ┊    │1. download               │ ┊ │1. build index from downl. │ ┊ ╭∇───────────────────────────╮
//	or WaitingPersist      ┊    │3. add Task Indexing (2)▷┄│┄┘ │3. check trigger (3)  ▷?┄┄┄│┄┘ │Task Persisting (once):     │
//	      or Complete      ┊    │2. CAS state transition   │   │2. CAS state transition    │   │- database write            │
//	                       ┊    │   downloading → indexing │   │   indexing →WaitingPersist│   │- CAS state transition      │
//	                       ┊    │               ▷┄┄┄┄┄┄┐   │   │            ▷┄┄┄┄┐         │   │  WaitingPersist → Complete │
//	┏━━━━━━━━━━━━━━━━━━━┓  ┊  ┏━━━━━━━━━━━━━━━━┓       ┊  ┏━━━━━━━━━━━━━━━━━━━┓  ┊   ┏━━━━━━━━━━━━━━━━━━┓   ┏━━━━━━━━━━━━━━━━┓
//	┃       state       ┃  ┊  ┃     state      ┃       ┊  ┃      state        ┃  ┊   ┃       state      ┃   ┃      state     ┃
//	┃      Pending      ┃──●─►┃  Downloading   ┃───────●─►┃     Indexing      ┃──●──►┃  WaitingPersist  ┃──►┃     Complete   ┃
//	┗━━━━━━━━━━━━┯━━━━━━┛     ┗━━━━━━━┯━━━━━━━━┛          ┗━━━━━━━━━━━━┯━━━━━━┛      ┗━━━━━━━┯━━━━━━━━━━┛   ┗━━━━━━━━━━━━━━━━┛
//	             ┃              ╰─────┃────────────────────╯   ╰───────┃───────────────────╯ ┃ ╰────────────────────────────╯
//	             ┃                    ┃                                ┃                     ┃
//	             ┃                    ┃         ┏━━━━━━━━━━━━━━━┓      ┃                     ┃
//	             ┗━━━━━━━━━━━━━━━━━━━━┻━━━━━━━━━▷   Abandoned   ◁━━━━━━┻━━━━━━━━━━━━━━━━━━━━━┛
//	                                            ┗━━━━━━━━━━━━━━━┛
//
// Trigger condition (1):
// ▷ Parent must be Downloading or Indexing or WaitingPersist or Complete.
// This can be implement as listening to the `OnParentStateUpdated` event
// (assuming no state change was missed, when pipeline was added to forest)
//
// Trigger condition (2):
// ▷ The download is completed.
// As we have a single routine executing the downloading task, this routine should always
// be able to perform the state transition, barring the processing being abandoned.
//
// Trigger condition (3):
// ▷ Pipeline must represent a sealed result _and_ parent's state must be Complete.
// This can be implement by listening to the events `OnParentStateUpdated` and `SetSealed`.
// Each must have been called once, the latter event (atomically) triggers the task `Persisting`.
//
// REQUIREMENTS:
// (I) All tasks are executed at most once.
// (II) Trigger conditions being satisfied must not be missed.
//
// The only exceptions to rule (II) are:
//   - Triggers can be missed if the pipeline reaches the Abandoned state. In this case, no further processing is required, so
//     so interim triggers for processing work can be ignored.
//   - The trigger to abandon processing can be missed (with low probability). In this case, we will do extra work but eventually
//     the result will be garbage collected. Doing more work optimistically to improve latency than minimally necessary is acceptable.
//
// In a concurrent environment, there might be a "blind period" between a `Pipeline2` instance being created and it being subscribed
// to the event sources emitting `OnParentStateUpdated` and `SetSealed` notifications. Notifications concurrently emitted during such
// "blind period" might violate requirement (II). Design patters to avoid a "blind period" for the higher-level business logic are:
//   - Atomic instantiation-and-subscription:
//     The `Pipeline2` instance is created and subscribed to the event sources within a single atomic operation. The result's own sealing
//     status as well as the status of the parent result do not change during this atomic operation.
//   - Instantiate-and-subscribe + Catchup:
//     After successful instantiating a Pipeline2 object and subscribing it to `OnParentStateUpdated` plus `SetSealed`, the higher-level
//     business logic does a second iteration over the result's sealing status and the parent result's status. It reads the status from
//     the _sources directly_, not relying on notifications. This newest information about the most up-to-date status is then forwarded
//     to the `Pipeline2` instance. When implementing this approach, we must follow an information-driven design:
//     https://www.notion.so/flowfoundation/Reactive-State-Management-in-Distributed-Systems-A-Guide-to-Eventually-Consistent-Designs-22d1aee1232480b2ad05e95a7c48a32d
//     Formally, you might think of the events as sets (of information), where the set relations defines a partial order.
//     Pending ⊂ Downloading ⊂ Indexing ⊂ WaitingPersist ⊂ Complete
//     (Pending ∪ Downloading ∪ Indexing ∪ WaitingPersist) ⊂ Abandoned
//     This implies:
//     (a) Pipeline2 must be idempotent with respect to `OnParentStateUpdated` and `SetSealed` invocations.
//     (b) Pipeline2 must recognize old information as such, which is already reflected in its internal state, and ignore it. For example,
//     the Pipeline bing in the state "Indexing" should be understood as "we should download the data and index it once we have it".
//     Then being informed (old information) that we have progressed to a point where we should be downloading the data, should result
//     in a no-op because we already know that (and more, namely that we should be indexing the data too once we have downloaded it).
//     (c) Pipeline2 must recognize notifications as such, that deliver newer information but skipping some intermediate state transitions.
//     Pipeline2 can't just silently discard that information. Either, we return a sentinel error, signalling to the caller that they need
//     to re-deliver missing interim notifications. Or Pipeline2 could advance the state internally to be up-to-date with the newest
//     information it received. This can happen as a race condition in a concurrent environment: for example, assume that Pipeline2 is in
//     the "Pending" state. It missed the notification that its parent has progressed to "Indexing". While the higher-level business logic
//     is just about the deliver the newer information, the parent transitions to Complete and emits a notification that arrives first.
//
// TODO: check if the following statement still applies after the refactoring:
// Pipeline2 The state machine is designed to be run in a single goroutine. The Run method must only be called once.
type Pipeline2 struct {
	log                  zerolog.Logger
	stateConsumer        optimistic_sync.PipelineStateConsumer
	stateChangedNotifier engine.Notifier
	core                 optimistic_sync.Core

	downloadingWorkerPool *workerpool.WorkerPool
	indexingWorkerPool    *workerpool.WorkerPool
	persistingWorkerPool  *workerpool.WorkerPool

	// TODO: This can likely be removed
	mu sync.Mutex

	// The following fields are accessed externally. they are stored using atomics to avoid
	// blocking the caller.
	state              *optimistic_sync.State2Tracker // current state of the pipeline
	parentStateTracker *optimistic_sync.State2Tracker // latest known state of the parent result; eventually consistent: might lag behind actual parent state!

	// TODO: remove the following fields, if they are no longer used (?)

	worker *worker // deprecated: to be removed

	isSealed    *atomic.Bool // deprecated: to be removed
	isAbandoned *atomic.Bool // deprecated: to be removed
	isIndexed   *atomic.Bool // deprecated: to be removed
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
) (*Pipeline2, error) {
	log = log.With().
		Str("component", "pipeline").
		Str("execution_result_id", executionResult.ExecutionDataID.String()).
		Str("block_id", executionResult.BlockID.String()).
		Logger()

	myState, err := optimistic_sync.NewState2Tracker(optimistic_sync.State2Pending)
	if err != nil {
		return nil, fmt.Errorf("could not create pipeline state tracker: %w", err)
	}
	parentState, err := optimistic_sync.NewState2Tracker(optimistic_sync.State2Pending)
	if err != nil {
		return nil, fmt.Errorf("could not create pipeline state tracker: %w", err)
	}

	return &Pipeline2{
		log:                   log,
		stateConsumer:         stateReceiver,
		worker:                newWorker(),
		state:                 myState,
		parentStateTracker:    parentState,
		isSealed:              atomic.NewBool(false),
		isAbandoned:           atomic.NewBool(false),
		isIndexed:             atomic.NewBool(false),
		stateChangedNotifier:  engine.NewNotifier(),
		downloadingWorkerPool: downloadingWorkerPool,
		indexingWorkerPool:    indexingWorkerPool,
		persistingWorkerPool:  persistingWorkerPool,
	}, nil
}

func (p *Pipeline2) Run(ctx context.Context, core optimistic_sync.Core, parentState optimistic_sync.State) error {
	panic("Run is deprecated, implementation now utilizes `State2Tracker` for atomicity and workerpools for any work with non-vanishing runtime")
}

// onStartProcessing performs the initial state transitions depending on the parent state:
// - Pending -> Processing
// - Pending -> Abandoned
// No error returns are expected during normal operations.
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
// No error returns are expected during normal operations.
func (p *Pipeline2) onProcessing() error {
	if p.isIndexed.Load() {
		return p.transitionTo(optimistic_sync.StateWaitingPersist)
	}
	return nil
}

// onPersistChanges performs the state transitions when the pipeline is in the WaitingPersist state.
// When the execution result has been sealed and the parent has already transitioned to StateComplete then
// we can persist the data and transition to StateComplete.
// No error returns are expected during normal operations.
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

// OnParentStateUpdated atomically updates the pipeline's based on the provided information about the parent state.
// This function follows an information-driven, eventually consistent design:
//   - `parentState` tells us that the parent result has progressed to _at least_ that state
//     (it might be further ahead, and we just haven't received information about it).
//   - Idempotency: redundant (older) information is handled gracefully.
//   - New information skipping intermediate states is handled gracefully.
//
// We assume that new information about the parent state changing can arrive through different algorithmic paths concurrently.
// This means that one information source might already be telling us about the parent's most recent new state, while a second
// information source could still be informing us about interim states that the parent has already passed through. This is the
// most robust design, where the caller has to know the least about the internal state of the pipeline and its parent. The
// caller can just feed us with all information they have about the parent's state, and the pipeline will incorporate it correctly.
func (p *Pipeline2) OnParentStateUpdated(parentState optimistic_sync.State2) {
	// Important: We assume that information (from different data sources) might arrive out of order or could be redundant.
	// We utilize the atomic `State2Tracker` to optimistically attempt evolving the state. If the state's tracker accepts, we know
	// exactly what state evolution has taken place (could be multiple state transitions at once, if we haven't heard about some
	// intermediate state transitions yet). If the state's tracker rejects the update, we retrospectively analyze what whether
	// this rejection was expected or is a symptom of a bug or state corruption (in the latter case safe continuation is impossible).

	p.mu.Lock()
	defer p.mu.Unlock()

	oldParentState, parentSuccess := p.parentStateTracker.Evolve(parentState)
	if !parentSuccess {
		panic("to be implemented")
	}
	// reaching the code below means we have successfully evolved the parent state's tracker from `oldParentState` to `parentState`

	if oldParentState == parentState {
		return // redundant information, no state change
	}

	p.checkTrigger1(parentState)
	p.checkTrigger3(parentState)

}

// checkTrigger1 checks and handles Trigger Condition (1): parent is in `Downloading` or `Indexing` or `WaitingPersist` or `Complete`.
// If this pipeline is in `Pending` state and (1) is satisfied, we transition to `Downloading` and schedule the downloading task.
// Concurrency safe, idempotent and tolerates information to arrive out of order about the parent state changing.
func (p *Pipeline2) checkTrigger1(parentState optimistic_sync.State2) {
	// Check Trigger Condition (1): parent is in `Downloading` or `Indexing` or `WaitingPersist` or `Complete`
	trigger1Satisfied := parentState == optimistic_sync.State2Downloading || parentState == optimistic_sync.State2Indexing ||
		parentState == optimistic_sync.State2WaitingPersist || parentState == optimistic_sync.State2Complete
	if !trigger1Satisfied {
		return
	}

	// If and only if Pipeline is in `Pending` state, we atomically transition to `Downloading`. In a second (non-atomic) step, we schedule
	// the downloading task. We must show that this satisfies requirement (I) and (II):
	// (I) We attempt to atomically transition the pipeline state from `Pending` to `Downloading` with an atomic CompareAndSwap operation. At
	// most one go-routine can apply this operation successfully and proceed to schedule the downloading task, which satisfies requirement (I).
	// (II) We assume that notifications about the parent state changing are delivered eventually (no notifications are missed). Despite
	// notifications arriving out of order or being late, we still will eventually learn that the parent has transitioned out of the pending
	// state. Hence, condition (II) is satisfied if the parent is not abandoned. However, if the parent is abandoned, this pipeline is by
	// definition also abandoned and the trigger can be missed (see exceptions to requirement (II) in the Pipeline2 struct specification).

	_, success := p.state.CompareAndSwap(optimistic_sync.State2Pending, optimistic_sync.State2Downloading)
	if !success {
		// some other thread transitioned the pipeline out of Pending state already and should have scheduled the downloading task
		return // noting more to do
	}
	panic("schedule task (1)")
}

// checkTrigger3 checks and handles Trigger Condition (3): sealed & parent completed
// to arrive out of order about the parent state changing .
func (p *Pipeline2) checkTrigger3(parentState optimistic_sync.State2) {

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
