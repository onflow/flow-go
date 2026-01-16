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
	//   - [optimistic_sync.ErrExecutionResultNotReady]: If criteria cannot be satisfied at the moment.
	//   - [optimistic_sync.CriteriaNotMetError]: Returned when the block is already
	//     sealed but no execution result can satisfy the provided criteria.
	ExecutionResultInfo(blockID flow.Identifier, criteria Criteria) (*ExecutionResultInfo, error)
}
