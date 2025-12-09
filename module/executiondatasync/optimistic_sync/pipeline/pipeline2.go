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
	"github.com/onflow/flow-go/module/irrecoverable"
)

// Pipeline2 represents a pipelined state machine for a processing single ExecutionResult.
//
// The state machine is initialized in the Pending state, and transitions through the following states:
//
//	Trigger condition: (1)▷┬┄┄┄┄┄┐                           ┌┄┄┐                                 Trigger condition (3):
//	        parent is      ┊    ╭∇─────────────────────────╮ ┊ ╭∇────────────────────────────╮ ┌┄(3)▷┄┬┄┄┄┄┐ sealed & parent completed
//	      Downloading      ┊    │Task Downloading (once):  │ ┊ │Task Indexing (once):        │ ┊      ┊   ╭∇───────────────────────╮
//	      or Indexing      ┊    │1. download               │ ┊ │1. build index from download │ ┊      ┊   │Task Persisting (once): │
//	or WaitingPersist      ┊    │3. add Task Indexing (2)▷┄│┄┘ │3. check trigger (3)  ▷?┄┄┄┄┄│┄┘      ┊   │- database write        │
//	    or Persisting      ┊    │2. CAS state transition   │   │2. CAS state transition      │        ┊   │- CAS state transition  │
//	      or Complete      ┊    │   downloading → indexing │   │   indexing → WaitingPersist │        ┊   │  Persisting → Complete │
//	                       ┊    │               ▷┄┄┄┄┄┄┐   │   │            ▷┄┐              │        ┊   │             ∇          │
//	┏━━━━━━━━━━━━━━━━━━━┓  ┊  ┏━━━━━━━━━━━━━━━━┓       ┊  ┏━━━━━━━━━━━━━━━┓   ┊  ┏━━━━━━━━━━━━━━━━┓   ┊  ┏━━━━━━━━━━━━┓ ┊  ┏━━━━━━━━━━┓
//	┃       state       ┃  ┊  ┃     state      ┃       ┊  ┃     state     ┃   ┊  ┃      state     ┃   ┊  ┃    state   ┃ ┊  ┃  state   ┃
//	┃      Pending      ┃──●─▶┃  Downloading   ┃───────●─▶┃    Indexing   ┃───●─▶┃ WaitingPersist ┃───●─▶┃ Persisting ┃─●─▶┃ Complete ┃
//	┗━━━━━━━━━━━━┯━━━━━━┛     ┗━━━━━━━┯━━━━━━━━┛          ┗━━━━━━━━━━┯━━━━┛      ┗━━━━━━┯━━━━━━━━━┛   ^  ┗━━━━━━━━━━━━┛    ┗━━━━━━━━━━┛
//	             ┃              ╰─────┃────────────────────╯   ╰─────┃──────────────────┃────╯        ^   ╰────────────────────────╯
//	             ┃                    ┃                              ┃                  ┃             ^
//	             ┃                    ┃       ┏━━━━━━━━━━━━━━━┓      ┃                  ┃          This state transition only happens
//	             ┗━━━━━━━━━━━━━━━━━━━━┻━━━━━━━▶   Abandoned   ◀━━━━━━┻━━━━━━━━━━━━━━━━━━┛          once this result is sealed. Hence,
//	                                          ┗━━━━━━━━━━━━━━━┛                                    the result can no longer be abandoned
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
	log           zerolog.Logger
	stateConsumer optimistic_sync.PipelineStateConsumer
	core          optimistic_sync.Core

	downloadingWorkerPool *workerpool.WorkerPool
	indexingWorkerPool    *workerpool.WorkerPool
	persistingWorkerPool  *workerpool.WorkerPool

	signalerCtx irrecoverable.SignalerContext // the escalate irrecoverable errors to the higher-level business logic

	// TODO: This can likely be removed
	mu sync.Mutex

	// The following fields are accessed externally. they are stored using atomics to avoid
	// blocking the caller.
	state              *optimistic_sync.State2Tracker // current state of the pipeline
	parentStateTracker *optimistic_sync.State2Tracker // latest known state of the parent result; eventually consistent: might lag behind actual parent state!

	// TODO: remove the following fields, if they are no longer used (?)

	stateChangedNotifier engine.Notifier // deprecated: to be removed
	worker               *worker         // deprecated: to be removed
	isAbandoned          *atomic.Bool    // deprecated: to be removed
	isIndexed            *atomic.Bool    // deprecated: to be removed

	// TODO: to be moved to Core?
	isSealed *atomic.Bool // deprecated: to be moved
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
	return optimistic_sync.State(p.state.Value())
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

	// (II) We assume that notifications about the parent state changing are delivered eventually (no notifications are missed). Despite
	// notifications arriving out of order or being late, we still will eventually learn that the parent has transitioned out of the pending
	// state. Hence, condition (II) is satisfied if the parent is not abandoned. However, if the parent is abandoned, this pipeline is by
	// definition also abandoned and the trigger can be missed (see exceptions to requirement (II) in the Pipeline2 struct specification).
	p.checkTrigger1(parentState)

	// TODO: explain why requirement (II) is satisfied for Trigger Condition (3)
	p.checkTrigger3(parentState)

}

