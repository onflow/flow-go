package approvals

import (
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
)

// ProcessingStatus is a state descriptor for the AssignmentCollector.
type ProcessingStatus int

const (
	// CachingApprovals is a state descriptor for the AssignmentCollector. In this state,
	// the collector is currently caching approvals but _not_ yet processing them.
	CachingApprovals ProcessingStatus = iota
	// VerifyingApprovals is a state descriptor for the AssignmentCollector. In this state,
	// the collector is processing approvals.
	VerifyingApprovals
	// Orphaned is a state descriptor for the AssignmentCollector. In this state,
	// the collector discards all approvals.
	Orphaned
)

func (ps ProcessingStatus) String() string {
	names := [...]string{"CachingApprovals", "VerifyingApprovals", "Orphaned"}
	if ps < CachingApprovals || ps > Orphaned {
		return "UNKNOWN"
	}
	return names[ps]
}

// AssignmentCollector tracks the known verifier assignments for a particular execution result.
// For the same result, there can be multiple different assignments. This happens if the result is
// incorporated in different forks, because in each fork a unique verifier assignment is generated
// using the incorporating block's unique 'source of randomness'.
// An AssignmentCollector can be in different states (enumerated by ProcessingStatus). State
// transitions are atomic and concurrency safe. The high-level AssignmentCollector implements the
// logic for state transitions, while it delegates the state-specific portion of the logic to
// `AssignmentCollectorState`.
type AssignmentCollector interface {
	AssignmentCollectorState

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
	ChangeProcessingStatus(expectedValue, newValue ProcessingStatus) error
}

// AssignmentCollectorState represents an AssignmentCollector in one specific state (without any
// knowledge about state transitions).
type AssignmentCollectorState interface {
	// BlockID returns the ID of the executed block.
	BlockID() flow.Identifier
	// Block returns the header of the executed block.
	Block() *flow.Header
	// ResultID returns the ID of the result this assignment collector tracks.
	ResultID() flow.Identifier
	// Result returns the result this assignment collector tracks.
	Result() *flow.ExecutionResult

	// ProcessIncorporatedResult starts tracking the approval for IncorporatedResult.
	// Method is idempotent.
	// Error Returns:
	//  * no errors expected during normal operation;
	//    errors might be symptoms of bugs or internal state corruption (fatal)
	ProcessIncorporatedResult(incorporatedResult *flow.IncorporatedResult) error

	// ProcessApproval ingests Result Approvals and triggers sealing of execution result
	// when sufficient approvals have arrived. Method is idempotent.
	// Error Returns:
	//  * nil in case of success (outdated approvals might be silently discarded)
	//  * engine.InvalidInputError if the result approval is invalid
	//  * any other errors might be symptoms of bugs or internal state corruption (fatal)
	ProcessApproval(approval *flow.ResultApproval) error

	// CheckEmergencySealing checks whether this AssignmentCollector can be emergency
	// sealed. If this is the case, the AssignmentCollector produces a candidate seal
	// as part of this method call. No errors are expected during normal operations.
	CheckEmergencySealing(observer consensus.SealingObservation, finalizedBlockHeight uint64) error

	// RequestMissingApprovals sends requests for missing approvals to the respective
	// verification nodes. Returns number of requests made. No errors are expected
	// during normal operations.
	RequestMissingApprovals(observer consensus.SealingObservation, maxHeightForRequesting uint64) (uint, error)

	// ProcessingStatus returns the AssignmentCollector's ProcessingStatus (state descriptor).
	ProcessingStatus() ProcessingStatus
}
