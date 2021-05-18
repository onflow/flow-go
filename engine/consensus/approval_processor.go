package sealing

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

	// ProcessReceipt processes receipt which was submitted from p2p network.
	// This function is needed only for phase 2 sealing and verification where receipts can
	// be broadcast through p2p network. Will be removed in phase 3.
	ProcessReceipt(receipt *flow.ExecutionReceipt) error

	// ProcessFinalizedBlock processes finalization events in blocking way. Concurrency safe.
	// Returns:
	// * exception in case of unexpected error
	// * nil - successfully processed finalized block
	ProcessFinalizedBlock(finalizedBlockID flow.Identifier) error
}
