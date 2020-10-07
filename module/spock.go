package module

import (
	"github.com/onflow/flow-go/model/flow"
)

// SpockVerifier provides functionality to verify spocks
type SpockVerifier interface {

	// AddReceipt ...
	AddReceipt(receipt *flow.ExecutionReceipt) error

	// VerifyApproval
	VerifyApproval(*flow.ResultApproval) (bool, error)
}
