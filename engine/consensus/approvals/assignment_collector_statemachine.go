package approvals

import (
	"errors"
	"fmt"
	"sync"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
)

var (
	ErrInvalidCollectorStateTransition = errors.New("invalid state transition")
	ErrDifferentCollectorState         = errors.New("different state")
)

// AssignmentCollectorStateMachine implements the `AssignmentCollector` interface.
// It wraps the current `AssignmentCollectorState` and provides logic for state transitions.
// Any state-specific logic is delegated to the state-specific instance.
// AssignmentCollectorStateMachine is fully concurrent.
//
// Comment on concurrency safety for state-specific business logic:
//  * AssignmentCollectorStateMachine processes state updates concurrently with
//    state-specific business logic. Hence, it can happen that we update a stale
//    state.
//  * To guarantee that we hand inputs to the latest state, we employ a
//    "Compare And Repeat Pattern": we atomically read the state before and after the
//    operation. If the state changed, we updated a stale state. We repeat until
//    we confirm that the latest state was updated.
type AssignmentCollectorStateMachine struct {
	AssignmentCollectorBase

	// collector references the assignment collector in its current state. The value is
	// frequently read, but infrequently updated. Reads are atomic and therefore concurrency
	// safe. For state updates (write), we use a mutex to guarantee that a state update is
	// always based on the most recent value.
	collector atomic.Value
	sync.Mutex
}

func (asm *AssignmentCollectorStateMachine) atomicLoadCollector() AssignmentCollectorState {
	return asm.collector.Load().(*atomicValueWrapper).collector
}

// atomic.Value doesn't allow storing interfaces as atomic values,
// it requires that stored type is always the same so we need a wrapper that will mitigate this restriction
// https://github.com/golang/go/issues/22550
type atomicValueWrapper struct {
	collector AssignmentCollectorState
}

func NewAssignmentCollectorStateMachine(collectorBase AssignmentCollectorBase) *AssignmentCollectorStateMachine {
	sm := &AssignmentCollectorStateMachine{
		AssignmentCollectorBase: collectorBase,
	}

	// by default start with caching collector
	sm.collector.Store(&atomicValueWrapper{
		collector: NewCachingAssignmentCollector(collectorBase),
	})
	return sm
}

// ProcessIncorporatedResult starts tracking the approval for IncorporatedResult.
// Method is idempotent.
// Error Returns:
//  * no errors expected during normal operation;
//    errors might be symptoms of bugs or internal state corruption (fatal)
func (asm *AssignmentCollectorStateMachine) ProcessIncorporatedResult(incorporatedResult *flow.IncorporatedResult) error {
	for { // Compare And Repeat if state update occurred concurrently
		collector := asm.atomicLoadCollector()
		currentState := collector.ProcessingStatus()
		err := collector.ProcessIncorporatedResult(incorporatedResult)
		if err != nil {
			return fmt.Errorf("could not process incorporated result %v: %w", incorporatedResult.ID(), err)
		}
		if currentState != asm.ProcessingStatus() {
			continue
		}
		return nil
	}
}

// ProcessApproval ingests Result Approvals and triggers sealing of execution result
// when sufficient approvals have arrived.
// Error Returns:
//  * nil in case of success (outdated approvals might be silently discarded)
//  * engine.InvalidInputError if the result approval is invalid
//  * any other errors might be symptoms of bugs or internal state corruption (fatal)
func (asm *AssignmentCollectorStateMachine) ProcessApproval(approval *flow.ResultApproval) error {
	for { // Compare And Repeat if state update occurred concurrently
		collector := asm.atomicLoadCollector()
		currentState := collector.ProcessingStatus()
		err := collector.ProcessApproval(approval)
		if err != nil {
			return fmt.Errorf("could not process approval %v: %w", approval.ID(), err)
		}
		if currentState != asm.ProcessingStatus() {
			continue
		}
		return nil
	}
}

// CheckEmergencySealing checks whether this AssignmentCollector can be emergency
// sealed. If this is the case, the AssignmentCollector produces a candidate seal
// as part of this method call. No errors are expected during normal operations.
func (asm *AssignmentCollectorStateMachine) CheckEmergencySealing(observer consensus.SealingObservation, finalizedBlockHeight uint64) error {
	collector := asm.atomicLoadCollector()
	return collector.CheckEmergencySealing(observer, finalizedBlockHeight)
}

