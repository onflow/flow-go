package module

import (
	"github.com/onflow/flow-go/model/flow"
)

// SpockVerifier provides functionality to verify SPoCK proofs.
// eg. if there are 5 receipts that have 5 SpockSets, say, Spockset1, Spockset2, Spockset3, Spockset4, and Spockset5.
// If Spockset1 and Spockset2 are for ER1, and they match with each other;
// And Spockset3, Spockset4, Spockset5 are for ER2. and they don't match with each other.
// Then there are 2 category, the first category has bucket: ER1_Spockset1, and the second category has buckets: ER2_Spockset3, ER2_Spockset4, and ER2_Spockset5.
// When we receive each approval, we will first check which category it should go, and find all the buckets to try matching against.
type SpockVerifier interface {

	// AddReceipt adds a receipt into map if the SPoCK proofs do not match any other receipt
	AddReceipt(receipt *flow.ExecutionReceipt) error

	/// VerifyApproval verifies an approval with all the distinct receipts for the approvals
	// result id and returns true if spocks match else false
	VerifyApproval(*flow.ResultApproval) (bool, error)

	// ClearReceipts clears all receipts for a specific resultID
	ClearReceipts(resultID flow.Identifier) bool
}
