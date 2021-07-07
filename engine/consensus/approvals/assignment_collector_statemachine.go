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

type AssignmentCollectorStateMachine struct {
	AssignmentCollectorBase

	sync.Mutex
	collector atomic.Value
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

func NewAssignmentCollectorStateMachine(collectorBase AssignmentCollectorBase) AssignmentCollector {
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
	for {
		collector := asm.atomicLoadCollector()
		currentState := collector.ProcessingStatus()
		err := collector.ProcessIncorporatedResult(incorporatedResult)
		if err != nil {
			return err
		}
		if currentState != asm.ProcessingStatus() {
			continue
		}
		break
	}
	return nil
}

// ProcessApproval ingests Result Approvals and triggers sealing of execution result
// when sufficient approvals have arrived.
// Error Returns:
//  * nil in case of success (outdated approvals might be silently discarded)
//  * engine.InvalidInputError if the result approval is invalid
//  * any other errors might be symptoms of bugs or internal state corruption (fatal)
func (asm *AssignmentCollectorStateMachine) ProcessApproval(approval *flow.ResultApproval) error {
	for {
		collector := asm.atomicLoadCollector()
		currentState := collector.ProcessingStatus()
		err := collector.ProcessApproval(approval)
		if err != nil {
			return err
		}
		if currentState != asm.ProcessingStatus() {
			continue
		}
		break
	}
	return nil
}

func (asm *AssignmentCollectorStateMachine) CheckEmergencySealing(observer consensus.SealingObservation, finalizedBlockHeight uint64) error {
	collector := asm.atomicLoadCollector()
	return collector.CheckEmergencySealing(observer, finalizedBlockHeight)
}

func (asm *AssignmentCollectorStateMachine) RequestMissingApprovals(observer consensus.SealingObservation, maxHeightForRequesting uint64) (uint, error) {
	collector := asm.atomicLoadCollector()
	return collector.RequestMissingApprovals(observer, maxHeightForRequesting)
}

func (asm *AssignmentCollectorStateMachine) ProcessingStatus() ProcessingStatus {
	collector := asm.atomicLoadCollector()
	return collector.ProcessingStatus()
}

// ChangeProcessingStatus changes the AssignmentCollector's internal processing
// status. The operation is implemented as an atomic compare-and-swap, i.e. the
// state transition is only executed if AssignmentCollector's internal state is
// equal to `expectedValue`. The return indicates whether the state was updated.
// The implementation only allows the transitions
//         CachingApprovals   -> VerifyingApprovals
//         CachingApprovals   -> Orphaned
//    and  VerifyingApprovals -> Orphaned
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

		// re-ingest IncorporatedResults and Approvals that were stored in the cachingCollector
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
	cachingCollector, ok := asm.atomicLoadCollector().(*CachingAssignmentCollector)
	if !ok {
		return nil, fmt.Errorf("collectors current state is %s: %w",
			cachingCollector.ProcessingStatus().String(), ErrDifferentCollectorState)
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
	verifyingCollector, ok := asm.atomicLoadCollector().(*VerifyingAssignmentCollector)
	if !ok {
		return nil, fmt.Errorf("collectors current state is %s: %w",
			verifyingCollector.ProcessingStatus().String(), ErrDifferentCollectorState)
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
	cachingCollector, ok := asm.atomicLoadCollector().(*CachingAssignmentCollector)
	if !ok {
		return nil, fmt.Errorf("collectors current state is %s: %w",
			cachingCollector.ProcessingStatus().String(), ErrDifferentCollectorState)
	}

	verifyingCollector, err := NewVerifyingAssignmentCollector(asm.AssignmentCollectorBase)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate VerifyingAssignmentCollector: %w", err)
	}
	asm.collector.Store(&atomicValueWrapper{collector: verifyingCollector})

	return cachingCollector, nil
}

// reIngestApprovalTask returns a functor for re-ingesting the specified incorporated result;
// functor handles all potential error returned by business logic.
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

// reIngestApprovalTask returns a functor for re-ingesting the specified approval;
// functor handles all potential error returned by business logic.
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
