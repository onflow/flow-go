package messages

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Proposal is part of the consensus protocol and represents a signed proposal from a consensus node.
type Proposal flow.UntrustedProposal

// ToInternal returns the internal type representation for Proposal.
//
// All errors indicate that the decode target contains a structurally invalid representation of the internal flow.Proposal.
func (p *Proposal) ToInternal() (any, error) {
	internal, err := flow.NewProposal(flow.UntrustedProposal(*p))
	if err != nil {
		return nil, fmt.Errorf("could not convert %T to internal type: %w", p, err)
	}
	return internal, nil
}

// BlockVote is part of the consensus protocol and represents a consensus node
// voting on the proposal of the leader of a given round.
type BlockVote struct {
	BlockID flow.Identifier
	View    uint64
	SigData []byte
}

// ToInternal converts the untrusted BlockVote into its trusted internal
// representation.
//
// This stub returns the receiver unchanged. A proper implementation
// must perform validation checks and return a constructed internal
// object.
func (b *BlockVote) ToInternal() (any, error) {
	// TODO(malleability, #7702) implement with validation checks
	return b, nil
}

// TimeoutObject is part of the consensus protocol and represents a consensus node
// timing out in given round. Contains a sequential number for deduplication purposes.
type TimeoutObject model.UntrustedTimeoutObject

// ToInternal returns the internal type representation for TimeoutObject.
//
// All errors indicate that the decode target contains a structurally invalid representation of the internal model.TimeoutObject.
func (t *TimeoutObject) ToInternal() (any, error) {
	internal, err := model.NewTimeoutObject(model.UntrustedTimeoutObject(*t))
	if err != nil {
		return nil, fmt.Errorf("could not convert %T to internal type: %w", t, err)
	}
	return internal, nil
}
