package approvals

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
)

var (
	ErrInvalidCollectorStateTransition = errors.New("invalid state transition")
	ErrDifferentCollectorState         = errors.New("different state")
)

type AssignmentCollectorStateMachine struct {
	assignmentCollectorBase

	sync.Mutex
	collector atomic.Value
}

func NewAssignmentCollectorStateMachine(collectorBase assignmentCollectorBase) AssignmentCollector {
	return &AssignmentCollectorStateMachine{
		assignmentCollectorBase: collectorBase,
	}
}

// ProcessIncorporatedResult starts tracking the approval for IncorporatedResult.
// Method is idempotent.
// Error Returns:
//  * no errors expected during normal operation;
//    errors might be symptoms of bugs or internal state corruption (fatal)
func (asm *AssignmentCollectorStateMachine) ProcessIncorporatedResult(incorporatedResult *flow.IncorporatedResult) error {
	for {
		collector := asm.collector.Load().(AssignmentCollectorState)
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
		collector := asm.collector.Load().(AssignmentCollectorState)
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

func (asm *AssignmentCollectorStateMachine) CheckEmergencySealing(finalizedBlockHeight uint64, observer consensus.SealingObservation) error {
	collector := asm.collector.Load().(AssignmentCollectorState)
	return collector.CheckEmergencySealing(finalizedBlockHeight, observer)
}

func (asm *AssignmentCollectorStateMachine) RequestMissingApprovals(maxHeightForRequesting uint64, observer consensus.SealingObservation) (uint, error) {
	collector := asm.collector.Load().(AssignmentCollectorState)
	return collector.RequestMissingApprovals(maxHeightForRequesting, observer)
}

func (asm *AssignmentCollectorStateMachine) ProcessingStatus() ProcessingStatus {
	collector := asm.collector.Load().(AssignmentCollectorState)
	return collector.ProcessingStatus()
}

// ChangeProcessingStatus changes the AssignmentCollector's internal processing
// status. The operation is implemented as an atomic compare-and-swap, i.e. the
// state transition is only executed if AssignmentCollector's internal state is
// equal to `expectedValue`. The return indicates whether the state was updated.
// The implementation only allows the transitions
//         CachingApprovals -> VerifyingApprovals
//    and                      VerifyingApprovals -> Orphaned
// Error returns:
// * nil if the state transition was successfully executed
// * ErrDifferentCollectorState if the AssignmentCollector's state is different than expectedCurrentStatus
// * ErrInvalidCollectorStateTransition if the given state transition is impossible
// * all other errors are unexpected and potential symptoms of internal bugs or state corruption (fatal)
func (asm *AssignmentCollectorStateMachine) ChangeProcessingStatus(expectedCurrentStatus, newStatus ProcessingStatus) error {
	// state transition: VerifyingApprovals -> Orphaned
	if (expectedCurrentStatus == VerifyingApprovals) && (newStatus == Orphaned) {
		_, err := asm.verifying2Orphaned()
		if err != nil {
			return fmt.Errorf("failed to transistion AssignmentCollector from %s to %s: %w", expectedCurrentStatus.String(), newStatus.String(), err)
		}
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
			asm.workerpool.Submit(task)
		}
		for _, approval := range cachingCollector.GetApprovals() {
			task := asm.reIngestApprovalTask(approval)
			asm.workerpool.Submit(task)
		}
		return nil
	}

	return fmt.Errorf("cannot transition from %s to %s: %w", expectedCurrentStatus.String(), newStatus.String(), ErrInvalidCollectorStateTransition)
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
	verifyingCollector, ok := asm.collector.Load().(*VerifyingAssignmentCollector)
	if !ok {
		return nil, fmt.Errorf("collectors current state is %s: %w",
			verifyingCollector.ProcessingStatus().String(), ErrDifferentCollectorState)
	}
	asm.collector.Store(NewOrphanAssignmentCollector(asm.assignmentCollectorBase))
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
	cachingCollector, ok := asm.collector.Load().(*CachingAssignmentCollector)
	if !ok {
		return nil, fmt.Errorf("collectors current state is %s: %w",
			cachingCollector.ProcessingStatus().String(), ErrDifferentCollectorState)
	}

	verifyingCollector, err := NewVerifyingAssignmentCollector(asm.assignmentCollectorBase)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate VerifyingAssignmentCollector: %w", err)
	}
	asm.collector.Store(verifyingCollector)

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
