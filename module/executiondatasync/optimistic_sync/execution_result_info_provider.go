package optimistic_sync

import (
	"github.com/onflow/flow-go/model/flow"
)

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
	//   - [optimistic_sync.ErrBlockBeforeNodeHistory]: If the request is for data before the node's root block.
	//   - [optimistic_sync.ErrParentMismatch]: If the execution fork of an execution node from which we were getting the
	//     execution results was abandoned.
	//   - [optimistic_sync.ErrNotEnoughAgreeingExecutors]: If there are not enough execution nodes that produced the
	//     execution result.
	//   - [optimistic_sync.ErrRequiredExecutorNotFound]: If the criteria's required executor is not in the group of
	//     execution nodes that produced the execution result.
	//   - [optimistic_sync.AgreeingExecutorsCountExceededError]: Agreeing executors count exceeds available executors.
	//   - [optimistic_sync.UnknownRequiredExecutorError]: A required executor ID is not in the available set.
	//   - [optimistic_sync.CriteriaNotMetError]: Returned when the block is already
	//     sealed but no execution result can satisfy the provided criteria.
	ExecutionResultInfo(blockID flow.Identifier, criteria Criteria) (*ExecutionResultInfo, error)
}