// RequestMissingApprovals sends requests for missing approvals to the respective
// verification nodes. Returns number of requests made. No errors are expected
// during normal operations.
func (asm *AssignmentCollectorStateMachine) RequestMissingApprovals(observer consensus.SealingObservation, maxHeightForRequesting uint64) (uint, error) {
	collector := asm.atomicLoadCollector()
	return collector.RequestMissingApprovals(observer, maxHeightForRequesting)
}

// ProcessingStatus returns the AssignmentCollector's ProcessingStatus (state descriptor).
func (asm *AssignmentCollectorStateMachine) ProcessingStatus() ProcessingStatus {
	collector := asm.atomicLoadCollector()
	return collector.ProcessingStatus()
}

// ChangeProcessingStatus changes the AssignmentCollector's internal processing
// status. The operation is implemented as an atomic compare-and-swap, i.e. the
// state transition is only executed if AssignmentCollector's internal state is
// equal to `expectedValue`. The return indicates whether the state was updated.
// The implementation only allows the following transitions:
//         CachingApprovals   -> VerifyingApprovals
//         CachingApprovals   -> Orphaned
//         VerifyingApprovals -> Orphaned
// Error returns:
// * nil if the state transition was successfully executed
// * ErrDifferentCollectorState if the AssignmentCollector's state is different than expectedCurrentStatus
// * ErrInvalidCollectorStateTransition if the given state transition is impossible
// * all other errors are unexpected and potential symptoms of internal bugs or state corruption (fatal)
func (asm *AssignmentCollectorStateMachine) ChangeProcessingStatus(expectedCurrentStatus, newStatus ProcessingStatus) error {
	// don't transition between same states
	if expectedCurrentStatus == newStatus {
		return nil
	}

	// state transition: VerifyingApprovals -> Orphaned
	if (expectedCurrentStatus == VerifyingApprovals) && (newStatus == Orphaned) {
		_, err := asm.verifying2Orphaned()
		if err != nil {
			return fmt.Errorf("failed to transistion AssignmentCollector from %s to %s: %w", expectedCurrentStatus.String(), newStatus.String(), err)
		}
		return nil
	}

	// state transition: CachingApprovals -> Orphaned
	if (expectedCurrentStatus == CachingApprovals) && (newStatus == Orphaned) {
		_, err := asm.caching2Orphaned()
		if err != nil {
			return fmt.Errorf("failed to transistion AssignmentCollector from %s to %s: %w", expectedCurrentStatus.String(), newStatus.String(), err)
		}
		return nil
	}

	// state transition: CachingApprovals -> VerifyingApprovals
	if (expectedCurrentStatus == CachingApprovals) && (newStatus == VerifyingApprovals) {
		cachingCollector, err := asm.caching2Verifying()
		if err != nil {
			return fmt.Errorf("failed to transistion AssignmentCollector from %s to %s: %w", expectedCurrentStatus.String(), newStatus.String(), err)
		}

		// From this goroutine's perspective, the "Compare And Repeat Pattern" guarantees
		// that any IncorporatedResult or ResultApproval is
		//  * either already stored in the old state (i.e. the CachingAssignmentCollector)
		//    when this goroutine retrieves it
		//  * or the incorporated result / approval is subsequently added to updated state
		//    (i.e. the VerifyingAssignmentCollector) after this goroutine stored it
		// Hence, this goroutine only needs to hand the IncorporatedResults and ResultApprovals
		// that are stored in the CachingAssignmentCollector to the VerifyingAssignmentCollector.
		//
		// Generally, we would like to process the cached data concurrently here, because
		// sequential processing is too slow. However, we should only allocate limited resources
		// to avoid other components being starved. Therefore, we use a WorkerPool to queue
		// the processing tasks and work through a limited number of them concurrently.
		for _, ir := range cachingCollector.GetIncorporatedResults() {
			task := asm.reIngestIncorporatedResultTask(ir)
			asm.workerPool.Submit(task)
		}
		for _, approval := range cachingCollector.GetApprovals() {
			task := asm.reIngestApprovalTask(approval)
			asm.workerPool.Submit(task)
		}
		return nil
	}

	return fmt.Errorf("cannot transition from %s to %s: %w", expectedCurrentStatus.String(), newStatus.String(), ErrInvalidCollectorStateTransition)
}

