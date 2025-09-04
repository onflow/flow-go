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

// ResultApproval is a message representation of a ResultApproval, which includes an approval for a chunk, verified by a verification n
type ResultApproval flow.UntrustedResultApproval

// ToInternal converts the untrusted ResultApproval into its trusted internal
// representation.
func (a *ResultApproval) ToInternal() (any, error) {
	internal, err := flow.NewResultApproval(flow.UntrustedResultApproval(*a))
	if err != nil {
		return nil, fmt.Errorf("could not construct result approval: %w", err)
	}
	return internal, nil
}
