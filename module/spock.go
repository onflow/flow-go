package module

import (
	"github.com/onflow/flow-go/model/flow"
)

// SpockVerifier verifies the sporks in the execution receipts and result approvals.
// It stores the execution receipts and verifies the approval with the stored result approvals.
// Note: it can not verify approvals for results that it hasn't seen.
type SpockVerifier interface {

	// AddReceipt adds a receipt into map if the SPoCK proofs do not match any other receipt
	AddReceipt(receipt *flow.ExecutionReceipt) error

	/// VerifyApproval verifies an approval with all the distinct receipts for the approvals
	// result id and returns true if spocks match else false
	VerifyApproval(*flow.ResultApproval) (bool, error)

	// ClearReceipts clears all receipts for a specific resultID
	ClearReceipts(resultID flow.Identifier) bool
}
