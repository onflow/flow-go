package messages

import "github.com/onflow/flow-go/model/flow"

// ApprovalRequest represents a request for a ResultApproval corresponding to
// a specific chunk.
type ApprovalRequest struct {
	Nonce      uint64
	ResultID   flow.Identifier
	ChunkIndex uint64
}

var _ UntrustedMessage = (*ApprovalRequest)(nil)

func (ap *ApprovalRequest) ToInternal() (any, error) {
	// Temporary: just return the unvalidated wire struct
	return ap, nil
}

// ApprovalResponse contains a response to an approval request.
type ApprovalResponse struct {
	Nonce    uint64
	Approval flow.ResultApproval
}

var _ UntrustedMessage = (*ApprovalResponse)(nil)

func (ap *ApprovalResponse) ToInternal() (any, error) {
	// Temporary: just return the unvalidated wire struct
	return ap, nil
}
