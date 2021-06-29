package consensus

import "github.com/onflow/flow-go/model/flow"

// SealingCore processes incoming execution results and result approvals.
// Accepts `flow.IncorporatedResult` to start processing approvals for particular result.
// Whenever enough approvals are collected produces a candidate seal and adds it to the mempool.
// Implementations of SealingCore are _concurrency safe_.
type SealingCore interface {
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
	// ProcessFinalizedBlock processes finalization events in blocking way. Concurrency safe.
	// Returns:
	// * exception in case of unexpected error
	// * nil - successfully processed finalized block
	ProcessFinalizedBlock(finalizedBlockID flow.Identifier) error
}