// checkTrigger1 checks Trigger Condition (1): parent is in `Downloading` or `Indexing` or `Persisting` `WaitingPersist` or `Complete`.
// If this pipeline is in state `Pending` and (1) is satisfied, we transition to `Downloading` and schedule the downloading task.
//
// Concurrency safe, idempotent and tolerates information to arrive out of order about the parent state changing.
// This function guarantees that requirement (I) is satisfied for trigger condition (1).
// However, the caller must ensure that requirement (II) is satisfied.
func (p *Pipeline2) checkTrigger1(parentState optimistic_sync.State2) {
	// Check Trigger Condition (1): we want to avoid erroneously continuing in case of bugs and broken future refactorings. Instead of specifying
	// for which states we do not want to trigger, we explicitly specify for which states we do want to trigger. Thereby, broken future refactorings
	// are more likely to produce liveness failures instead of incorrectly continuing processing and producing incorrect results.
	trigger1Satisfied := parentState == optimistic_sync.State2Downloading || parentState == optimistic_sync.State2Indexing ||
		parentState == optimistic_sync.State2WaitingPersist || parentState == optimistic_sync.State2Persisting ||
		parentState == optimistic_sync.State2Complete
	if !trigger1Satisfied {
		return
	}

	// If and only if Pipeline is in `Pending` state, we atomically transition to `Downloading`. In a second (non-atomic) step, we schedule
	// the downloading task. We must show that this satisfies requirement (I):
	// (I) We attempt to atomically transition the pipeline state from `Pending` to `Downloading` with an atomic CompareAndSwap operation. At
	// most one go-routine can apply this operation successfully and proceed to schedule the downloading task, which satisfies requirement (I).
	_, success := p.state.CompareAndSwap(optimistic_sync.State2Pending, optimistic_sync.State2Downloading)
	if !success {
		// some other thread transitioned the pipeline out of Pending state already and should have scheduled the downloading task
		return // noting more to do
	}
	p.stateConsumer.OnStateUpdated(optimistic_sync.State2Downloading)
	p.downloadingWorkerPool.Submit(p.downloadTask)
}

// downloadTask packages the work of downloading the execution result data (encapsulated in [Core.Download])
// with the lifecycle updates of the pipeline's state machine. Once the call returns from Core without error,
// the downloading work is complete, and we transition the pipeline state from `Downloading` to `Indexing`.
// This should _always_ succeed as the downloading task is the only place allowed to perform this state transition.
//
// TODO:
// Unexpected errors returned from [Core.Download] are escalated as irrecoverable exceptions to the higher-level
// business logic via the SignalerContext.
//
// CAUTION: not idempotent! According to Pipeline2 struct specification, this task is only executed once.
// The caller is responsible for satisfying requirements (I) and (II) for the downloading task (see
// Pipeline2 struct specification for details).
func (p *Pipeline2) downloadTask() {
	panic("update core to allow the following commented-out code: ")
	//err := p.core.Download(ctx)
	//if err != nil {
	// TODO Expected error returns during normal operations:
	//   - context.Canceled: when the context is canceled
	//}

	// Downloading Task finished successful. Atomically transition pipeline state to Indexing. In a second (non-atomic) step, we schedule
	// the indexing task. We must show that this satisfies requirement (I) and (II):
	// (I) We attempt to atomically transition the pipeline state from `Downloading` to `Indexing` with an atomic CompareAndSwap operation. At
	// most one go-routine can apply this operation successfully and proceed to schedule the indexing task, which satisfies requirement (I).
	// (II) Assuming that the algorithm up to this point is correct, trigger conditions are not missed for the downloading task to start.
	// Hence, once the downloading task is complete, we can always proceed to schedule the indexing task, as no additional trigger conditions
	// are required. Hence, condition (II) is satisfied also for the indexing task.
	_, success := p.state.CompareAndSwap(optimistic_sync.State2Downloading, optimistic_sync.State2Indexing)
	if !success {
		panic("this state transition must always succeed")
		// TODO: emit irrecoverable here and escalate it upwards to the higher-level engine
	}
	p.stateConsumer.OnStateUpdated(optimistic_sync.State2Indexing)
	p.indexingWorkerPool.Submit(p.indexingTask)
}

