package messages

import (
	"github.com/onflow/flow-go/model/flow"
)

// UntrustedBlock represents untrusted block model received over the network.
// This type exists only to explicitly differentiate between trusted and untrusted instances of a block.
// This differentiation is currently largely unused, but eventually untrusted models should use
// a different type (like this one), until such time as they are fully validated.
type UntrustedBlock flow.Block

// ToHeader converts the untrusted block into a compact [flow.Header] representation,
// where the payload is compressed to a hash reference.
// TODO(malleability immutable): This conversion should eventually be accompanied by a full validation of the untrusted input.
func (ub *UntrustedBlock) ToHeader() *flow.Header {
	internal := flow.NewBlock(ub.Header, ub.Payload)
	return internal.ToHeader()
}

// UntrustedProposal is part of the consensus protocol and represents the leader
// of a consensus round pushing a new proposal to the network.
// This differentiation is currently largely unused, but eventually untrusted models should use
// a different type (like this one), until such time as they are fully validated
type UntrustedProposal flow.BlockProposal

func NewUntrustedProposal(internal *flow.BlockProposal) *UntrustedProposal {
	p := UntrustedProposal(*internal)
	return &p
}

// ToInternal converts the UntrustedProposal to a trusted internal flow.BlockProposal.
// TODO(malleability immutable): This conversion should eventually be accompanied by a full validation of the untrusted input.
func (msg *UntrustedProposal) ToInternal() *flow.BlockProposal {
	return &flow.BlockProposal{
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
