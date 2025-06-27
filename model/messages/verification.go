package messages

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ApprovalRequest represents a request for a ResultApproval corresponding to
// a specific chunk.
//
type ApprovalRequest struct {
	Nonce      uint64
	ResultID   flow.Identifier
	ChunkIndex uint64
}

// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
type UntrustedApprovalRequest ApprovalRequest

//
func NewApprovalRequest(untrusted UntrustedApprovalRequest) (*ApprovalRequest, error) {
	if untrusted.ResultID == flow.ZeroID {
		return nil, fmt.Errorf("result ID must not be zero")
	}
	return &ApprovalRequest{
		Nonce:      untrusted.Nonce,
		ResultID:   untrusted.ResultID,
		ChunkIndex: untrusted.ChunkIndex,
	}, nil
}

// ApprovalResponse contains a response to an approval request.
//
type ApprovalResponse struct {
	Nonce    uint64
	Approval flow.ResultApproval
}

// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
type UntrustedApprovalResponse ApprovalResponse

//
func NewApprovalResponse(untrusted UntrustedApprovalResponse) (*ApprovalResponse, error) {
	return &ApprovalResponse{
		Nonce:    untrusted.Nonce,
		Approval: untrusted.Approval,
	}, nil
}
