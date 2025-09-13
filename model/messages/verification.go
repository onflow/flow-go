package messages

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ApprovalRequest represents a request for a ResultApproval corresponding to
// a specific chunk.
type ApprovalRequest struct {
	Nonce      uint64
	ResultID   flow.Identifier
	ChunkIndex uint64
}

// ToInternal converts the untrusted ApprovalRequest into its trusted internal
// representation.
func (a *ApprovalRequest) ToInternal() (any, error) {
	if a.ResultID == flow.ZeroID {
		return nil, fmt.Errorf("ResultID of approval request must not be zero")
	}

	return &flow.ApprovalRequest{
		Nonce:      a.Nonce,
		ResultID:   a.ResultID,
		ChunkIndex: a.ChunkIndex,
	}, nil
}

// ApprovalResponse contains a response to an approval request.
type ApprovalResponse struct {
	Nonce    uint64
	Approval flow.UntrustedResultApproval
}

// ToInternal converts the untrusted ApprovalResponse into its trusted internal
// representation.
func (a *ApprovalResponse) ToInternal() (any, error) {
	approval, err := flow.NewResultApproval(a.Approval)
	if err != nil {
		return nil, fmt.Errorf("invalid result approval: %w", err)
	}

	return &flow.ApprovalResponse{
		Nonce:    a.Nonce,
		Approval: *approval,
	}, nil
}

// ResultApproval is a message representation of a ResultApproval, which includes an approval for a chunk, verified by a verification n
type ResultApproval flow.UntrustedResultApproval

// ToInternal converts the untrusted ResultApproval into its trusted internal
// representation.
func (a *ResultApproval) ToInternal() (any, error) {
	internal, err := flow.NewResultApproval(flow.UntrustedResultApproval(*a))
	if err != nil {
		return nil, fmt.Errorf("could not convert %T to internal type: %w", a, err)
	}
	return internal, nil
}
