package messages

import (
	"fmt"

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

// DeclareStructurallyValid converts the UntrustedProposal to a trusted internal flow.Proposal.
// CAUTION: Prior to using this function, ensure that the untrusted proposal has been fully validated.
//
// All errors indicate that the input message could not be converted to a valid proposal.
func (msg *UntrustedProposal) DeclareStructurallyValid() (*flow.Proposal, error) {
	block, err := flow.NewBlock(
		flow.UntrustedBlock{
			Header:  msg.Block.Header,
			Payload: msg.Block.Payload,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not build block: %w", err)
	}

	// validate ProposerSigData
	if len(msg.ProposerSigData) == 0 {
		return nil, fmt.Errorf("proposer signature must not be empty")
	}
	return &flow.Proposal{
		Block:           *block,
		ProposerSigData: msg.ProposerSigData,
	}, nil
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
