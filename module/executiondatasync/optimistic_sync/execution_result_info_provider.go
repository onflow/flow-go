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
}

// DefaultCriteria is the operator's default criteria for execution result queries.
var DefaultCriteria = Criteria{
	AgreeingExecutorsCount: 2,
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
	// based on the provided criteria. It returns ExecutionResultInfo containing the execution result and
	// the execution nodes that produced it.
	//
	// Expected error returns during normal operation:
	//   - [common.InsufficientExecutionReceipts]: Found insufficient receipts for given block ID.
	//   - [storage.ErrNotFound]: If the data was not found.
	//   - [common.RequiredExecutorsCountExceeded]: Required executor IDs count exceeds available executors.
	//   - [common.UnknownRequiredExecutor]: A required executor ID is not in the available set.
	ExecutionResultInfo(blockID flow.Identifier, criteria Criteria) (*ExecutionResultInfo, error)
}
