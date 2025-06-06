package messages

import (
	"github.com/onflow/flow-go/model/flow"
)

// UntrustedBlock represents untrusted block received over the network (from a potentially byzantine peer).
// This type exists only to explicitly differentiate between trusted and untrusted instances of a block.
// This differentiation is currently largely unused. But eventually, untrusted data should be
// represented by different types (like this one), until it is fully validated.
type UntrustedBlock flow.Block

// UntrustedProposal is part of the consensus protocol and represents the leader
// of a consensus round pushing a new proposal to the network.
// This differentiation is currently largely unused, but eventually untrusted models should use
// a different type (like this one), until such time as they are fully validated
type UntrustedProposal struct {
	Block flow.OldBlock
}

func NewUntrustedProposal(internal *flow.BlockProposal) *UntrustedProposal {
	return &UntrustedProposal{
		Block: *flow.OldBlockFromProposal(internal),
	}
}

// DeclareTrusted converts the UntrustedProposal to a trusted internal flow.BlockProposal.
// CAUTION: Prior to using this function, ensure that the untrusted proposal has been fully validated.
// TODO(malleability immutable): This conversion should eventually be accompanied by a full validation of the untrusted input.
func (msg *UntrustedProposal) DeclareTrusted() *flow.BlockProposal {
	return msg.Block.ConvertToProposal()
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
