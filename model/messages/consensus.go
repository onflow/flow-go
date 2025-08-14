package messages

import (
	"github.com/onflow/flow-go/model/flow"
)

// Proposal is part of the consensus protocol and represents a consensus node
// untrusted signed block proposal.
type Proposal flow.UntrustedProposal

// TODO: Proposal should implement UntrustedMessage interface
func (p *Proposal) ToInternal() (*flow.Proposal, error) {
	return flow.NewProposal(flow.UntrustedProposal(*p))
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
