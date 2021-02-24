package module

import "github.com/onflow/flow-go/model/flow"

// ReceiptValidator is an interface which is used for validating
// receipts with respect to current protocol state.
type ReceiptValidator interface {
	// Validate performs verifies that the ExecutionReceipt satisfies
	// the following conditions:
	// 	* is from Execution node with positive weight
	//	* has valid signature
	//	* chunks are in correct format
	// 	* execution result has a valid parent and satisfies the subgraph check
	// Returns nil if all checks passed successfully.
	// Expected errors during normal operations:
	// * engine.InvalidInputError
	// * validation.MissingPreviousResultError
	Validate(receipts []*flow.ExecutionReceipt) error
	ValidatePayload(candidate *flow.Block) error
}
