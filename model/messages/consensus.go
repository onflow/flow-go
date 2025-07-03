package messages

import (
	"fmt"

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
type UntrustedProposal flow.BlockProposal

func NewUntrustedProposal(internal *flow.BlockProposal) *UntrustedProposal {
	p := UntrustedProposal(*internal)
	return &p
}

// DeclareTrusted converts the UntrustedProposal to a trusted internal flow.BlockProposal.
// CAUTION: Prior to using this function, ensure that the untrusted proposal has been fully validated.
//
// All errors indicate that the input message could not be converted to a valid proposal.
func (msg *UntrustedProposal) DeclareTrusted() (*flow.BlockProposal, error) {
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
	return &flow.BlockProposal{
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
