package optimistic_sync

import (
	"github.com/onflow/flow-go/model/flow"
)

// Criteria defines the filtering criteria for execution result queries.
// It specifies requirements for execution result selection, including the number
// of agreeing executors and requires executor nodes.
type Criteria struct {
	// AgreeingExecutorsCount is the number of receipts including the same ExecutionResult
	AgreeingExecutorsCount uint

	// RequiredExecutors is the list of EN node IDs, one of which must have produced the result
	RequiredExecutors flow.IdentifierList

	// ParentExecutionResultID is the ID of the parent execution result.
	// If set, the result's PreviousResultID field must exactly match.
	ParentExecutionResultID flow.Identifier
}

// DefaultCriteria is the operator's default criteria for execution result queries.
var DefaultCriteria = Criteria{
	AgreeingExecutorsCount: 1,
}

// OverrideWith overrides the original criteria with the incoming criteria, returning a new Criteria object.
// Fields from `override` criteria take precedence when set.
func (c *Criteria) OverrideWith(override Criteria) Criteria {
	newCriteria := *c

	if override.AgreeingExecutorsCount > 0 {
		newCriteria.AgreeingExecutorsCount = override.AgreeingExecutorsCount
	}

	if len(override.RequiredExecutors) > 0 {
		newCriteria.RequiredExecutors = override.RequiredExecutors
	}

	if override.ParentExecutionResultID != flow.ZeroID {
		newCriteria.ParentExecutionResultID = override.ParentExecutionResultID
	}

	return newCriteria
}

// ExecutionResultInfo contains the result of an execution result query.
// It includes both the execution result and the execution nodes that produced it.
type ExecutionResultInfo struct {
	// ExecutionResult is the execution result for the queried block
	ExecutionResultID flow.Identifier
	// ExecutionNodes is the list of execution node identities that produced the result
	ExecutionNodes flow.IdentitySkeletonList
}

// ExecutionResultInfoProvider provides execution results and execution nodes based on criteria.
// It allows querying for execution results by block ID with specific filtering criteria
// to ensure consistency and reliability of execution results.
type ExecutionResultInfoProvider interface {
	// ExecutionResultInfo retrieves execution results and associated execution nodes for a given block ID
	// based on the provided criteria.
	//
	// Expected errors during normal operations:
	//   - [optimistic_sync.ErrBlockNotFound]: If the request is for the spork root block, and the node was bootstrapped
	//     from a newer block.
	//   - [optimistic_sync.NotEnoughAgreeingExecutorsError]: If no insufficient receipts are found for given block ID.
	//   - [optimistic_sync.ErrForkAbandoned]: If the execution fork of an execution node from which we were getting the
	//     execution results was abandoned.
	//   - [optimistic_sync.ErrNotEnoughAgreeingExecutors]: If there are not enough execution nodes that produced the
	//     execution result.
	//   - [optimistic_sync.ErrRequiredExecutorNotFound]: If the criteria's required executor is not in the group of
	//     execution nodes that produced the execution result.
	ExecutionResultInfo(blockID flow.Identifier, criteria Criteria) (*ExecutionResultInfo, error)
}
