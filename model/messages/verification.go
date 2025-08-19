package messages

import "github.com/onflow/flow-go/model/flow"

// ApprovalRequest represents a request for a ResultApproval corresponding to
// a specific chunk.
type ApprovalRequest struct {
	Nonce      uint64
	ResultID   flow.Identifier
	ChunkIndex uint64
}

func (a *ApprovalRequest) ToInternal() (any, error) {
	// TODO(malleability, #7717) implement with validation checks
	return a, nil
}

// ApprovalResponse contains a response to an approval request.
type ApprovalResponse struct {
	Nonce    uint64
	Approval flow.ResultApproval
}

func (a *ApprovalResponse) ToInternal() (any, error) {
	// TODO(malleability, #7718) implement with validation checks
	return a, nil
}
