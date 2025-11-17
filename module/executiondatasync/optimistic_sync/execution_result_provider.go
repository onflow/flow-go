package optimistic_sync

import (
	"github.com/onflow/flow-go/model/flow"
)

// ExecutionResultInfoProvider provides execution results and execution nodes based on criteria.
// It allows querying for execution results by block ID with specific filtering criteria
// to ensure consistency and reliability of execution results.
type ExecutionResultInfoProvider interface {
	// ExecutionResultInfo retrieves execution results and associated execution nodes for a given block ID
	// based on the provided criteria. It returns ExecutionResultInfo containing the execution result and
	// the execution nodes that produced it.
	//
	// Expected errors during normal operations:
	//   - backend.InsufficientExecutionReceipts - found insufficient receipts for given block ID.
	ExecutionResultInfo(blockID flow.Identifier, criteria Criteria) (*ExecutionResultInfo, error)
}

// ExecutionResultInfo contains the result of an execution result query.
// It includes both the execution result and the execution nodes that produced it.
type ExecutionResultInfo struct {
	// ExecutionResult is the execution result for the queried block
	ExecutionResultID flow.Identifier
	// ExecutionNodes is the list of execution node identities that produced the result
	ExecutionNodes flow.IdentitySkeletonList
}
