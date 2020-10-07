package module

import (
	"github.com/onflow/flow-go/model/flow"
)

// SpockVerifier provides functionality to verify spocks
type SpockVerifier interface {

	// AddReceipt adds a receipt into map if the spocks do not match any other receipts
	// with the same result id
	AddReceipt(receipt *flow.ExecutionReceipt) error

	// VerifyApproval verifies an approval with all the distict receipts for the approvals
	// result id and returns true if spocks match else false
	VerifyApproval(*flow.ResultApproval) (bool, error)

	// ClearReceipts clears all receipts for a specific resultID
	ClearReceipts(resultID flow.Identifier) bool
}
