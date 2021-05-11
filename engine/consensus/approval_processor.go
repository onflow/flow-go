package consensus

import "github.com/onflow/flow-go/model/flow"

// ResultApprovalProcessor performs processing of execution results and result approvals.
// Accepts `flow.IncorporatedResult` to start processing approvals for particular result.
// Whenever enough approvals are collected produces a candidate seal and adds it to the mempool.
type ResultApprovalProcessor interface {
	// ProcessApproval processes approval in blocking way. Concurrency safe.
	// Returns:
	// * exception in case of unexpected error
	// * nil - successfully processed result approval
	ProcessApproval(approval *flow.ResultApproval) error
	// ProcessIncorporatedResult processes incorporated result in blocking way. Concurrency safe.
	// Returns:
	// * exception in case of unexpected error
	// * nil - successfully processed incorporated result
	ProcessIncorporatedResult(result *flow.IncorporatedResult) error
}