// caching2Orphaned ensures that the collector is currently in state `CachingApprovals`
// and replaces it by a newly-created OrphanAssignmentCollector.
// Returns:
// * CachingAssignmentCollector as of before the update
// * ErrDifferentCollectorState if the AssignmentCollector's state is _not_ `CachingApprovals`
// * all other errors are unexpected and potential symptoms of internal bugs or state corruption (fatal)
func (asm *AssignmentCollectorStateMachine) caching2Orphaned() (*CachingAssignmentCollector, error) {
	asm.Lock()
	defer asm.Unlock()
	clr := asm.atomicLoadCollector()
	cachingCollector, ok := clr.(*CachingAssignmentCollector)
	if !ok {
		return nil, fmt.Errorf("collector's current state is %s: %w", clr.ProcessingStatus().String(), ErrDifferentCollectorState)
	}
	asm.collector.Store(&atomicValueWrapper{collector: NewOrphanAssignmentCollector(asm.AssignmentCollectorBase)})
	return cachingCollector, nil
}

// verifying2Orphaned ensures that the collector is currently in state `VerifyingApprovals`
// and replaces it by a newly-created OrphanAssignmentCollector.
// Returns:
// * VerifyingAssignmentCollector as of before the update
// * ErrDifferentCollectorState if the AssignmentCollector's state is _not_ `VerifyingApprovals`
// * all other errors are unexpected and potential symptoms of internal bugs or state corruption (fatal)
func (asm *AssignmentCollectorStateMachine) verifying2Orphaned() (*VerifyingAssignmentCollector, error) {
	asm.Lock()
	defer asm.Unlock()
	clr := asm.atomicLoadCollector()
	verifyingCollector, ok := clr.(*VerifyingAssignmentCollector)
	if !ok {
		return nil, fmt.Errorf("collector's current state is %s: %w", clr.ProcessingStatus().String(), ErrDifferentCollectorState)
	}
	asm.collector.Store(&atomicValueWrapper{collector: NewOrphanAssignmentCollector(asm.AssignmentCollectorBase)})
	return verifyingCollector, nil
}

// caching2Verifying ensures that the collector is currently in state `CachingApprovals`
// and replaces it by a newly-created VerifyingAssignmentCollector.
// Returns:
// * CachingAssignmentCollector as of before the update
// * ErrDifferentCollectorState if the AssignmentCollector's state is _not_ `CachingApprovals`
// * all other errors are unexpected and potential symptoms of internal bugs or state corruption (fatal)
func (asm *AssignmentCollectorStateMachine) caching2Verifying() (*CachingAssignmentCollector, error) {
	asm.Lock()
	defer asm.Unlock()
	clr := asm.atomicLoadCollector()
	cachingCollector, ok := clr.(*CachingAssignmentCollector)
	if !ok {
		return nil, fmt.Errorf("collector's current state is %s: %w", clr.ProcessingStatus().String(), ErrDifferentCollectorState)
	}

	verifyingCollector, err := NewVerifyingAssignmentCollector(asm.AssignmentCollectorBase)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate VerifyingAssignmentCollector: %w", err)
	}
	asm.collector.Store(&atomicValueWrapper{collector: verifyingCollector})

	return cachingCollector, nil
}

// reIngestIncorporatedResultTask returns a functor for re-ingesting the specified
// IncorporatedResults; functor handles all potential business logic errors.
func (asm *AssignmentCollectorStateMachine) reIngestIncorporatedResultTask(incResult *flow.IncorporatedResult) func() {
	task := func() {
		err := asm.ProcessIncorporatedResult(incResult)
		if err != nil {
			asm.log.Fatal().Err(err).
				Str("executed_block_id", incResult.Result.BlockID.String()).
				Str("result_id", incResult.Result.ID().String()).
				Str("incorporating_block_id", incResult.IncorporatedBlockID.String()).
				Str("incorporated_result_id", incResult.ID().String()).
				Msg("re-ingesting incorporated results failed")
		}
	}
	return task
}

// reIngestApprovalTask returns a functor for re-ingesting the specified
// ResultApprovals; functor handles all potential business logic errors.
func (asm *AssignmentCollectorStateMachine) reIngestApprovalTask(approval *flow.ResultApproval) func() {
	task := func() {
		err := asm.ProcessApproval(approval)
		if err != nil {
			lg := asm.log.With().Err(err).
				Str("approver_id", approval.Body.ApproverID.String()).
				Str("executed_block_id", approval.Body.BlockID.String()).
				Str("result_id", approval.Body.ExecutionResultID.String()).
				Str("approval_id", approval.ID().String()).
				Logger()
			if engine.IsInvalidInputError(err) {
				lg.Error().Msgf("received invalid approval")
				return
			}
			asm.log.Fatal().Msg("unexpected error re-ingesting result approval")
		}
	}
	return task
}
