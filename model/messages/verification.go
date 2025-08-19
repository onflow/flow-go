package messages

import "github.com/onflow/flow-go/model/flow"

// ApprovalRequest represents a request for a ResultApproval corresponding to
// a specific chunk.
type ApprovalRequest struct {
	Nonce      uint64
	ResultID   flow.Identifier
	ChunkIndex uint64
}

// ToInternal converts the untrusted ApprovalRequest into its trusted internal
// representation.
//
// This stub returns the receiver unchanged. A proper implementation
// must perform validation checks and return a constructed internal
// object.
func (a *ApprovalRequest) ToInternal() (any, error) {
	// TODO(malleability, #7717) implement with validation checks
	return a, nil
}

// ApprovalResponse contains a response to an approval request.
type ApprovalResponse struct {
	Nonce    uint64
	Approval flow.ResultApproval
}

// ToInternal converts the untrusted ApprovalResponse into its trusted internal
// representation.
//
// This stub returns the receiver unchanged. A proper implementation
// must perform validation checks and return a constructed internal
// object.
func (a *ApprovalResponse) ToInternal() (any, error) {
	// TODO(malleability, #7718) implement with validation checks
	return a, nil
}
