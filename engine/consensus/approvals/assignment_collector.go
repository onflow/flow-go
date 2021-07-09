package approvals

import (
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
)

type ProcessingStatus int

const (
	CachingApprovals ProcessingStatus = iota
	VerifyingApprovals
	Orphaned
)

func (ps ProcessingStatus) String() string {
	names := [...]string{"CachingApprovals", "VerifyingApprovals", "Orphaned"}
	if ps < CachingApprovals || ps > Orphaned {
		return "UNKNOWN"
	}
	return names[ps]
}

// AssignmentCollectorState is a general interface for representing AssignmentCollector in business logic.
// AssignmentCollectorState can be in 3 different states, described by ProcessingStatus.
// Objects implementing AssignmentCollectorState can change their internal state over object lifetime.
type AssignmentCollectorState interface {
	BlockID() flow.Identifier
	Block() *flow.Header

	ResultID() flow.Identifier
	Result() *flow.ExecutionResult

	// ProcessIncorporatedResult starts tracking the approval for IncorporatedResult.
	// Method is idempotent.
	// Error Returns:
	//  * no errors expected during normal operation;
	//    errors might be symptoms of bugs or internal state corruption (fatal)
	ProcessIncorporatedResult(incorporatedResult *flow.IncorporatedResult) error

	// ProcessApproval ingests Result Approvals and triggers sealing of execution result
	// when sufficient approvals have arrived.
	// Error Returns:
	//  * nil in case of success (outdated approvals might be silently discarded)
	//  * engine.InvalidInputError if the result approval is invalid
	//  * any other errors might be symptoms of bugs or internal state corruption (fatal)
	ProcessApproval(approval *flow.ResultApproval) error

	CheckEmergencySealing(observer consensus.SealingObservation, finalizedBlockHeight uint64) error
	RequestMissingApprovals(observer consensus.SealingObservation, maxHeightForRequesting uint64) (uint, error)

	// ProcessingStatus returns the AssignmentCollector's  ProcessingStatus
	ProcessingStatus() ProcessingStatus
}

// AssignmentCollector is an specific extended interface which is used by state machine to perform state transitions.
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
