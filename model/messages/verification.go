package messages

import "github.com/onflow/flow-go/model/flow"

// ApprovalRequest represents a request for a ResultApproval corresponding to
// a specific chunk.
//
//structwrite:immutable
type ApprovalRequest struct {
	Nonce      uint64
	ResultID   flow.Identifier
	ChunkIndex uint64
}

// UntrustedApprovalRequest is an untrusted input-only representation of an ApprovalRequest,
// used for construction.
//
// An instance of UntrustedApprovalRequest should be validated and converted into
// a trusted ApprovalRequest using NewApprovalRequest constructor.
type UntrustedApprovalRequest ApprovalRequest

// NewApprovalRequest creates a new instance of ApprovalRequest.
//
// Parameters:
//   - untrusted: untrusted ApprovalRequest to be validated
//
// Returns:
//   - *ApprovalRequest: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewApprovalRequest(untrusted UntrustedApprovalRequest) (*ApprovalRequest, error) {
	// TODO: add validation logic
	return &ApprovalRequest{
		Nonce:      untrusted.Nonce,
		ResultID:   untrusted.ResultID,
		ChunkIndex: untrusted.ChunkIndex,
	}, nil
}

// ApprovalResponse contains a response to an approval request.
//
//structwrite:immutable
type ApprovalResponse struct {
	Nonce    uint64
	Approval flow.ResultApproval
}

// UntrustedApprovalResponse is an untrusted input-only representation of an ApprovalResponse,
// used for construction.
//
// An instance of UntrustedApprovalResponse should be validated and converted into
// a trusted ApprovalResponse using NewApprovalResponse constructor.
type UntrustedApprovalResponse ApprovalResponse

// NewApprovalResponse creates a new instance of ApprovalResponse.
//
// Parameters:
//   - untrusted: untrusted ApprovalResponse to be validated
//
// Returns:
//   - *ApprovalResponse: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewApprovalResponse(untrusted UntrustedApprovalResponse) (*ApprovalResponse, error) {
	// TODO: add validation logic
	return &ApprovalResponse{Nonce: untrusted.Nonce, Approval: untrusted.Approval}, nil
}
