package module

import "github.com/onflow/flow-go/model/flow"

// ReceiptValidator is an interface which is used for validating
// receipts with respect to current protocol state.
type ReceiptValidator interface {

	// Validate verifies that the ExecutionReceipt satisfies the following conditions:
	//   - is from Execution node with positive weight
	//   - has valid signature
	//   - chunks are in correct format
	//   - execution result has a valid parent and satisfies the subgraph check
	//
	// Returns nil if all checks passed successfully.
	//
	// CAUTION:
	//  - Executed block must be known to protocol state. Otherwise, an exception is returned.
	//
	// Expected errors during normal operations:
	//   - engine.InvalidInputError if receipt violates protocol condition
	//   - engine.UnverifiableInputError if receipt's parent result is unknown
	//
	// All other error are potential symptoms critical internal failures, such as bugs or state corruption.
	Validate(receipts *flow.ExecutionReceipt) error

	// ValidatePayload verifies the ExecutionReceipts and ExecutionResults
	// in the payload for compliance with the protocol:
	// Receipts:
	//   - are from Execution node with positive weight
	//   - have valid signature
	//   - chunks are in correct format
	//   - no duplicates in fork
	//
	// Results:
	//   - have valid parents and satisfy the subgraph check
	//   - extend the execution tree, where the tree root is the latest
	//     finalized block and only results from this fork are included
	//   - no duplicates in fork
	//
	// CAUTION:
	//   - Executed block must be known to protocol state. Otherwise, an exception is returned.
	//
	// Expected errors during normal operations:
	//   - engine.InvalidInputError if some receipts in the candidate block violate protocol condition
	//   - engine.UnverifiableInputError if for some of the receipts, their respective parent result is unknown
	ValidatePayload(candidate *flow.Block) error
}
