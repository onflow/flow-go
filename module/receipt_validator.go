package module

import (
	"github.com/onflow/flow-go/model/flow"
)

// ReceiptValidator is an interface which is used for validating
// receipts with respect to current protocol state.
type ReceiptValidator interface {

	// Validate verifies that the ExecutionReceipt satisfies the following conditions:
	//   - is from Execution node with positive weight
	//   - has valid signature
	//   - chunks are in correct format
	//   - execution result has a valid parent and satisfies the subgraph check
	//
	// In order to validate a receipt, both the executed block and the parent result
	// referenced in `receipt.ExecutionResult` must be known. We return nil if all checks
	// pass successfully.
	//
	// Expected errors during normal operations:
	//   - engine.InvalidInputError if receipt violates protocol condition
	//   - module.UnknownBlockError if the executed block is unknown
	//   - module.UnknownResultError if the receipt's parent result is unknown
	//
	// All other error are potential symptoms critical internal failures, such as bugs or state corruption.
	Validate(receipt *flow.ExecutionReceipt) error

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
	// Expected errors during normal operations:
	//   - engine.InvalidInputError if some receipts in the candidate block violate protocol condition
	//   - module.UnknownBlockError if the candidate block's _parent_ is unknown
	//
	// All other error are potential symptoms critical internal failures, such as bugs or state corruption.
	ValidatePayload(candidate *flow.Block) error
}
