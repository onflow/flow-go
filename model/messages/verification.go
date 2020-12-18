package messages

import "github.com/onflow/flow-go/model/flow"

// ApprovalRequest represents a request for a ResultApproval corresponding to
// the a specific chunk, identified by the ID of the ExecutionResult and the
// chunk index within the result.
type ApprovalRequest struct {
	Nonce      uint64
	ResultID   flow.Identifier
	ChunkIndex uint64
}

// ApprovalResponse contains a response to an approval request.
type ApprovalResponse struct {
	Nonce    uint64
	Approval flow.ResultApproval
}