// indexingTask packages the work of indexing the execution result (encapsulated in [Core.Index])
// with the lifecycle updates of the pipeline's state machine. Once the call returns from Core without error,
// the Index work is complete, and we transition the pipeline state from `Indexing` to `WaitingPersist`. This
// should _always_ succeed as the indexing task is the only place allowed to perform this state transition.
//
// TODO:
// Unexpected errors returned from [Core.Index] are escalated as irrecoverable exceptions to the higher-level
// business logic via the SignalerContext.
//
// CAUTION: not idempotent! According to Pipeline2 struct specification, this task is only executed once.
// The caller is responsible for satisfying requirements (I) and (II) for the indexing task (see
// Pipeline2 struct specification for details).
func (p *Pipeline2) indexingTask() {
	err := p.core.Index()
	if err != nil {
		// TODO Expected error returns during normal operations:
		//   - context.Canceled: when the context is canceled
	}

	// Indexing Task finished successful. Atomically transition pipeline state to WaitingPersist:
	_, success := p.state.CompareAndSwap(optimistic_sync.State2Indexing, optimistic_sync.State2WaitingPersist)
	if !success {
		panic("this state transition must always succeed")
		// TODO: emit irrecoverable here and escalate it upwards to the higher-level engine
	}
	p.stateConsumer.OnStateUpdated(optimistic_sync.State2WaitingPersist)

	// Check Trigger Condition (3):
	p.checkTrigger3(p.parentStateTracker.Value())
}

// checkTrigger3 checks Trigger Condition (3): this result is sealed and the processing the parent is Complete.
// If this pipeline is in state `WaitingPersist` and (3) is satisfied, we transition to state `Persisting` and schedule the Persisting task.
//
// Concurrency safe, idempotent and tolerates information to arrive out of order about the parent state changing.
// This function guarantees that requirement (I) is satisfied for trigger condition (1).
// However, the caller must ensure that requirement (II) is satisfied.
func (p *Pipeline2) checkTrigger3(parentState optimistic_sync.State2) {
	trigger3Satisfied := parentState == optimistic_sync.State2Complete && p.IsSealed()
	if !trigger3Satisfied {
		return
	}

	// If and only if Pipeline is in `WaitingPersist` state, we atomically transition to `Persisting`. In a second (non-atomic) step, we schedule
	// the persisting task. We must show that this satisfies requirement (I):
	// (I) We attempt to atomically transition the pipeline state from `WaitingPersist` to `Persisting` with an atomic CompareAndSwap operation.
	// At most one go-routine can apply this operation successfully and proceed to schedule the Persisting task, which satisfies requirement (I).
	_, success := p.state.CompareAndSwap(optimistic_sync.State2WaitingPersist, optimistic_sync.State2Persisting)
	if !success {
		// * We might not have reached `WaitingPersist` yet. In this case, the indexing task will check Trigger Condition (3) again once it is done.
		// * We might have already transitioned from `WaitingPersist` to `Persisting`. In this case, the persisting task has already been scheduled.
		// * The only remaining possibility is that the pipeline is Abandoned.
		return // In all cases, there is nothing more to do here.
	}
	p.stateConsumer.OnStateUpdated(optimistic_sync.State2Persisting)
	p.indexingWorkerPool.Submit(p.persistingTask)
}

// indexingTask packages the work of saving the indexed execution result to the database (encapsulated in
// [Core.Persist]) with the lifecycle updates of the pipeline's state machine. Once the call returns from Core without
// error, the persisting work is complete, and we transition the pipeline state from `Persisting` to `Complete`. This
// should _always_ succeed as the downloading task is the only place allowed to perform this state transition.
//
// TODO:
// Unexpected errors returned from [Core.Persist] are escalated as irrecoverable exceptions to the higher-level
// business logic via the SignalerContext.
//
// CAUTION: not idempotent! According to Pipeline2 struct specification, this task is only executed once.
// The caller is responsible for satisfying requirements (I) and (II) for the indexing task (see
// Pipeline2 struct specification for details).
func (p *Pipeline2) persistingTask() {
	err := p.core.Persist()
	if err != nil {
		// TODO Are there any expected error returns here? Maybe (?):
		//   - context.Canceled: when the context is canceled (e.g. during graceful shutdown).
		//     We could include it for uniformity here, even though the implementation might never return this sentinel.
	}

	// Persisting Task finished successful. Atomically transition pipeline state to Complete:
	_, success := p.state.CompareAndSwap(optimistic_sync.State2Persisting, optimistic_sync.State2Complete)
	if !success {
		panic("this state transition must always succeed")
		// TODO: emit irrecoverable here and escalate it upwards to the higher-level engine
	}
	p.stateConsumer.OnStateUpdated(optimistic_sync.State2Complete)
}

// IsSealed returns true if and only if the execution result has been marked as sealed.
func (p *Pipeline2) IsSealed() bool {
	panic("implement me")
}

// Abandon marks the pipeline as abandoned
// This will cause the pipeline to eventually transition to the Abandoned state and halt processing
func (p *Pipeline2) Abandon() {
	if p.isAbandoned.CompareAndSwap(false, true) {
		p.stateChangedNotifier.Notify()
	}
}
