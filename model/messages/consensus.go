package messages

import (
	"github.com/onflow/flow-go/model/flow"
)

// UntrustedProposal is part of the consensus protocol and represents the leader
// of a consensus round pushing a new proposal to the network.
// This differentiation is currently largely unused, but eventually untrusted models should use
// a different type (like this one), until such time as they are fully validated
type UntrustedProposal flow.Proposal

func NewUntrustedProposal(internal *flow.Proposal) *UntrustedProposal {
	p := UntrustedProposal(*internal)
	return &p
}

// DeclareTrusted converts the UntrustedProposal to a trusted internal flow.Proposal.
// CAUTION: Prior to using this function, ensure that the untrusted proposal has been fully validated.
// TODO(malleability immutable): This conversion should eventually be accompanied by a full validation of the untrusted input.
func (msg *UntrustedProposal) DeclareTrusted() *flow.Proposal {
	return &flow.Proposal{
		Block:           *flow.NewBlock(msg.Block.Header, msg.Block.Payload),
		ProposerSigData: msg.ProposerSigData,
	}
}

// BlockVote is part of the consensus protocol and represents a consensus node
// voting on the proposal of the leader of a given round.
type BlockVote struct {
	BlockID flow.Identifier
	View    uint64
	SigData []byte
}

// TimeoutObject is part of the consensus protocol and represents a consensus node
// timing out in given round. Contains a sequential number for deduplication purposes.
type TimeoutObject struct {
	TimeoutTick uint64
	View        uint64
	NewestQC    *flow.QuorumCertificate
	LastViewTC  *flow.TimeoutCertificate
	SigData     []byte
}
